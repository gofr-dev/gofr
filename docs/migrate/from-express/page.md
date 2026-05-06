---
description: "Migration guide for Node.js developers moving from Express to GoFr. JavaScript-to-Go mental model, handler translations, async/await analogues, and gradual adoption."
nextjs:
  metadata:
    title: "Express (Node.js) to GoFr Migration — JavaScript Devs Adopting Go"
    description: "Migration guide for Node.js developers moving from Express to GoFr. JavaScript-to-Go mental model, handler translations, async/await analogues, and gradual adoption."
---

# Migrate from Express (Node.js) to GoFr

{% answer %}
Coming from Express to GoFr is more than a framework migration — it's a language change. The mental model translates well: routing, middleware, request/response, and async I/O all have direct Go equivalents. Handlers go from `(req, res) => res.json(data)` to `func(c *gofr.Context) (any, error) { return data, nil }`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model translation

| Concept | Express / Node.js | GoFr / Go |
|---|---|---|
| Async runtime | Single-threaded event loop with `await` | Goroutines + channels (true concurrency) |
| Request handler | `(req, res, next) => {}` | `func(c *gofr.Context) (any, error)` |
| Middleware | `(req, res, next) => next()` | `func(http.Handler) http.Handler` |
| Body parsing | `express.json()` middleware | `c.Bind(&struct)` |
| Path params | `req.params.id` | `c.PathParam("id")` |
| Query params | `req.query.q` | `c.Param("q")` |
| JSON response | `res.json(data)` | `return data, nil` |
| Error handling | `next(err)` | `return nil, err` |
| Logging | Pino, Winston, Bunyan | Built into GoFr |
| Tracing | `@opentelemetry/instrumentation-express` | Built into GoFr |
| Database | pg, mongoose, ioredis | Built into GoFr (`c.SQL`, `c.Mongo`, `c.Redis`) |

## Hello world side-by-side

**Express:**
```js
import express from 'express'
const app = express()
app.use(express.json())

app.get('/hello', (req, res) => {
  res.json({ message: 'Hello, world' })
})

app.listen(8000)
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

## Async patterns

In Node, you `await` a database call. In Go, you call the function directly — concurrency is provided by goroutines, not callbacks or promises.

**Express:**
```js
app.get('/users/:id', async (req, res) => {
  const user = await db.getUser(req.params.id)
  res.json(user)
})
```

**GoFr:**
```go
app.GET("/users/{id}", func(c *gofr.Context) (any, error) {
    return db.GetUser(c.PathParam("id"))
})
```

The `c` (Context) carries deadline and cancellation just like JavaScript's `AbortController`, but is automatically propagated to all DB and HTTP calls.

## What you tend to gain

- **Static typing.** Request bodies, response shapes, and DB rows are typed; many Express runtime errors disappear at compile time.
- **Concurrency.** Goroutines + channels handle background work without async/await chains.
- **Single binary deploy.** No `node_modules`, no runtime dependency on Node version.
- **Built-in production glue.** Tracing, metrics, structured logging, datasource clients — Express requires you to assemble all of this.

## Common gotchas

- **No callback-style error propagation.** `next(err)` becomes `return nil, err`. Errors travel up the call stack; nothing happens implicitly.
- **No `req.body` mutation.** Bind into a struct and mutate the struct.
- **Goroutines leak silently if you don't `defer` cleanup.** A `defer rows.Close()` in your DB query is not optional in Go.
- **JSON shape is slightly different.** GoFr wraps successful responses as `{"data": ...}`. If Express clients expect the raw object, return a wrapper.
- **`process.env` becomes `app.Config.Get(key)`.** Configuration is loaded from `.env` files in the `configs/` directory by default.

## Estimated effort per service

A small Express service (10-20 routes, light DB usage) typically takes 2–4 engineering days for a developer new to Go. Most of the time goes to learning Go idioms (error handling, struct composition) rather than the framework itself.

## Recommended adoption

1. Pick a small, isolated Node service to rebuild in GoFr (an internal tool, a webhook receiver).
2. Match its endpoints 1:1.
3. Run both side-by-side in your traffic split or as separate environments.
4. Migrate larger services as your team builds confidence with Go.

{% faq %}

{% faq-item question="Will my JSON contracts change?" %}
GoFr wraps successful responses as `{"data": ...}` by default — and a plain struct returned from a handler is always wrapped. If your existing Express clients expect a different envelope (or no envelope), return one of GoFr's special response types instead: `response.Raw{Data: …}` writes the value directly with no envelope, and `response.Response` lets you control the shape. The wrapper is only bypassed when you return one of these typed responses, not when you return an arbitrary struct.
{% /faq-item %}

{% faq-item question="What about NestJS or Fastify users?" %}
NestJS users will find GoFr's structured approach familiar (controllers map to handlers, modules to packages). Fastify users will appreciate the lower runtime overhead.
{% /faq-item %}

{% /faq %}
