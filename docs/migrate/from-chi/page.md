---
description: "Migration guide for Go developers moving from chi router to GoFr framework. Handler signature, middleware, route groups, URL params, and the router-vs-framework trade-off."
nextjs:
  metadata:
    title: "Migrate from chi to GoFr — Go Router to Full Framework"
    description: "Migration guide for Go developers moving from chi router to GoFr framework. Handler signature, middleware, route groups, URL params, and the router-vs-framework trade-off."
---

# Migrate from chi to GoFr

{% answer %}
chi is a router; GoFr is a framework. Migrating means dropping a lot of glue you wrote yourself — logging, tracing, metrics, datasource pooling, health endpoints, retry/circuit-breaker on outbound calls — and accepting GoFr's opinions on response shape and configuration. Handlers change from `http.HandlerFunc` to `func(c *gofr.Context) (any, error)`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model: router vs framework

chi's design goal is "a thin, idiomatic, `net/http`-compatible router". You bring everything else: a logger, OpenTelemetry instrumentation, Prometheus middleware, your own datasource pools, your own retry library, your own health endpoint. That's a feature when you want full control. It becomes a tax when every microservice in your fleet ends up reassembling the same five libraries.

GoFr makes the opposite trade: an opinionated handler signature in exchange for built-in observability, datasource clients, resilience on outbound HTTP, and health out of the box. If your chi service is mostly your own glue around the router, the migration mostly deletes code.

## Handler translation

**chi:**
```go
r := chi.NewRouter()
r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    user, err := db.GetUser(id)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(user)
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

The path syntax (`{id}`) is identical. You no longer touch `http.ResponseWriter` directly for typical JSON responses.

## Request binding

chi has no body binding — you reach for `json.NewDecoder(r.Body).Decode(&v)`. In GoFr:

```go
var input CreateUser
if err := c.Bind(&input); err != nil {
    return nil, err
}
```

`c.Bind` handles JSON, form, and multipart.

## Param access

| Operation | chi | GoFr |
|---|---|---|
| Path param | `chi.URLParam(r, "id")` | `c.PathParam("id")` |
| Query param | `r.URL.Query().Get("q")` | `c.Param("q")` |
| Header | `r.Header.Get("X-Foo")` | `c.Request.Header.Get("X-Foo")` |
| Raw `*http.Request` | `r` | `c.Request` |

## Middleware

chi middleware is `func(http.Handler) http.Handler` — and so is GoFr's. Most chi middleware can be adapted by changing the registration call:

**chi:**
```go
r.Use(myMiddleware)
```

**GoFr:**
```go
app.UseMiddleware(myMiddleware)
```

You can usually delete chi middleware that exists only for cross-cutting infra (`chi/middleware.Logger`, `chi/middleware.Recoverer`, OTel/Prom adapters) — GoFr already provides those.

## Route groups and sub-routers

chi's `r.Route("/api/v1", func(r chi.Router) { ... })` pattern doesn't have a one-line equivalent in GoFr. The pragmatic translation is to register a path prefix per route, or wrap a small helper that closes over the prefix. For larger surfaces, model bounded contexts as separate handler structs and register their methods.

## Render package

If you used `go-chi/render` for `render.JSON(w, r, v)`, the GoFr equivalent is just `return v, nil`. Error responses are produced from `return nil, err` and shaped by GoFr's [error handling](/docs/advanced-guide/gofr-errors).

## Datasources

In a chi service you typically `sql.Open` yourself, manage a `*sql.DB`, set pool sizes, and instrument it. GoFr replaces all of that with:

```go
app.AddSQL(/* read from .env */)
app.AddRedis(...)
app.AddMongo(...)
```

Inside handlers, use `c.SQL`, `c.Redis`, `c.Mongo`. SQL (MySQL/Postgres/Oracle/SQLite/SQL Server), Redis, Mongo, Cassandra, ScyllaDB, Couchbase, ArangoDB, Dgraph, SurrealDB. SQL/Mongo/Redis/Dgraph migrations are first-class — see [migrations](/docs/advanced-guide/handling-data-migrations).

## Observability

Where a chi service typically wires `otelhttp`, `prometheus/promhttp`, a logger, and a `/healthz` endpoint by hand, GoFr ships OpenTelemetry tracing, Prometheus metrics at `/metrics`, structured JSON logs with trace IDs, and `/.well-known/health` automatically.

## Outbound HTTP

For service-to-service calls, instead of layering Hystrix-style libraries onto an `http.Client`:

```go
app.AddHTTPService("payments", "http://payments:8000")
```

Circuit breaker, retries, and rate limiting are configured per service.

## Gradual adoption

Run a new GoFr service alongside your chi services. From GoFr, call into the chi service via `app.AddHTTPService` with built-in resilience. Move endpoints across at your gateway one bounded context at a time.

{% faq %}

{% faq-item question="Can I run chi and GoFr in the same cluster?" %}
Yes — both are stateless Go HTTP servers. Bridge them via HTTP through `app.AddHTTPService` or via a shared Pub/Sub topic.
{% /faq-item %}

{% faq-item question="Will I lose chi's raw performance?" %}
GoFr uses a comparable router under the hood; the perf difference at typical service throughput is dwarfed by what your handlers and datasources do. The honest trade-off is opinionated response shape, not throughput.
{% /faq-item %}

{% faq-item question="Can I keep using `http.HandlerFunc`-style middleware?" %}
Yes — GoFr's `UseMiddleware` accepts `func(http.Handler) http.Handler`, so most chi middleware drops in unchanged.
{% /faq-item %}

{% /faq %}
