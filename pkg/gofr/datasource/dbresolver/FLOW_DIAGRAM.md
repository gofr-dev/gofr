# DBResolver Code Flow Diagram

## Quick Reference: Code Paths

### Initialization Path
```
main()
  └─> gofr.New()
       └─> container.NewContainer()
            └─> sql.NewSQL()  [Primary DB]
                 └─> container.SQL = primaryDB

  └─> dbresolver.NewProvider(strategy, fallback)
       └─> ResolverProvider{app, cfg}

  └─> app.AddDBResolver(provider)
       └─> external_db.go:225
            ├─> Validate primary exists (line 227)
            ├─> provider.UseLogger() (line 232)
            ├─> provider.UseMetrics() (line 233)
            ├─> provider.UseTracer() (line 235)
            └─> provider.Connect() (line 238)
                 └─> factory.go:77
                      ├─> Get primary: app.GetSQL() (line 79)
                      ├─> Create replicas (line 100)
                      │    └─> factory.go:194
                      │         ├─> Parse DB_REPLICA_HOSTS (line 195)
                      │         └─> For each host:port:
                      │              ├─> createReplicaConnection() (line 219)
                      │              │    └─> sql.NewSQL(replicaCfg, ...)
                      │              └─> Wrap with circuit breaker
                      │                   └─> resolver.go:75-82
                      │
                      ├─> Build primary routes map (line 106-110)
                      ├─> Create strategy (line 112)
                      │    └─> strategy.go:34-36 or 65-67
                      │
                      └─> NewResolver() (line 115)
                           └─> resolver.go:73
                                ├─> Wrap replicas (line 75-82)
                                ├─> Initialize stats (line 90)
                                ├─> Set strategy (line 96-98)
                                ├─> Apply options (line 101-103)
                                ├─> Initialize metrics (line 112)
                                └─> Start background tasks (line 113)

  └─> Replace container.SQL (external_db.go:241)
       └─> container.SQL = resolver
```

### HTTP Request Path
```
HTTP Request
  └─> http.Server
       └─> Router
            └─> Handler
                 └─> HTTP Middleware (if registered)
                      └─> factory.go:181-190
                           ├─> Extract r.Method
                           ├─> Extract r.URL.Path
                           ├─> ctx = WithHTTPMethod(ctx, method)
                           ├─> ctx = WithRequestPath(ctx, path)
                           └─> r = r.WithContext(ctx)

                 └─> handler.ServeHTTP()
                      └─> handler.go:55
                           ├─> newContext(responder, request, container)
                           └─> h.function(c)  [User handler]
                                └─> ctx.SQL.QueryContext(...)
                                     └─> resolver.QueryContext()
                                          └─> resolver.go:302
```

### Query Execution Path (Read)
```
ctx.SQL.QueryContext(ctx, query, args)
  └─> resolver.QueryContext()  [resolver.go:302]
       ├─> start = time.Now()
       ├─> stats.totalQueries.Add(1)
       ├─> addTrace() → OpenTelemetry span
       │    └─> resolver.go:258
       │
       └─> shouldUseReplica(ctx)  [resolver.go:177]
            ├─> Check: len(replicas) == 0? → false
            ├─> Check: isPrimaryRoute(path)? → false
            ├─> Extract: method = ctx.Value(contextKeyHTTPMethod)
            └─> Return: method == "GET" || "HEAD" || "OPTIONS"

       ├─> IF useReplica && len(replicas) > 0:
       │    └─> executeReplicaQuery()  [resolver.go:323]
       │         ├─> selectHealthyReplica()  [resolver.go:215]
       │         │    ├─> For each replica:
       │         │    │    └─> wrapper.breaker.allowRequest()
       │         │    │         └─> circuit_breaker.go:38
       │         │    │              ├─> Check state (CLOSED/OPEN/HALF_OPEN)
       │         │    │              └─> Return true/false
       │         │    │
       │         │    ├─> Filter healthy replicas
       │         │    └─> strategy.Choose(availableDbs)
       │         │         └─> strategy.go:41 (RoundRobin)
       │         │              ├─> count = current.Add(1)
       │         │              └─> return replicas[count % len]
       │         │
       │         ├─> IF wrapper == nil:
       │         │    └─> fallbackToPrimary()  [resolver.go:353]
       │         │         ├─> Check: readFallback enabled?
       │         │         ├─> stats.primaryFallbacks.Add(1)
       │         │         └─> primary.QueryContext()
       │         │
       │         └─> wrapper.db.QueryContext(ctx, query, args)
       │              ├─> IF success:
       │              │    ├─> stats.replicaReads.Add(1)
       │              │    ├─> wrapper.breaker.recordSuccess()
       │              │    │    └─> circuit_breaker.go:60
       │              │    │         ├─> failures.Store(0)
       │              │    │         └─> state = CLOSED
       │              │    └─> recordStats("replica")
       │              │
       │              └─> IF error:
       │                   ├─> wrapper.breaker.recordFailure()
       │                   │    └─> circuit_breaker.go:71
       │                   │         ├─> failures.Add(1)
       │                   │         └─> IF failures >= max: state = OPEN
       │                   ├─> stats.replicaFailures.Add(1)
       │                   └─> fallbackToPrimary()
       │
       └─> ELSE (use primary):
            ├─> stats.primaryWrites.Add(1)
            └─> primary.QueryContext(ctx, query, args)
```

