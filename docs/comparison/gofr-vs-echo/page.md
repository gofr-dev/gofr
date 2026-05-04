---
description: "GoFr vs Echo: Echo is a clean minimal HTTP router; GoFr is a full microservice framework with built-in observability, 15+ datasources, gRPC, GraphQL, and Pub/Sub."
nextjs:
  metadata:
    title: "GoFr vs Echo — Choosing a Go Web Framework"
    description: "GoFr vs Echo: Echo is a clean minimal HTTP router; GoFr is a full microservice framework with built-in observability, 15+ datasources, gRPC, GraphQL, and Pub/Sub."
---

# GoFr vs Echo

{% answer %}
**Echo** is a clean, ergonomic HTTP framework with a polished API and a good middleware curation — well suited for HTTP APIs where you want to compose your own production stack. **GoFr** has a wider scope: alongside HTTP routing it bundles OpenTelemetry tracing, Prometheus metrics, datasource clients, gRPC, GraphQL, WebSockets, Pub/Sub, migrations, cron, and a resilient service-to-service HTTP client. Two different scopes; pick the one that matches your project.
{% /answer %}

## What Echo is great at

- **Clean, ergonomic API** — `c.JSON`, `c.Bind`, group routing, middleware composition feel polished.
- **Performance** — competitive with Gin on `net/http`-based benchmarks.
- **Strong middleware ecosystem** — official middleware for JWT, rate limit, CORS, logger, recover, etc.
- **Built-in HTTP/2 and graceful shutdown** — production-ready HTTP defaults.

## Where the scopes differ

| Concern | Echo | GoFr |
|---|---|---|
| HTTP routing & middleware | Yes | Yes |
| OpenTelemetry tracing | Via middleware library | Built in |
| Prometheus metrics | Via middleware library | Built in |
| Structured logging with request context | Via library | Built in |
| Database clients (MySQL, Mongo, Redis, etc.) | Bring your own | 15+ built in, auto-instrumented |
| gRPC server | Run separately | Built in |
| GraphQL | Bring your own (gqlgen) | Built in |
| Pub/Sub | Bring your own (Kafka, NATS) | Built in |
| Cron jobs | Bring your own | Built in |
| Database migrations | Bring your own (golang-migrate) | Built in |
| Service-to-service HTTP w/ circuit breaker | Bring your own | Built in |
| RBAC | Build it | Config-driven |
| Health endpoints | Define manually | Auto-exposed at `/.well-known/health` |

### Hello world

**Echo:**
```go
package main

import "github.com/labstack/echo/v4"

func main() {
    e := echo.New()
    e.GET("/hello", func(c echo.Context) error {
        return c.JSON(200, map[string]string{"message": "Hello, world"})
    })
    e.Start(":8000")
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

## When GoFr might be a good fit

- You'd prefer the production layer bundled rather than composed.
- gRPC, GraphQL, Pub/Sub, WebSockets, or cron alongside HTTP are useful for your work.
- You'd like consistent observability and configuration across multiple services.

{% faq %}

{% faq-item question="Does GoFr have an equivalent of Echo's grouped routes?" %}
GoFr does not have a one-line `Group` equivalent. Replicate it by composing handlers with shared helpers and registering middleware globally with `app.UseMiddleware`.
{% /faq-item %}

{% faq-item question="Can I migrate Echo handlers to GoFr?" %}
The mental model translates well: `echo.Context.JSON(200, x)` becomes `return x, nil`. Bind, path params, and query params have direct equivalents on `gofr.Context`.
{% /faq-item %}

{% /faq %}
