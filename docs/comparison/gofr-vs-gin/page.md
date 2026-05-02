---
description: "GoFr vs Gin: when to use a minimal HTTP router (Gin) versus a full microservice framework (GoFr) with observability, datasources, gRPC, and pub/sub."
nextjs:
  metadata:
    title: "GoFr vs Gin — Choosing a Go Framework for Microservices"
    description: "GoFr vs Gin: when to use a minimal HTTP router (Gin) versus a full microservice framework (GoFr) with observability, datasources, gRPC, and pub/sub."
---

# GoFr vs Gin

{% answer %}
**Gin** is a fast, minimal HTTP router with a familiar API and a mature middleware ecosystem — a great fit when you want a thin router and to compose the rest of your stack yourself. **GoFr** has a wider scope: alongside HTTP routing it bundles OpenTelemetry tracing, Prometheus metrics, structured logging, datasource clients, gRPC, GraphQL, WebSockets, Pub/Sub, migrations, cron, circuit breakers, and health checks. Two different trade-offs; both are open source.
{% /answer %}

## What Gin is great at

- **Performance** — minimal overhead on top of `net/http`, fast routing.
- **Familiar API** — `c.JSON`, `c.Bind`, `c.Param` patterns are intuitive.
- **Mature middleware ecosystem** — community packages for almost everything.
- **Stable, large community** — battle-tested in production.

## Where the projects differ

Gin is intentionally focused on routing. Anything beyond routing — observability, database access, message brokers, retries, circuit breakers, health checks — is something you compose by picking libraries you trust. That's a deliberate strength when you want full control. GoFr takes the opposite design choice: it bundles a common production layer behind one configuration surface so teams maintaining several services don't make those composition choices repeatedly. Neither is universally better — pick the one that matches how your team prefers to work.

### Hello world side-by-side

**Gin:**
```go
package main

import "github.com/gin-gonic/gin"

func main() {
    r := gin.Default()
    r.GET("/hello", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello, world"})
    })
    r.Run(":8000")
}
```

**GoFr:**
```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()
    app.GET("/hello", func(c *gofr.Context) (any, error) {
        return "Hello, world", nil
    })
    app.Run()
}
```

### Adding tracing, metrics, and a Postgres connection

**Gin** — pull in `otelgin`, `otelhttp`, `prometheus/client_golang`, `pgx`. Configure each. Wire them together. Make sure trace IDs propagate from request → DB query.

**GoFr** — set `TRACER_HOST`, `METRICS_PORT`, and `DB_HOST` in `.env`. Call `c.SQL` to query. Traces, metrics, and structured logs are emitted automatically.

### Service-to-service HTTP with circuit breaker

```go
// Register a downstream service once at startup:
app.AddHTTPService("payments", "https://payments.internal")

// Inside any handler, look it up via the request context:
func chargeHandler(ctx *gofr.Context) (any, error) {
    resp, err := ctx.GetHTTPService("payments").Get(ctx, "/charge", nil)
    // ...
}
```
Circuit breaker, retry, rate limit, connection pool, and auth are configurable through the service registration.

### gRPC, Pub/Sub, cron, WebSockets

```go
app.RegisterService(serviceDesc, impl)      // gRPC
app.Subscribe("orders", orderHandler)       // Pub/Sub (Kafka, NATS, etc.)
app.AddCronJob("0 * * * *", "billing", run) // Cron
app.WebSocket("/stream", wsHandler)         // WebSocket
```

## When GoFr might be a good fit

- You'd prefer tracing, metrics, and structured logs available by default rather than composed.
- You'd like gRPC, GraphQL, Pub/Sub, or WebSockets alongside HTTP under one framework.
- You maintain several similar services and would rather standardize the production wiring once.
- You're deploying to Kubernetes and want health checks, graceful shutdown, and consistent configuration as defaults.

## Migration

Already on Gin? See the [Migrate from Gin guide](/migrate/from-gin) for concrete code translations.

{% faq %}

{% faq-item question="Can I use Gin middleware in GoFr?" %}
Not directly — GoFr has its own middleware signature `func(http.Handler) http.Handler` which is the standard `net/http` pattern, not Gin's `gin.HandlerFunc`. Translating a typical Gin middleware is straightforward; see the migration guide.
{% /faq-item %}

{% faq-item question="Does GoFr support all of Gin's request binding?" %}
GoFr supports JSON, form, multipart, path params, and query params via `ctx.Bind`, `ctx.PathParam`, `ctx.Param`. Validation is left to the choice of library.
{% /faq-item %}

{% /faq %}
