# DBResolver - Read/Replica Routing Analysis

## Overview

The DBResolver in GoFr implements read/write splitting for database operations. It automatically routes read queries to replica databases and write operations to the primary database.

## Read/Replica Choice Flow

### 1. HTTP Middleware Setup (Entry Point)

**Location:** `gofr/pkg/gofr/datasource/dbresolver/factory.go:179-191`

```go
func createHTTPMiddleware() gofrHTTP.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := r.Context()
            ctx = WithHTTPMethod(ctx, r.Method)      // Sets HTTP method in context
            ctx = WithRequestPath(ctx, r.URL.Path)   // Sets request path in context
            
            r = r.WithContext(ctx)
            next.ServeHTTP(w, r)
        })
    }
}
```

**Purpose:** Injects HTTP method and request path into the request context so the resolver can make routing decisions.

**Failure Points:**
- ❌ **Middleware not registered**: If `InitDBResolver()` is not called or middleware is not added, context values won't be set
- ❌ **Context propagation lost**: If context is replaced somewhere in the middleware chain, routing information is lost

---

### 2. Routing Decision Logic

**Location:** `gofr/pkg/gofr/datasource/dbresolver/resolver.go:177-197`

```go
func (r *Resolver) shouldUseReplica(ctx context.Context) bool {
    // 1. Check if replicas exist
    if len(r.replicas) == 0 {
        return false
    }

    // 2. Check if path requires primary (explicit primary routes)
    if path, ok := ctx.Value(contextKeyRequestPath).(string); ok {
        if r.isPrimaryRoute(path) {
            return false
        }
    }

    // 3. Check HTTP method
    method, ok := ctx.Value(contextKeyHTTPMethod).(string)
    if !ok {
        return false // Default to primary for safety
    }

    // 4. Only GET, HEAD, OPTIONS use replicas
    return method == "GET" || method == "HEAD" || method == "OPTIONS"
}
```

**Decision Flow:**
1. **No replicas configured** → Always use primary
2. **Path in primary routes** → Use primary (even for GET)
3. **HTTP method not in context** → Default to primary (safety)
4. **GET/HEAD/OPTIONS** → Use replica
5. **POST/PUT/DELETE/PATCH** → Use primary

**Failure Points:**
- ❌ **Context value missing**: If `contextKeyHTTPMethod` is not set, defaults to primary (safe but may not route optimally)
- ❌ **Type assertion fails**: If context value is wrong type, `ok` is false → defaults to primary
- ❌ **No replicas configured**: All queries go to primary (not a failure, but suboptimal)

---

### 3. Replica Selection (Strategy)

**Location:** `gofr/pkg/gofr/datasource/dbresolver/resolver.go:215-255`

```go
func (r *Resolver) selectHealthyReplica() *replicaWrapper {
    if len(r.replicas) == 0 {
        return nil
    }

    // Filter healthy replicas (circuit breaker check)
    var availableDbs []container.DB
    var availableWrappers []*replicaWrapper

    for _, wrapper := range r.replicas {
        if wrapper.breaker.allowRequest() {  // Circuit breaker check
            availableDbs = append(availableDbs, wrapper.db)
            availableWrappers = append(availableWrappers, wrapper)
        }
    }

    if len(availableDbs) == 0 {
        // All circuit breakers are open
        return nil
    }

    // Use strategy to choose from available replicas
    chosenDB, err := r.strategy.Choose(availableDbs)
    if err != nil {
        return nil
    }

    // Find wrapper for chosen DB
    for _, wrapper := range availableWrappers {
        if wrapper.db == chosenDB {
            return wrapper
        }
    }

    return availableWrappers[0]  // Fallback
}
```

**Selection Process:**
1. **Filter by circuit breaker**: Only consider replicas with closed/half-open circuit breakers
2. **Apply strategy**: Use RoundRobin or Random strategy to select one
3. **Return wrapper**: Returns the replica wrapper with circuit breaker

**Failure Points:**
- ❌ **All circuit breakers open**: All replicas marked as failed → returns `nil` → falls back to primary
- ❌ **Strategy error**: If strategy.Choose() fails → returns `nil` → falls back to primary
- ❌ **No available replicas**: Empty list after filtering → returns `nil` → falls back to primary