### Query Execution Path (Write)
```
ctx.SQL.ExecContext(ctx, query, args)
  └─> resolver.ExecContext()  [resolver.go:414]
       ├─> start = time.Now()
       ├─> stats.primaryWrites.Add(1)
       ├─> stats.totalQueries.Add(1)
       ├─> addTrace() → OpenTelemetry span
       └─> primary.ExecContext(ctx, query, args)
            └─> Always routes to primary (no routing decision)
```

### Circuit Breaker State Machine
```
CLOSED (Healthy)
  │
  ├─> Failure occurs
  │   └─> recordFailure()
  │       ├─> failures++
  │       └─> IF failures >= maxFailures:
  │            └─> State = OPEN
  │
OPEN (Unhealthy)
  │
  ├─> Timeout passes
  │   └─> allowRequest() checks timeout
  │       └─> IF time.Since(lastFailure) > timeout:
  │            └─> Try transition: OPEN → HALF_OPEN
  │
HALF_OPEN (Testing)
  │
  ├─> Request succeeds
  │   └─> recordSuccess()
  │       ├─> failures = 0
  │       └─> State = CLOSED
  │
  └─> Request fails
      └─> recordFailure()
          └─> State = OPEN
```

## Code File Organization

```
gofr/pkg/gofr/datasource/dbresolver/
├── resolver.go          [Main resolver implementation]
│   ├── NewResolver()           Line 73
│   ├── QueryContext()          Line 302
│   ├── ExecContext()           Line 414
│   ├── shouldUseReplica()      Line 177
│   ├── selectHealthyReplica()  Line 215
│   ├── executeReplicaQuery()   Line 323
│   └── fallbackToPrimary()     Line 353
│
├── factory.go          [Provider and initialization]
│   ├── NewDBResolverProvider() Line 52
│   ├── Connect()               Line 77
│   ├── createReplicas()        Line 194
│   ├── InitDBResolver()         Line 167
│   └── createHTTPMiddleware()   Line 180
│
├── strategy.go         [Load balancing strategies]
│   ├── RoundRobinStrategy      Line 30
│   │   └── Choose()            Line 41
│   └── RandomStrategy          Line 62
│       └── Choose()            Line 70
│
├── circuit_breaker.go  [Circuit breaker implementation]
│   ├── newCircuitBreaker()     Line 25
│   ├── allowRequest()          Line 38
│   ├── recordSuccess()         Line 60
│   └── recordFailure()         Line 71
│
├── options.go          [Resolver options]
│   ├── WithStrategy()
│   ├── WithFallback()
│   └── WithPrimaryRoutes()
│
├── logger.go          [Logger interface]
├── metrics.go         [Metrics interface]
└── README.md          [Documentation]
```

## Integration Points

### 1. GoFr App Integration
```
gofr/pkg/gofr/
├── external_db.go
│   └── AddDBResolver()  [Line 225]
│       └─> Replaces container.SQL
│
├── handler.go
│   └── ServeHTTP()  [Line 55]
│       └─> Creates gofr.Context
│
└── context.go
    └── Context struct
        └─> *container.Container
            └─> SQL container.DB  [Now Resolver]
```

### 2. Container Integration
```
gofr/pkg/gofr/container/
└── datasources.go
    └── Container struct
        └─> SQL container.DB  [Interface]
            └─> Implemented by Resolver
```

### 3. SQL Package Integration
```
gofr/pkg/gofr/datasource/sql/
├── sql.go
│   └── NewSQL()  [Creates primary/replica connections]
│
└── db.go
    └── DB struct  [Implements container.DB]
        └─> Used by Resolver
```

## Key Decision Points

### 1. Routing Decision
**Location**: `resolver.go:177-197`
```go
shouldUseReplica(ctx)
  ├─> No replicas? → PRIMARY
  ├─> Path in primaryRoutes? → PRIMARY
  ├─> Method not in context? → PRIMARY (safety)
  ├─> Method == GET/HEAD/OPTIONS? → REPLICA
  └─> Otherwise → PRIMARY
```

### 2. Replica Selection
**Location**: `resolver.go:215-255`
```go
selectHealthyReplica()
  ├─> Filter by circuit breaker
  ├─> IF no healthy replicas → nil (fallback)
  ├─> Apply strategy (RoundRobin/Random)
  └─> Return selected replica wrapper
```

### 3. Fallback Decision
**Location**: `resolver.go:353-373`
```go
fallbackToPrimary()
  ├─> IF readFallback == false → ERROR
  ├─> Update stats
  └─> Execute on primary
```

## Error Handling Flow

```
Query Execution
  ├─> Replica selection fails
  │   └─> fallbackToPrimary()
  │       └─> IF fallback disabled → errReplicaFailedNoFallback
  │
  ├─> Replica query fails
  │   ├─> recordFailure() → Circuit breaker
  │   └─> fallbackToPrimary()
  │
  ├─> Primary fallback fails
  │   └─> Return error to user
  │
  └─> All circuit breakers open
      └─> All queries fallback to primary
```

## Metrics Collection Flow

```
Background Task (resolver.go:148-162)
  └─> Every 30 seconds
       └─> updateMetrics()
            ├─> SetGauge("dbresolver_primary_reads")
            ├─> SetGauge("dbresolver_primary_writes")
            ├─> SetGauge("dbresolver_replica_reads")
            ├─> SetGauge("dbresolver_fallbacks")
            └─> SetGauge("dbresolver_failures")

Per-Query Metrics (resolver.go:274-294)
  └─> recordStats()
       └─> RecordHistogram("dbresolver_query_duration")
            └─> With attributes: method, target
```

This flow diagram provides a complete code-level view of how DBResolver works.


