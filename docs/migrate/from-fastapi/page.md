---
description: "Migration guide for Python developers moving from FastAPI to GoFr. Async/await to goroutines, Pydantic to Go structs, automatic OpenAPI to built-in Swagger UI."
nextjs:
  metadata:
    title: "FastAPI (Python) to GoFr Migration — Async Devs Adopting Go"
    description: "Migration guide for Python developers moving from FastAPI to GoFr. Async/await to goroutines, Pydantic to Go structs, automatic OpenAPI to built-in Swagger UI."
---

# Migrate from FastAPI (Python) to GoFr

{% answer %}
FastAPI users moving to GoFr trade `async def`/`await` for goroutines that GoFr manages on each request. Pydantic models become Go structs validated through `c.Bind(&struct)`. FastAPI's automatic OpenAPI generation maps to GoFr's built-in Swagger UI, and uvicorn is replaced by a single `gofr.New()` binary.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model

GoFr handlers are synchronous functions, but each one runs in its own goroutine — you don't decorate handlers with `async` because the runtime already gives you concurrency for free. Where FastAPI uses `async def` + `await` to avoid blocking the event loop, GoFr blocks the goroutine and lets the Go scheduler interleave others. The result is the same shape of code as a sync FastAPI route, with throughput closer to async.

Pydantic's runtime validation becomes compile-time struct typing plus tag-based validation on `Bind`. FastAPI's `Depends()` injection is replaced by passing dependencies through constructors or accessing datasources via `*gofr.Context`.

## Side-by-side: FastAPI handler ↔ GoFr handler

**FastAPI:**
```python
from fastapi import FastAPI
from pydantic import BaseModel

class CreateUser(BaseModel):
    name: str
    email: str

app = FastAPI()

@app.post("/users")
async def create_user(payload: CreateUser):
    user = await db.create(payload.dict())
    return user
```

**GoFr:**
```go
package main

import "gofr.dev/pkg/gofr"

type CreateUser struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    app := gofr.New()
    app.POST("/users", func(c *gofr.Context) (any, error) {
        var input CreateUser
        if err := c.Bind(&input); err != nil {
            return nil, err
        }
        return createUser(c, input)
    })
    app.Run()
}
```

## Concurrency: async/await → goroutines

A typical FastAPI deployment runs uvicorn workers, each one running its own event loop with cooperative async tasks. A GoFr service is a single binary; each request runs as a goroutine, and I/O calls block the goroutine without blocking the OS thread. There is no `await` keyword in user code — the framework, drivers, and HTTP/SQL clients propagate cancellation via `context.Context` (which `*gofr.Context` embeds).

If you previously offloaded CPU work via `run_in_threadpool`, in Go you simply call the function: the scheduler will move blocked goroutines off the worker threads.

## Validation and OpenAPI

| FastAPI | GoFr |
|---|---|
| Pydantic `BaseModel` | Go struct with JSON tags |
| `Field(..., min_length=3)` | Use a validator library (e.g. `go-playground/validator`) on the bound struct |
| Automatic OpenAPI at `/docs` | Drop your generated `openapi.json` into `static/` to serve via the built-in Swagger UI |
| `response_model` | Return typed structs; the response shape is the struct |

GoFr ships a Swagger UI that renders any `openapi.json` you place in the static directory — see the [openapi-documentation guide](/docs/advanced-guide/openapi-documentation).

## Dependency injection

FastAPI's `Depends()` is replaced by either:
- **Constructor passing** — build a struct holding your dependencies and use methods as handlers.
- **`*gofr.Context`** — datasources (SQL, Redis, Mongo, Pub/Sub) are accessed through the request context, so per-request injection of those is automatic.

## Datasources

FastAPI users typically reach for SQLAlchemy / Tortoise / Motor. In GoFr, SQL and Redis are auto-initialised from environment variables — set `DB_DIALECT`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` (or `REDIS_HOST`, `REDIS_PORT`) in `configs/.env` and `gofr.New()` wires the connection for you. Other clients are registered explicitly with a provider, e.g.:

```go
app.AddMongo(mongo.New(mongo.Config{/* ... */}))
```

Then access them inside the handler via `c.SQL`, `c.Redis`, `c.Mongo`. GoFr supports SQL (MySQL/Postgres/Oracle/SQLite/SQL Server), MongoDB, Redis, Cassandra, ScyllaDB, Couchbase, ArangoDB, Dgraph and SurrealDB. SQL/Mongo/Redis/Dgraph migrations are first-class — see the [datasources reference](/docs/datasources).

## Observability

FastAPI users typically wire `opentelemetry-instrumentation-fastapi` and `prometheus-fastapi-instrumentator` themselves. GoFr emits OpenTelemetry traces, Prometheus metrics at `/metrics`, and structured JSON logs (with trace IDs) by default. Health is exposed at `/.well-known/health`. Log levels are changeable at runtime via the [remote log-level endpoint](/docs/advanced-guide/remote-log-level-change).

## Gradual adoption

Run your FastAPI service alongside a new GoFr microservice and call it from GoFr using the built-in HTTP client with circuit breaker + retry + rate limiting:

```go
app.AddHTTPService("legacy-api", "http://legacy-fastapi:8000")
```

Move endpoints over progressively, repointing your gateway/load balancer until the old service can be retired.

{% faq %}

{% faq-item question="Can I run FastAPI and GoFr in the same cluster?" %}
Yes. They are independent processes. GoFr can call your FastAPI service through `app.AddHTTPService` with circuit breaker, retries, and rate limiting configured.
{% /faq-item %}

{% faq-item question="Is there an equivalent of Pydantic's strict validation?" %}
GoFr binds JSON, form, and multipart bodies into structs, but doesn't ship a validator. Most teams pair `c.Bind` with `go-playground/validator` for tag-based validation.
{% /faq-item %}

{% faq-item question="Where do background tasks (FastAPI's BackgroundTasks) go?" %}
Use goroutines for fire-and-forget work scoped to the request, GoFr's cron jobs for scheduled work, or Pub/Sub subscribers (Kafka, NATS, SQS, MQTT, Google Pub/Sub, Azure Event Hub) for queue-based jobs.
{% /faq-item %}

{% /faq %}