---

### 4. Circuit Breaker Health Check

**Location:** `gofr/pkg/gofr/datasource/dbresolver/circuit_breaker.go:38-58`

```go
func (cb *circuitBreaker) allowRequest() bool {
    state := cb.state.Load()
    
    // CLOSED or HALF_OPEN: allow requests
    if *state != circuitStateOpen {
        return true
    }

    // OPEN state: check if timeout has passed
    lastFailurePtr := cb.lastFailure.Load()
    if lastFailurePtr == nil {
        return true
    }

    // Still within timeout period
    if time.Since(*lastFailurePtr) <= cb.timeout {
        return false  // Reject request
    }

    // Timeout passed: try to transition to HALF_OPEN
    openState := circuitStateOpen
    halfOpenState := circuitStateHalfOpen
    return cb.state.CompareAndSwap(&openState, &halfOpenState)
}
```

**Circuit Breaker States:**
- **CLOSED**: Healthy, allows requests
- **OPEN**: Failed too many times, rejects requests
- **HALF_OPEN**: Testing if replica recovered, allows limited requests

**Failure Points:**
- ❌ **Circuit breaker stuck OPEN**: If replica is down, circuit breaker stays open → no requests allowed
- ❌ **Race condition**: State transition might fail in concurrent scenarios
- ❌ **Timeout too short**: Circuit breaker might not wait long enough before retry

---

### 5. Query Execution with Fallback

**Location:** `gofr/pkg/gofr/datasource/dbresolver/resolver.go:323-373`

```go
func (r *Resolver) executeReplicaQuery(ctx context.Context, span trace.Span, start time.Time,
    query string, args ...any) (*sql.Rows, error) {
    
    // 1. Select healthy replica
    wrapper := r.selectHealthyReplica()
    if wrapper == nil {
        return r.fallbackToPrimary(ctx, span, start, query, 
            "No healthy replica available, falling back to primary", args...)
    }

    // 2. Execute query on replica
    rows, err := wrapper.db.QueryContext(ctx, query, args...)
    if err == nil {
        // Success: record success, update stats
        r.stats.replicaReads.Add(1)
        wrapper.breaker.recordSuccess()
        r.recordStats(start, "query", "replica", span, true)
        return rows, nil
    }

    // 3. Failure: record failure, trigger circuit breaker
    wrapper.breaker.recordFailure()
    r.stats.replicaFailures.Add(1)
    
    if r.logger != nil {
        r.logger.Errorf("Replica #%d failed, circuit breaker triggered: %v", 
            wrapper.index+1, err)
    }

    // 4. Fallback to primary
    return r.fallbackToPrimary(ctx, span, start, query, 
        "Falling back to primary for read operation", args...)
}
```

**Execution Flow:**
1. **Select replica** → If none available, fallback immediately
2. **Execute query** → On selected replica
3. **Success** → Record success, return results
4. **Failure** → Record failure, trigger circuit breaker, fallback to primary

**Failure Points:**
- ❌ **Replica query fails**: Network error, timeout, or database error → falls back to primary
- ❌ **Fallback disabled**: If `readFallback = false`, returns error instead of falling back
- ❌ **Primary also fails**: If primary is down, the fallback will also fail
- ❌ **Context timeout**: If context is cancelled/timed out, query fails

---

### 6. Fallback to Primary

**Location:** `gofr/pkg/gofr/datasource/dbresolver/resolver.go:353-373`

```go
func (r *Resolver) fallbackToPrimary(ctx context.Context, span trace.Span, start time.Time,
    query, warningMsg string, args ...any) (*sql.Rows, error) {
    
    // Check if fallback is enabled
    if !r.readFallback {
        r.recordStats(start, "query", "replica-failed", span, true)
        return nil, errReplicaFailedNoFallback
    }

    // Update stats
    r.stats.primaryFallbacks.Add(1)
    r.stats.primaryReads.Add(1)

    // Log warning
    if r.logger != nil && warningMsg != "" {
        r.logger.Warn(warningMsg)
    }

    // Execute on primary
    rows, err := r.primary.QueryContext(ctx, query, args...)
    r.recordStats(start, "query", "primary-fallback", span, true)

    return rows, err
}
```

