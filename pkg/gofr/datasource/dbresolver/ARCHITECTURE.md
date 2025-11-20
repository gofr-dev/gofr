# DBResolver Architecture & Flow Documentation

## Table of Contents
1. [Architecture Overview](#architecture-overview)
2. [Component Diagram](#component-diagram)
3. [Complete Request Flow](#complete-request-flow)
4. [Data Structures](#data-structures)
5. [Code Reference Map](#code-reference-map)
6. [Integration Points](#integration-points)
7. [Sequence Diagrams](#sequence-diagrams)

---

## Architecture Overview

The DBResolver implements a **read/write splitting pattern** for database operations in GoFr. It automatically routes:
- **Read operations** (GET, HEAD, OPTIONS) → Replica databases
- **Write operations** (POST, PUT, DELETE, PATCH) → Primary database

### Key Components

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Request Flow                         │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  HTTP Middleware (factory.go)                                │
│  - Injects HTTP method & path into context                  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  GoFr Handler (handler.go)                                   │
│  - Creates gofr.Context from http.Request                    │
│  - Executes user handler function                            │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  User Handler Code                                           │
│  ctx.SQL.QueryContext(...) or ctx.SQL.ExecContext(...)      │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Resolver (resolver.go)                                      │
│  - Implements container.DB interface                        │
│  - Routes based on HTTP method                              │
│  - Selects replica using strategy                           │
│  - Handles circuit breakers                                 │
│  - Falls back to primary on failure                         │
└─────────────────────────────────────────────────────────────┘
                            │
                ┌───────────┴───────────┐
                ▼                         ▼
    ┌───────────────────┐      ┌───────────────────┐
    │  Primary DB       │      │  Replica DB(s)    │
    │  (Write/Read)     │      │  (Read Only)      │
    └───────────────────┘      └───────────────────┘
```

---

## Component Diagram

### 1. Application Layer
```
gofr.App
├── AddDBResolver()          // external_db.go:225
│   ├── Validates primary SQL exists
│   ├── Sets logger, metrics, tracer
│   ├── Calls resolver.Connect()
│   └── Replaces container.SQL with resolver
│
└── InitDBResolver()         // factory.go:167
    ├── Creates ResolverProvider
    ├── Registers HTTP middleware
    └── Calls AddDBResolver()
```

### 2. Middleware Layer
```
HTTP Middleware (factory.go:179-191)
├── Intercepts all HTTP requests
├── Extracts r.Method → context
├── Extracts r.URL.Path → context
└── Passes context to next handler
```

### 3. Resolver Layer
```
Resolver (resolver.go:50-70)
├── Primary DB connection
├── Replica wrappers (with circuit breakers)
├── Strategy (RoundRobin/Random)
├── Statistics tracking
└── Options (fallback, primary routes)
```

### 4. Circuit Breaker Layer
```
Circuit Breaker (circuit_breaker.go:17-23)
├── State: CLOSED/OPEN/HALF_OPEN
├── Failure counter
├── Last failure timestamp
└── Timeout configuration
```

### 5. Strategy Layer
```
Strategy (strategy.go:24-27)
├── RoundRobinStrategy
│   └── Sequential selection with atomic counter
└── RandomStrategy
    └── Random selection
```

---

## Complete Request Flow

### Phase 1: Application Initialization

**Step 1.1: Create GoFr App**
```go
// User code
app := gofr.New()
```
- **Location**: `gofr/pkg/gofr/gofr.go`
- **Result**: App instance with container initialized

**Step 1.2: Initialize Primary SQL Connection**
```go
// Internal (automatic from config)
container.SQL = sql.NewSQL(configs, logger, metrics)
```
- **Location**: `gofr/pkg/gofr/container/container.go`
- **Result**: Primary database connection stored in container

**Step 1.3: Initialize DBResolver**
```go
// User code
rb := dbresolver.NewRoundRobinStrategy(3)
resolver := dbresolver.NewProvider(rb, true)
app.AddDBResolver(resolver)
```

**Flow:**
```
app.AddDBResolver(resolver)
  │
  ├─> external_db.go:225
  │   ├─> Validate primary SQL exists (line 227-230)
  │   ├─> Set logger (line 232)
  │   ├─> Set metrics (line 233)
  │   ├─> Set tracer (line 235-236)
  │   ├─> resolver.Connect() (line 238)
  │   │   │
  │   │   └─> factory.go:77
  │   │       ├─> Get primary from app (line 79)
  │   │       ├─> Create replicas from config (line 100)
  │   │       │   │
  │   │       │   └─> factory.go:194
  │   │       │       ├─> Parse DB_REPLICA_HOSTS (line 195)
  │   │       │       ├─> For each replica:
  │   │       │       │   ├─> Parse host:port (line 212-214)
  │   │       │       │   ├─> Create replica connection (line 219)
  │   │       │       │   └─> Wrap with circuit breaker (resolver.go:75-82)
  │   │       │       └─> Return replica list
  │   │       │
  │   │       ├─> Build primary routes map (line 106-110)
  │   │       ├─> Create strategy (line 112)
  │   │       └─> NewResolver() (line 115-123)
  │   │           │
  │   │           └─> resolver.go:73
  │   │               ├─> Wrap replicas with circuit breakers (line 75-82)
  │   │               ├─> Initialize statistics (line 90)
  │   │               ├─> Set default strategy (line 96-98)
  │   │               ├─> Apply options (line 101-103)
  │   │               ├─> Initialize metrics (line 112)
  │   │               └─> Start background tasks (line 113)
  │   │
  │   └─> Replace container.SQL with resolver (line 241)
```

**Step 1.4: Register HTTP Middleware (if using InitDBResolver)**
```go
// Alternative: Using InitDBResolver
dbresolver.InitDBResolver(app, cfg)
```

**Flow:**
```
InitDBResolver(app, cfg)
  │
  ├─> factory.go:167
  │   ├─> Create ResolverProvider (line 168)
  │   ├─> Register HTTP middleware (line 171)
  │   │   │
  │   │   └─> factory.go:180
  │   │       └─> createHTTPMiddleware()
  │   │           └─> Returns middleware function (line 181-190)
  │   │               ├─> Extracts r.Method (line 184)
  │   │               ├─> Extracts r.URL.Path (line 185)
  │   │               └─> Sets in context
  │   │
  │   └─> AddDBResolver() (line 174)
```

---

### Phase 2: HTTP Request Processing

**Step 2.1: HTTP Request Arrives**
```
HTTP Request → http.Server → Router → Handler
```

**Step 2.2: Middleware Execution**
```go
// factory.go:181-190
func(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    ctx = WithHTTPMethod(ctx, r.Method)      // Sets "GET", "POST", etc.
    ctx = WithRequestPath(ctx, r.URL.Path)   // Sets "/users", etc.
    r = r.WithContext(ctx)
    next.ServeHTTP(w, r)
}
```
- **Location**: `factory.go:184-185`
- **Context Keys**: 
  - `contextKeyHTTPMethod = "dbresolver.http_method"`
  - `contextKeyRequestPath = "dbresolver.request_path"`

**Step 2.3: Handler Execution**
```go
// handler.go:55-113
func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    c := newContext(gofrHTTP.NewResponder(w, r.Method), 
                    gofrHTTP.NewRequest(r), 
                    h.container)
    // ... timeout handling ...
    result, err = h.function(c)  // User handler
}
```
- **Location**: `gofr/pkg/gofr/handler.go:55`
- **Result**: `gofr.Context` created with embedded `container.Container`

**Step 2.4: User Handler Code**
```go
// User code
func GetUsers(ctx *gofr.Context) (interface{}, error) {
    rows, err := ctx.SQL.QueryContext(ctx, "SELECT * FROM users")
    // ...
}
```
- **Location**: User application code
- **Note**: `ctx.SQL` is now the Resolver (not direct SQL connection)

---

### Phase 3: Query Routing Decision

**Step 3.1: QueryContext Called**
```go
// resolver.go:302
func (r *Resolver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
    start := time.Now()
    r.stats.totalQueries.Add(1)
    
    tracedCtx, span := r.addTrace(ctx, "query", query)
    useReplica := r.shouldUseReplica(ctx)  // ← DECISION POINT
    
    if useReplica && len(r.replicas) > 0 {
        return r.executeReplicaQuery(tracedCtx, span, start, query, args...)
    }
    
    // Use primary
    r.stats.primaryWrites.Add(1)
    rows, err := r.primary.QueryContext(tracedCtx, query, args...)
    r.recordStats(start, "query", "primary", span, false)
    return rows, err
}
```

**Step 3.2: Routing Decision Logic**
```go
// resolver.go:177-197
func (r *Resolver) shouldUseReplica(ctx context.Context) bool {
    // 1. Check if replicas exist
    if len(r.replicas) == 0 {
        return false  // No replicas → use primary
    }
    
    // 2. Check if path requires primary (explicit routes)
    if path, ok := ctx.Value(contextKeyRequestPath).(string); ok {
        if r.isPrimaryRoute(path) {
            return false  // Explicit primary route → use primary
        }
    }
    
    // 3. Check HTTP method
    method, ok := ctx.Value(contextKeyHTTPMethod).(string)
    if !ok {
        return false  // Method not in context → default to primary (safety)
    }
    
    // 4. Only GET, HEAD, OPTIONS use replicas
    return method == "GET" || method == "HEAD" || method == "OPTIONS"
}
```

**Decision Tree:**
```
shouldUseReplica()
  │
  ├─> No replicas? → false (use primary)
  │
  ├─> Path in primaryRoutes? → false (use primary)
  │
  ├─> HTTP method not in context? → false (use primary)
  │
  └─> Method is GET/HEAD/OPTIONS? → true (use replica)
      └─> Otherwise → false (use primary)
```

---

### Phase 4: Replica Selection

**Step 4.1: Select Healthy Replica**
```go
// resolver.go:215-255
func (r *Resolver) selectHealthyReplica() *replicaWrapper {
    if len(r.replicas) == 0 {
        return nil
    }
    
    // Filter by circuit breaker
    var availableDbs []container.DB
    var availableWrappers []*replicaWrapper
    
    for _, wrapper := range r.replicas {
        if wrapper.breaker.allowRequest() {  // ← Circuit breaker check
            availableDbs = append(availableDbs, wrapper.db)
            availableWrappers = append(availableWrappers, wrapper)
        }
    }
    
    if len(availableDbs) == 0 {
        return nil  // All circuit breakers open
    }
    
    // Use strategy to choose
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

**Step 4.2: Circuit Breaker Check**
```go
// circuit_breaker.go:38-58
func (cb *circuitBreaker) allowRequest() bool {
    state := cb.state.Load()
    
    // CLOSED or HALF_OPEN: allow
    if *state != circuitStateOpen {
        return true
    }
    
    // OPEN: check timeout
    lastFailurePtr := cb.lastFailure.Load()
    if lastFailurePtr == nil {
        return true
    }
    
    // Still in timeout period
    if time.Since(*lastFailurePtr) <= cb.timeout {
        return false  // Reject
    }
    
    // Timeout passed: try HALF_OPEN
    openState := circuitStateOpen
    halfOpenState := circuitStateHalfOpen
    return cb.state.CompareAndSwap(&openState, &halfOpenState)
}
```

**Step 4.3: Strategy Selection**
```go
// strategy.go:41-54 (RoundRobin)
func (s *RoundRobinStrategy) Choose(replicas []container.DB) (container.DB, error) {
    replicaCount := int64(len(replicas))
    if replicaCount == 0 {
        return nil, errNoReplicasAvailable
    }
    
    count := s.current.Add(1)  // Atomic increment
    idx64 := count % replicaCount
    idx := int(idx64)
    
    return replicas[idx], nil
}
```

---

### Phase 5: Query Execution

**Step 5.1: Execute on Replica**
```go
// resolver.go:323-351
func (r *Resolver) executeReplicaQuery(ctx context.Context, span trace.Span, 
    start time.Time, query string, args ...any) (*sql.Rows, error) {
    
    // Select replica
    wrapper := r.selectHealthyReplica()
    if wrapper == nil {
        return r.fallbackToPrimary(ctx, span, start, query, 
            "No healthy replica available, falling back to primary", args...)
    }
    
    // Execute query
    rows, err := wrapper.db.QueryContext(ctx, query, args...)
    
    if err == nil {
        // Success
        r.stats.replicaReads.Add(1)
        wrapper.breaker.recordSuccess()
        r.recordStats(start, "query", "replica", span, true)
        return rows, nil
    }
    
    // Failure
    wrapper.breaker.recordFailure()
    r.stats.replicaFailures.Add(1)
    
    if r.logger != nil {
        r.logger.Errorf("Replica #%d failed, circuit breaker triggered: %v", 
            wrapper.index+1, err)
    }
    
    // Fallback to primary
    return r.fallbackToPrimary(ctx, span, start, query, 
        "Falling back to primary for read operation", args...)
}
```

**Step 5.2: Circuit Breaker State Update**
```go
// circuit_breaker.go:60-69 (Success)
func (cb *circuitBreaker) recordSuccess() {
    cb.failures.Store(0)
    cb.lastFailure.Store(nil)
    closedState := circuitStateClosed
    cb.state.Store(&closedState)
}

// circuit_breaker.go:71-81 (Failure)
func (cb *circuitBreaker) recordFailure() {
    failures := cb.failures.Add(1)
    now := time.Now()
    cb.lastFailure.Store(&now)
    
    if failures >= cb.maxFailures {
        openState := circuitStateOpen
        cb.state.Store(&openState)
    }
}
```

**Step 5.3: Fallback to Primary**
```go
// resolver.go:353-373
func (r *Resolver) fallbackToPrimary(ctx context.Context, span trace.Span, 
    start time.Time, query, warningMsg string, args ...any) (*sql.Rows, error) {
    
    // Check if fallback enabled
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

---

## Data Structures

### 1. Resolver
```go
// resolver.go:50-70
type Resolver struct {
    primary      container.DB           // Primary database connection
    replicas     []*replicaWrapper      // Replica connections with circuit breakers
    strategy     Strategy               // Load balancing strategy
    readFallback bool                   // Enable fallback to primary
    
    logger  Logger                      // Logger interface
    metrics Metrics                     // Metrics interface
    tracer  trace.Tracer                // OpenTelemetry tracer
    
    primaryRoutes   map[string]bool     // Explicit primary routes
    primaryPrefixes []string            // Primary route prefixes
    
    stats *statistics                   // Atomic counters
    
    stopChan chan struct{}              // Background task control
    wg       sync.WaitGroup             // Wait group for background tasks
    once     sync.Once                  // Ensure cleanup runs once
}
```

### 2. Replica Wrapper
```go
// resolver.go:43-48
type replicaWrapper struct {
    db      container.DB                // Replica database connection
    breaker *circuitBreaker             // Circuit breaker for health
    index   int                         // Replica index
}
```

### 3. Circuit Breaker
```go
// circuit_breaker.go:17-23
type circuitBreaker struct {
    failures    atomic.Int32            // Failure count
    lastFailure atomic.Pointer[time.Time] // Last failure time
    state       atomic.Pointer[circuitBreakerState] // Current state
    maxFailures int32                   // Max failures before open
    timeout     time.Duration            // Timeout before retry
}
```

### 4. Statistics
```go
// resolver.go:33-41
type statistics struct {
    primaryReads     atomic.Uint64      // Reads on primary
    primaryWrites    atomic.Uint64      // Writes on primary
    replicaReads     atomic.Uint64      // Reads on replicas
    primaryFallbacks atomic.Uint64      // Fallbacks to primary
    replicaFailures  atomic.Uint64      // Replica failures
    totalQueries     atomic.Uint64      // Total queries
}
```

### 5. Context Keys
```go
// resolver.go:20-23
const (
    contextKeyHTTPMethod  contextKey = "dbresolver.http_method"
    contextKeyRequestPath contextKey = "dbresolver.request_path"
)
```

---

## Code Reference Map

### Initialization Flow
| Step | File | Lines | Description |
|------|------|-------|-------------|
| App creation | `gofr/pkg/gofr/gofr.go` | - | Create GoFr app |
| Primary SQL | `gofr/pkg/gofr/datasource/sql/sql.go` | 66-124 | Initialize primary connection |
| Add resolver | `gofr/pkg/gofr/external_db.go` | 225-244 | Register resolver with app |
| Connect | `gofr/pkg/gofr/datasource/dbresolver/factory.go` | 77-126 | Initialize resolver |
| Create replicas | `gofr/pkg/gofr/datasource/dbresolver/factory.go` | 194-230 | Create replica connections |
| New resolver | `gofr/pkg/gofr/datasource/dbresolver/resolver.go` | 73-120 | Create resolver instance |

### Request Flow
| Step | File | Lines | Description |
|------|------|-------|-------------|
| HTTP middleware | `factory.go` | 179-191 | Inject method/path into context |
| Handler | `gofr/pkg/gofr/handler.go` | 55-113 | Create gofr.Context |
| Query call | `resolver.go` | 302-321 | QueryContext entry point |
| Routing decision | `resolver.go` | 177-197 | shouldUseReplica() |
| Replica selection | `resolver.go` | 215-255 | selectHealthyReplica() |
| Circuit breaker | `circuit_breaker.go` | 38-58 | allowRequest() |
| Strategy | `strategy.go` | 41-54 | Choose() |
| Execute query | `resolver.go` | 323-351 | executeReplicaQuery() |
| Fallback | `resolver.go` | 353-373 | fallbackToPrimary() |

### Write Operations
| Operation | File | Lines | Description |
|-----------|------|-------|-------------|
| Exec | `resolver.go` | 409-424 | Always routes to primary |
| ExecContext | `resolver.go` | 414-424 | Always routes to primary |
| Begin | `resolver.go` | 467-471 | Transactions always on primary |
| Prepare | `resolver.go` | 460-464 | Prepared statements on primary |

---

## Integration Points

### 1. Container Integration
```go
// gofr/pkg/gofr/container/container.go
type Container struct {
    SQL container.DB  // ← Replaced with Resolver
    // ... other fields
}
```

### 2. Context Integration
```go
// gofr/pkg/gofr/context.go
type Context struct {
    context.Context
    *container.Container  // ← Contains SQL (now Resolver)
    // ...
}

// Usage: ctx.SQL.QueryContext(...)
```

### 3. HTTP Middleware Integration
```go
// factory.go:180-191
app.UseMiddleware(createHTTPMiddleware())
```

### 4. Metrics Integration
```go
// resolver.go:122-138
r.metrics.NewHistogram("dbresolver_query_duration", ...)
r.metrics.NewGauge("dbresolver_primary_reads", ...)
r.metrics.NewGauge("dbresolver_replica_reads", ...)
```

### 5. Tracing Integration
```go
// resolver.go:258-272
tracedCtx, span := r.tracer.Start(ctx, fmt.Sprintf("dbresolver-%s", method))
span.SetAttributes(
    attribute.String("dbresolver.query", query),
    attribute.String("dbresolver.target", target),
    attribute.Bool("dbresolver.is_read", isRead),
)
```

---

## Sequence Diagrams

### Successful Read Query Flow

```
User Handler    Resolver      Strategy    Circuit Breaker    Replica DB
     │             │             │              │                │
     │ QueryContext│             │              │                │
     ├────────────>│             │              │                │
     │             │ shouldUseReplica()         │                │
     │             ├─────────────┐              │                │
     │             │<────────────┘              │                │
     │             │ (true)                    │                │
     │             │ selectHealthyReplica()     │                │
     │             ├───────────────────────────>│                │
     │             │             │ allowRequest()                │
     │             │             │<────────────┤                │
     │             │             │ (true)       │                │
     │             │             │ Choose()     │                │
     │             │<────────────┘              │                │
     │             │ (replica)                 │                │
     │             │ QueryContext()            │                │
     │             ├───────────────────────────────────────────>│
     │             │<───────────────────────────────────────────│
     │             │ (rows)                    │                │
     │             │ recordSuccess()            │                │
     │             │<────────────┘              │                │
     │<────────────│ (rows)                    │                │
```

### Failed Replica Query with Fallback

```
User Handler    Resolver      Circuit Breaker    Replica DB    Primary DB
     │             │              │                │              │
     │ QueryContext│              │                │              │
     ├────────────>│              │                │              │
     │             │ selectHealthyReplica()       │              │
     │             ├─────────────────────────────>│              │
     │             │              │ (allowed)     │              │
     │             │ QueryContext()               │              │
     │             ├─────────────────────────────>│              │
     │             │<──────────────────────────────│              │
     │             │ (error)      │                │              │
     │             │ recordFailure()               │              │
     │             ├──────────────────────────────┘              │
     │             │ fallbackToPrimary()                         │
     │             │ QueryContext()                              │
     │             ├───────────────────────────────────────────>│
     │             │<───────────────────────────────────────────│
     │             │ (rows)                                      │
     │<────────────│ (rows)                                      │
```

### Write Operation Flow

```
User Handler    Resolver      Primary DB
     │             │              │
     │ ExecContext()│              │
     ├─────────────>│              │
     │             │ (always primary)            │
     │             │ ExecContext()                │
     │             ├─────────────────────────────>│
     │             │<─────────────────────────────│
     │             │ (result)                    │
     │<────────────│ (result)                    │
```

---

## Key Design Patterns

1. **Strategy Pattern**: Load balancing strategies (RoundRobin, Random)
2. **Circuit Breaker Pattern**: Health checking for replicas
3. **Decorator Pattern**: Resolver wraps primary and replicas
4. **Context Pattern**: HTTP method/path passed via context
5. **Fallback Pattern**: Automatic fallback to primary on failure

---

## Configuration

### Environment Variables
- `DB_REPLICA_HOSTS`: Comma-separated list of `host:port` pairs
- `DB_REPLICA_USER`: Replica database user (defaults to `DB_USER`)
- `DB_REPLICA_PASSWORD`: Replica database password (defaults to `DB_PASSWORD`)
- `DB_REPLICA_MAX_IDLE_CONNECTIONS`: Max idle connections per replica
- `DB_REPLICA_MAX_OPEN_CONNECTIONS`: Max open connections per replica

### Programmatic Configuration
```go
cfg := dbresolver.Config{
    Strategy:      dbresolver.StrategyRoundRobin,
    ReadFallback:  true,
    MaxFailures:   5,
    TimeoutSec:    30,
    PrimaryRoutes: []string{"/admin/*", "/write"},
}
```

---

This document provides a complete view of the DBResolver architecture, flow, and code organization.


