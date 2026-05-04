---
description: "Migration guide for Python developers moving from Flask to GoFr. Pythonic patterns, handler translations, request binding, and gradual adoption strategy."
nextjs:
  metadata:
    title: "Flask (Python) to GoFr Migration — Python Devs Adopting Go"
    description: "Migration guide for Python developers moving from Flask to GoFr. Pythonic patterns, handler translations, request binding, and gradual adoption strategy."
---

# Migrate from Flask (Python) to GoFr

{% answer %}
Flask developers tend to like GoFr because both are minimal in the right places — small, opinionated cores with sensible defaults. Flask's `@app.route` decorator becomes `app.GET("/path", handler)`. Request access via `request.json` becomes `c.Bind(&struct)`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model translation

| Concept | Flask / Python | GoFr / Go |
|---|---|---|
| Concurrency | WSGI process / thread workers | Goroutines (one process, true concurrency) |
| Route | `@app.route('/users/<id>')` | `app.GET("/users/{id}", handler)` |
| Request body | `request.get_json()` | `c.Bind(&struct)` |
| Path param | `def view(id):` (function arg) | `c.PathParam("id")` |
| Query param | `request.args.get('q')` | `c.Param("q")` |
| Response | `return jsonify(data), 200` | `return data, nil` |
| Error response | `abort(404)` | `return nil, fmt.Errorf("not found")` |
| Logging | `logging` + structlog | Built-in GoFr structured logging |
| Tracing | OpenTelemetry Python instrumentation | Built into GoFr |
| Database | SQLAlchemy / psycopg / pymongo | Built-in clients |
| Background jobs | Celery / RQ | GoFr cron, Pub/Sub subscribers |

## Hello world

**Flask:**
```python
from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/hello')
def hello():
    return jsonify(message="Hello, world")

if __name__ == '__main__':
    app.run(port=8000)
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

## Concurrency: from gunicorn workers to goroutines

A typical Flask deployment runs gunicorn with N worker processes (or threads). Each request occupies one worker for its duration, including I/O wait. Scaling is by adding processes / replicas.

A GoFr service is a single binary. Each request is a goroutine. I/O is non-blocking. You typically need fewer instances at the same throughput.

## What you can drop

- `python-json-logger` / structlog config → built-in.
- `flask-prometheus-metrics` → built-in.
- `opentelemetry-instrumentation-flask` → built-in.
- Custom DB connection pooling on top of SQLAlchemy → handled by GoFr's SQL client.
- `flask-healthz` / hand-rolled `/healthz` → auto-exposed at `/.well-known/health`.

## Common gotchas

- **No global `request`.** The handler receives a `*Context` parameter; pass it where you need it. Goroutines + a goroutine-local `request` don't mix in Go.
- **`@app.errorhandler(Exception)` becomes explicit error returns.** Every error travels back as the second return value.
- **Database sessions aren't `flask-sqlalchemy`.** GoFr's SQL client gives you a connection pool with raw queries; pair with `sqlc` for type-safe queries if you want ORM-like ergonomics.
- **Decorators don't translate.** `@app.before_request` becomes middleware; `@app.errorhandler` becomes explicit error mapping in your handlers.
- **`abort(404)` becomes `return nil, errSomething` mapped to 404 via GoFr's error handling.** See [Error Handling](/docs/advanced-guide/gofr-errors).

## Estimated effort per service

A small Flask service (10-20 routes) typically takes 2–4 engineering days for a Python developer new to Go. Most of the time is spent on Go idioms.

## Recommended adoption

1. Pick a small Flask service (an internal webhook, a CRUD API) and rebuild it in GoFr.
2. Run side-by-side, validate observability output.
3. Iterate — port more services as your team gains comfort.

{% faq %}

{% faq-item question="Are there async equivalents of Quart / FastAPI in Go?" %}
Go's concurrency primitives mean you don't need an async/await separation — every handler runs in its own goroutine, and I/O is non-blocking by default. GoFr fits this model.
{% /faq-item %}

{% faq-item question="Does GoFr have an ORM like SQLAlchemy?" %}
No. GoFr's SQL client provides connection pooling, observability, and parameter binding, not an ORM. Many Go teams use `sqlc` for type-safe queries; some use `gorm`. Both work fine inside GoFr handlers.
{% /faq-item %}

{% faq-item question="Can I run Celery-style background jobs in GoFr?" %}
Yes — GoFr has built-in cron scheduling and Pub/Sub subscribers (Kafka, NATS, Google Pub/Sub, MQTT, SQS, Azure Event Hub). Combined, these cover most Celery use cases.
{% /faq-item %}

{% /faq %}