**Failure Points:**
- ❌ **Fallback disabled**: If `readFallback = false`, returns `errReplicaFailedNoFallback` error
- ❌ **Primary connection down**: Primary query will fail with connection error
- ❌ **Primary overloaded**: Too many fallback requests can overload primary

---

## Complete Failure Scenarios

### Scenario 1: Middleware Not Registered
**Symptom:** All queries go to primary, even GET requests
**Root Cause:** `InitDBResolver()` not called or middleware not added
**Location:** `factory.go:179-191`
**Fix:** Ensure `InitDBResolver()` is called before routes are registered

### Scenario 2: Context Value Missing
**Symptom:** GET requests go to primary instead of replica
**Root Cause:** HTTP method not in context (middleware not executed or context replaced)
**Location:** `resolver.go:191-194`
**Fix:** Ensure middleware runs before handler execution

### Scenario 3: All Circuit Breakers Open
**Symptom:** All read queries fallback to primary
**Root Cause:** All replicas have failed multiple times (default: 5 failures)
**Location:** `resolver.go:234-239`
**Fix:** Check replica health, wait for circuit breaker timeout (default: 30s)

### Scenario 4: Replica Query Fails
**Symptom:** Query fails or falls back to primary
**Root Cause:** Network error, timeout, database error, or replica down
**Location:** `resolver.go:332-350`
**Fix:** Check replica connectivity, network, and database status

### Scenario 5: Fallback Disabled
**Symptom:** Query fails with `errReplicaFailedNoFallback`
**Root Cause:** `readFallback = false` and replica failed
**Location:** `resolver.go:356-360`
**Fix:** Enable fallback or ensure replicas are healthy

### Scenario 6: Primary Also Down
**Symptom:** Fallback query also fails
**Root Cause:** Primary database is down or unreachable
**Location:** `resolver.go:369`
**Fix:** Check primary database health and connectivity

### Scenario 7: Strategy Selection Fails
**Symptom:** Falls back to primary even with healthy replicas
**Root Cause:** Strategy.Choose() returns error (e.g., empty replica list)
**Location:** `resolver.go:243-246`
**Fix:** Check strategy implementation and replica list

### Scenario 8: Race Condition in Circuit Breaker
**Symptom:** Inconsistent routing decisions
**Root Cause:** Concurrent access to circuit breaker state
**Location:** `circuit_breaker.go:38-58`
**Fix:** Already uses atomic operations, but high concurrency might cause issues

---

## Key Code Locations Summary

| Component | File | Lines | Purpose |
|-----------|------|-------|---------|
| **Middleware Setup** | `factory.go` | 179-191 | Injects HTTP method/path into context |
| **Routing Decision** | `resolver.go` | 177-197 | Decides replica vs primary |
| **Replica Selection** | `resolver.go` | 215-255 | Chooses which replica to use |
| **Circuit Breaker** | `circuit_breaker.go` | 38-58 | Health check for replicas |
| **Query Execution** | `resolver.go` | 323-373 | Executes query with fallback |
| **Fallback Logic** | `resolver.go` | 353-373 | Falls back to primary on failure |
| **Strategy** | `strategy.go` | 39-54 | RoundRobin/Random selection |

---

## Best Practices

1. **Always use `InitDBResolver()`**: Ensures middleware is registered
2. **Monitor circuit breaker state**: Check health endpoints regularly
3. **Enable fallback**: Set `readFallback = true` for production
4. **Configure primary routes**: Explicitly mark write endpoints
5. **Monitor metrics**: Track `dbresolver_fallbacks` and `dbresolver_failures`
6. **Test failure scenarios**: Verify fallback behavior in staging

---

## Debugging Tips

1. **Check context values**: Log `ctx.Value(contextKeyHTTPMethod)` in handler
2. **Monitor circuit breaker state**: Use `HealthCheck()` endpoint
3. **Check logs**: Look for "falling back to primary" warnings
4. **Verify middleware**: Ensure middleware runs before handler
5. **Check metrics**: Monitor `dbresolver_replica_reads` vs `dbresolver_primary_reads`


