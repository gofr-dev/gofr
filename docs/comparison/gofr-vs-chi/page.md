---
description: "GoFr vs Chi: Chi is an idiomatic, minimal net/http router favored for composability. GoFr is a full microservice framework with built-in observability, 15+ datasources, gRPC, GraphQL, and Pub/Sub."
nextjs:
  metadata:
    title: "GoFr vs Chi — Minimal Router vs Microservice Framework"
    description: "GoFr vs Chi: Chi is an idiomatic, minimal net/http router favored for composability. GoFr is a full microservice framework with built-in observability, 15+ datasources, gRPC, GraphQL, and Pub/Sub."
---

# GoFr vs Chi

{% answer %}
**Chi** is a small, idiomatic `net/http`-compatible router that composes beautifully with the standard library — a great fit when minimal dependencies and full control matter. **GoFr** has a wider scope: HTTP routing alongside gRPC, GraphQL, WebSockets, Pub/Sub, cron, migrations, OpenTelemetry tracing, Prometheus metrics, structured logging, datasource clients, and a service-to-service HTTP client with circuit breakers. Different goals, both open source — both have happy users.
{% /answer %}

## What Chi is great at

- **Idiomatic Go** — `func(http.ResponseWriter, *http.Request)` everywhere; zero magic.
- **Lightweight** — small dependency footprint, fast.
- **Composable** — works seamlessly with `net/http` middleware, the standard library, and any third-party `net/http`-compatible library.
- **Maintained by go-chi/chi** — well-respected in the Go community.

## Where the projects differ

Chi takes no position on how you structure your service or which libraries you bring for logging, tracing, datasources, or downstream calls — that's a strength when you want full control and a small dependency footprint. GoFr takes the opposite design choice: it standardizes a common combination of those layers (OpenTelemetry, Prometheus, structured logging, datasource clients with retries, message brokers, circuit breakers, health checks) so teams maintaining several services don't make the same composition choices repeatedly. Both approaches have their place.

### Side-by-side: a service that calls a database and emits a trace

**Chi (with manual wiring):**
```go
import (
    "database/sql"
    "log/slog"

    "github.com/go-chi/chi/v5"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "go.opentelemetry.io/otel"
    // ... otel exporter setup, prometheus registry setup, db driver, slog setup
)

// You write tracer init, metrics init, logger init, DB connection,
// then wrap your handler with otelhttp, register Prom on /metrics,
// and propagate a request-scoped logger.
```

**GoFr:**
```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()
    app.GET("/users/{id}", func(c *gofr.Context) (any, error) {
        var name string
        err := c.SQL.QueryRowContext(c, "SELECT name FROM users WHERE id=?", c.PathParam("id")).Scan(&name)
        return map[string]string{"name": name}, err
    })
    app.Run()
}
```
Tracing, metrics, structured logging with trace IDs, and DB span correlation are emitted automatically.

## When GoFr might be a good fit

- You're maintaining several services and the same wiring keeps reappearing in each.
- You'd like gRPC, Pub/Sub, GraphQL, or WebSockets alongside HTTP under one framework.
- Auto-instrumented database clients fit your operational model.
- Consistent configuration and observability defaults matter to you across multiple services.

{% faq %}

{% faq-item question="Can I use Chi-style net/http middleware in GoFr?" %}
Yes. GoFr's `UseMiddleware` accepts `func(http.Handler) http.Handler` — the standard `net/http` signature Chi uses.
{% /faq-item %}

{% faq-item question="Does GoFr support route patterns like Chi's?" %}
GoFr supports path parameters, wildcards, and method-specific routing. The exact syntax differs slightly; see the routing reference.
{% /faq-item %}

{% /faq %}
