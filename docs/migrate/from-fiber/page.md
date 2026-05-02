---
description: "Step-by-step migration from Fiber to GoFr. Handler translations, fasthttp-to-net/http transition, datasource integration, and observability mapping."
nextjs:
  metadata:
    title: "Migrate from Fiber to GoFr — Code Translations"
    description: "Step-by-step migration from Fiber to GoFr. Handler translations, fasthttp-to-net/http transition, datasource integration, and observability mapping."
---

# Migrate from Fiber to GoFr

{% answer %}
Migrating from Fiber to GoFr also moves you from `fasthttp` to `net/http`. This is usually a simplification — `net/http`-compatible libraries become directly usable, and middleware translation is straightforward. Handlers go from `func(c *fiber.Ctx) error` to `func(c *gofr.Context) (any, error)`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Handler translation

**Fiber:**
```go
app.Get("/users/:id", func(c *fiber.Ctx) error {
    id := c.Params("id")
    user, err := db.GetUser(id)
    if err != nil {
        return c.Status(404).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(user)
})
```

**GoFr:**
```go
app.GET("/users/{id}", func(c *gofr.Context) (any, error) {
    id := c.PathParam("id")
    user, err := db.GetUser(id)
    return user, err
})
```

## Request body and params

| Operation | Fiber | GoFr |
|---|---|---|
| Path param | `c.Params("id")` | `c.PathParam("id")` |
| Query param | `c.Query("q")` | `c.Param("q")` |
| Body parse | `c.BodyParser(&input)` | `c.Bind(&input)` |

## Middleware

Fiber middleware is `fiber.Handler`. GoFr middleware is the standard `net/http` `func(http.Handler) http.Handler`. Most third-party `net/http` middleware works directly with GoFr.

## Observability and datasources

This is where the migration pays off most. In Fiber:

- Tracing → install `otelfiber`, configure exporter, propagate spans manually for DB calls.
- Metrics → install `fiber/v2/middleware/monitor` or expose Prometheus separately.
- Database → use `database/sql` or driver of choice; instrument it yourself.

In GoFr:

- Tracing, metrics, and structured logging are emitted by default.
- DB clients (`c.SQL`, `c.Redis`, `c.Mongo`, etc.) are auto-instrumented with span correlation.

## net/http compatibility

If your Fiber service used `adaptor.HTTPHandler` to wrap `net/http` middleware, those adapters become unnecessary in GoFr — `net/http` is native. Drop them.

## Common gotchas

- **fasthttp libraries don't work with `net/http`.** If you depend on `valyala/fasthttp`-specific packages, plan to swap each for a `net/http` equivalent.
- **`c.Locals` has no direct equivalent.** `*gofr.Context` does not expose `Set` / `Get` methods for per-request locals. Either pass values through Go closures, or — since `*gofr.Context` embeds `context.Context` — use `context.WithValue(c, key, value)` and retrieve with `c.Value(key)`.
- **`adaptor.HTTPHandler`** wrappers you used to call `net/http` middleware from Fiber are now unnecessary — drop them.
- **Streaming response patterns differ.** GoFr does not ship a built-in SSE responder; for raw streaming, write to the underlying `http.ResponseWriter` from a custom middleware.
- **Compression / static-file middleware** that you composed in Fiber needs to be re-added explicitly in GoFr if you relied on it.

## Estimated effort

A typical Fiber-based REST service migrates in 1–2 engineering days. The biggest unknown is whether any of your dependencies are fasthttp-only.

## Recommended order

1. Migrate one new service to GoFr first.
2. Validate datasource clients connect to existing databases.
3. Confirm OTel traces and Prometheus metrics reach existing collectors.
4. Migrate remaining services as you touch them.

{% faq %}

{% faq-item question="Does GoFr support Fiber's request lifecycle features (Locals, etc.)?" %}
There is no `c.Locals`-style per-request locals API on `*gofr.Context`. Pass values through closures, or use the standard `context.Context` mechanism — `*gofr.Context` embeds `context.Context`, so `context.WithValue(c, key, value)` and `c.Value(key)` work.
{% /faq-item %}

{% faq-item question="Can I keep using fasthttp libraries with GoFr?" %}
No — GoFr is `net/http`-based. Libraries written for fasthttp won't work directly. Most have `net/http`-equivalent alternatives.
{% /faq-item %}

{% /faq %}
