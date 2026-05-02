---
description: "Migration guide for Go developers moving from Echo to GoFr. Handler signature, middleware, route groups, binding, and gradual adoption with side-by-side examples."
nextjs:
  metadata:
    title: "Migrate from Echo to GoFr — Go Router to Full Framework"
    description: "Migration guide for Go developers moving from Echo to GoFr. Handler signature, middleware, route groups, binding, and gradual adoption with side-by-side examples."
---

# Migrate from Echo to GoFr

{% answer %}
Echo handlers translate to GoFr almost line-for-line. The handler signature changes from `func(c echo.Context) error` (where you call `c.JSON(status, value)`) to `func(c *gofr.Context) (any, error)` — you return the value and any error, and GoFr writes the response. Echo's `MiddlewareFunc` becomes the standard `func(http.Handler) http.Handler`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

If you're still picking between the two, see [GoFr vs Echo](/comparison/gofr-vs-echo) for a non-migration comparison.

## Mental model

Echo gives you a fast router and a thin `Context`; everything else (logging, metrics, tracing, datasource wiring, health, retries, circuit breaker on outbound calls) you assemble yourself. GoFr is a framework: the same routing surface, plus those operational pieces wired in. Migrating is mostly about deleting code you no longer need.

## Handler translation

**Echo:**
```go
e := echo.New()
e.GET("/users/:id", func(c echo.Context) error {
    id := c.Param("id")
    user, err := db.GetUser(id)
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, err.Error())
    }
    return c.JSON(http.StatusOK, user)
})
```

**GoFr:**
```go
app := gofr.New()
app.GET("/users/{id}", func(c *gofr.Context) (any, error) {
    id := c.PathParam("id")
    return db.GetUser(id)
})
```

Note the path syntax: Echo uses `:id`, GoFr uses `{id}`.

## Request binding

**Echo:**
```go
var input CreateUser
if err := c.Bind(&input); err != nil {
    return err
}
```

**GoFr:**
```go
var input CreateUser
if err := c.Bind(&input); err != nil {
    return nil, err
}
```

Both accept JSON, form, and multipart. Validation isn't built into either — pair with `go-playground/validator` if you want tag-based rules.

## Path, query, and header access

| Operation | Echo | GoFr |
|---|---|---|
| Path param | `c.Param("id")` | `c.PathParam("id")` |
| Query param | `c.QueryParam("q")` | `c.Param("q")` |
| Header | `c.Request().Header.Get("X-Foo")` | `c.Request.Header.Get("X-Foo")` |
| Raw request | `c.Request()` | `c.Request` |

## Middleware

**Echo:**
```go
e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        start := time.Now()
        err := next(c)
        log.Printf("%s took %s", c.Path(), time.Since(start))
        return err
    }
})
```

**GoFr:**
```go
app.UseMiddleware(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s took %s", r.URL.Path, time.Since(start))
    })
})
```

In practice you rarely need this — request logging, tracing, and Prometheus metrics are already wired.

## Route groups

Echo's `e.Group("/api/v1")` does not have a one-line GoFr equivalent. The most common idiom is to register a path prefix on each route, or wrap a registration helper that closes over the prefix.

## Static files and templates

Echo's `e.Static` and `e.Renderer` are replaced by GoFr's static file serving — drop assets in the configured static directory. Templating libraries (text/template, html/template) work as usual inside handlers.

## Datasources

Echo leaves datasource wiring to you. GoFr ships clients you register on the app once:

```go
app.AddSQL(/* read from .env */)
app.AddRedis(...)
app.AddMongo(...)
```

You then access them via `c.SQL`, `c.Redis`, `c.Mongo` in handlers. SQL (MySQL/Postgres/Oracle/SQLite/SQL Server), Redis, Mongo, Cassandra, ScyllaDB, Couchbase, ArangoDB, Dgraph, SurrealDB are supported, with first-class migrations for SQL/Mongo/Redis/Dgraph. See [datasources](/docs/datasources).

## Observability

Echo users typically integrate `echoprometheus`, `otelecho`, and a logger of their choice. With GoFr these are built-in: OpenTelemetry tracing, Prometheus metrics at `/metrics`, structured JSON logs with trace IDs, health at `/.well-known/health`, and runtime log-level changes.

## Libraries you can typically remove

- `otelecho` middleware → built-in tracing.
- `echoprometheus` → built-in metrics.
- Hand-rolled `/healthz` → `/.well-known/health` is auto-exposed.
- Custom retry / circuit-breaker code on outbound calls → `app.AddHTTPService`.

## Gradual adoption

Run new endpoints in a GoFr service alongside Echo. Call back into the Echo service from GoFr through `app.AddHTTPService("legacy", baseURL)` with circuit breaker, retries, and rate limiting configured. Migrate routes in batches grouped by data dependency.

{% faq %}

{% faq-item question="Can I run Echo and GoFr in the same cluster?" %}
Yes. They are independent Go binaries. Use `app.AddHTTPService` from the GoFr side to call the Echo service with built-in resilience.
{% /faq-item %}

{% faq-item question="Will my Echo middleware drop in unchanged?" %}
No — the signatures differ (`echo.MiddlewareFunc` vs `func(http.Handler) http.Handler`). The logic translates directly; only the wrapper changes.
{% /faq-item %}

{% faq-item question="Does GoFr have a faster router than Echo?" %}
Performance is comparable for most workloads — GoFr trades raw routing micro-benchmarks for built-in observability, datasources, and resilience. Pick based on what your team should not be writing themselves.
{% /faq-item %}

{% /faq %}
