---
description: "Step-by-step migration from Gin to GoFr. Handler signature, middleware, binding, route groups, error handling, and gradual adoption strategy with side-by-side code examples."
nextjs:
  metadata:
    title: "Migrate from Gin to GoFr — Code Translations and Examples"
    description: "Step-by-step migration from Gin to GoFr. Handler signature, middleware, binding, route groups, error handling, and gradual adoption strategy with side-by-side code examples."
---

# Migrate from Gin to GoFr

{% answer %}
Gin handlers translate to GoFr cleanly. The biggest mental shift is the handler signature: `func(c *gin.Context)` becomes `func(c *gofr.Context) (any, error)` — you return data and an error instead of calling `c.JSON(status, value)`. Middleware uses the standard `net/http` signature instead of Gin's `gin.HandlerFunc`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Handler translation

**Gin:**
```go
r.GET("/users/:id", func(c *gin.Context) {
    id := c.Param("id")
    user, err := db.GetUser(id)
    if err != nil {
        c.JSON(404, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, user)
})
```

**GoFr:**
```go
app.GET("/users/{id}", func(c *gofr.Context) (any, error) {
    id := c.PathParam("id")
    user, err := db.GetUser(id)
    if err != nil {
        return nil, err
    }
    return user, nil
})
```

## Request binding

**Gin:**
```go
var input CreateUser
if err := c.ShouldBindJSON(&input); err != nil {
    c.JSON(400, gin.H{"error": err.Error()})
    return
}
```

**GoFr:**
```go
var input CreateUser
if err := c.Bind(&input); err != nil {
    return nil, err
}
```

## Query and path parameters

| Operation | Gin | GoFr |
|---|---|---|
| Path param | `c.Param("id")` | `c.PathParam("id")` |
| Query param | `c.Query("q")` | `c.Param("q")` |
| Default query | `c.DefaultQuery("page", "1")` | `c.Param("page")` (handle empty case) |

## Middleware

**Gin:**
```go
r.Use(func(c *gin.Context) {
    start := time.Now()
    c.Next()
    log.Printf("%s took %s", c.Request.URL.Path, time.Since(start))
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

In practice you rarely need this in GoFr — request logging, tracing, and metrics are built in.

## Libraries you can typically remove

After moving to GoFr, several Gin-side helpers usually become unnecessary because the framework already includes equivalents — keep whatever you'd still rather wire yourself:

- `otelgin` middleware → built-in tracing.
- `gin-prometheus` → built-in metrics at `/metrics`.
- `zap-gin` request logging → built-in structured logging with trace IDs.
- Manual `db.Ping()` / health endpoints → auto-exposed at `/.well-known/health`.
- Custom retry / circuit-breaker code on outbound HTTP calls → `app.AddHTTPService` with config.

## Common gotchas

- **`c.MustGet` has no direct equivalent.** Use `c.Get(key)` and handle the missing-value case explicitly.
- **Gin's middleware ordering matters at registration time.** GoFr's default observability middleware runs before your custom `UseMiddleware` chain — assume tracing and metrics are already wired by the time your code runs.
- **Response wrapping is different.** GoFr returns `{"data": ...}` on success and `{"error": ...}` on error. If your existing clients expect the raw object, return a wrapper struct that controls the envelope.
- **No `gin.H{}`.** Use plain `map[string]any{}` or, better, named structs.
- **Validation isn't built in.** Gin uses `binding:"required"` tags via go-playground/validator by default. With GoFr, pick your validator explicitly.

## Estimated effort

A typical 5-10 endpoint Gin service migrates in 1–2 engineering days. Most of the time goes to validating that observability output (traces, metrics) lands in your existing stack with the right names — not to handler translation.

## Recommended order

1. Move one endpoint to GoFr in a new file/service.
2. Validate observability (traces and metrics) reach your existing collectors.
3. Port remaining endpoints in batches grouped by data dependency.
4. Drop now-redundant Gin middleware libraries.
5. Decommission the old service when traffic has shifted.

{% faq %}

{% faq-item question="What happens to my existing tests?" %}
GoFr provides testing utilities — see the [testing reference](/docs/references/testing). Most Gin tests rewrite naturally because the handler logic is similar; the test setup changes.
{% /faq-item %}

{% faq-item question="Does GoFr support all of Gin's binding tags?" %}
GoFr's Bind handles JSON, form, and multipart. Validation is left to the choice of library (e.g., go-playground/validator on bound structs).
{% /faq-item %}

{% /faq %}
