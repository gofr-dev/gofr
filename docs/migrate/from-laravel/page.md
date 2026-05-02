---
description: "Migration guide for PHP developers moving from Laravel to GoFr. Controllers to handlers, Eloquent to SQL drivers, Artisan to GoFr CLI, queues to Pub/Sub."
nextjs:
  metadata:
    title: "Laravel (PHP) to GoFr Migration — PHP Devs Adopting Go"
    description: "Migration guide for PHP developers moving from Laravel to GoFr. Controllers to handlers, Eloquent to SQL drivers, Artisan to GoFr CLI, queues to Pub/Sub."
---

# Migrate from Laravel (PHP) to GoFr

{% answer %}
Laravel devs moving to GoFr trade Eloquent and the Service Container for explicit SQL and constructor passing — and gain a static binary, built-in observability, and goroutine concurrency. Routes, controllers, middleware, validation, queues, and CLI commands all have direct GoFr analogues; the `.env` file even keeps its name.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model

| Laravel | GoFr |
|---|---|
| `Route::get('/users/{id}', ...)` | `app.GET("/users/{id}", handler)` |
| Controller method | Handler function (or method on a struct) |
| `$request->input('name')` | `var dto CreateUser; c.Bind(&dto)` |
| Form Request validation | Struct tags + a validator library |
| Middleware (`Kernel.php`) | `app.UseMiddleware(...)` |
| Service Container / `app()` | Constructor passing of dependencies |
| Eloquent ORM | SQL drivers (`c.SQL`); pair with `sqlc` or `gorm` for ergonomics |
| Migrations (`php artisan migrate`) | GoFr SQL migrations |
| Artisan commands | GoFr CLI / sub-commands |
| Queues (database/Redis/SQS) | GoFr Pub/Sub (Kafka, NATS, SQS, MQTT, Google Pub/Sub, Azure Event Hub) |
| Scheduler (`Kernel::schedule`) | `app.AddCronJob(...)` |
| `.env` | `configs/.env` |
| Telescope / Horizon dashboards | Prometheus metrics + traces in your existing stack |
| Sanctum / Passport | Built-in Basic / APIKey / OAuth-JWT + RBAC |

## Side-by-side: controller ↔ handler

**Laravel:**
```php
class UserController extends Controller {
    public function store(Request $request) {
        $data = $request->validate([
            'name'  => 'required|min:3',
            'email' => 'required|email',
        ]);
        return User::create($data);
    }
}

Route::post('/users', [UserController::class, 'store']);
```

**GoFr:**
```go
type CreateUser struct {
    Name  string `json:"name"  validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
}

app.POST("/users", func(c *gofr.Context) (any, error) {
    var dto CreateUser
    if err := c.Bind(&dto); err != nil {
        return nil, err
    }
    return createUser(c, dto)
})
```

## Auto-CRUD via AddRESTHandlers

If your Laravel resource is "controller + Eloquent model + standard CRUD", you can collapse it in GoFr to:

```go
app.AddRESTHandlers(&User{})
```

— which exposes `GET / POST / GET/{id} / PUT/{id} / DELETE/{id}` against your struct/table. See the [REST scaffolding guide](/docs/advanced-guide/scaffolding-rest-server).

## Validation

Laravel's Form Requests collapse parsing + validating into one. In GoFr it's two steps:

- `c.Bind(&dto)` — parse JSON / form / multipart.
- A validator library (e.g. `go-playground/validator`) — apply struct-tag rules.

The trade-off is more explicit code, less magic.

## Middleware

Laravel's `Kernel.php` middleware groups translate to:

```go
app.UseMiddleware(authMiddleware)
app.UseMiddleware(rateLimiter)
```

Authentication options ship in GoFr (Basic, API Key, OAuth-JWT — see [authentication](/docs/advanced-guide/http-authentication)) and you can layer RBAC on top.

## Eloquent → SQL drivers

This is the biggest shift. GoFr does not include an ORM. Replace Eloquent calls with explicit SQL via `c.SQL.Query` / `Exec`, and pair with `sqlc` for generated type-safe queries or `gorm` for ORM-like ergonomics.

Migrations move from `php artisan make:migration` to versioned [GoFr SQL migrations](/docs/advanced-guide/handling-data-migrations) — files applied in order at boot.

## Queues → Pub/Sub

Laravel queues backed by Redis / database / SQS map to GoFr's Pub/Sub:

```go
app.Subscribe("user.created", func(c *gofr.Context) error {
    var msg UserCreated
    if err := c.Bind(&msg); err != nil {
        return err
    }
    return process(c, msg)
})
```

Supported backends: Kafka, NATS, SQS, MQTT, Google Pub/Sub, Azure Event Hub. Publish via `app.GetPublisher().Publish(c, topic, payload)`.

## Artisan → GoFr CLI

Laravel's Artisan commands (cleanup jobs, data backfills, one-off scripts) map onto GoFr's CLI / sub-command support — register sub-commands on the same app and invoke as `./mybinary <subcommand>`. See the [CLI command guide](/docs/advanced-guide/using-gofr-cli).

For periodic work, use `app.AddCronJob(spec, fn)` instead of `php artisan schedule:run`.

## Datasources

```go
app.AddSQL(/* read from .env */)
app.AddRedis(...)
app.AddMongo(...)
```

SQL (MySQL/Postgres/Oracle/SQLite/SQL Server), Redis, Mongo, Cassandra, ScyllaDB, Couchbase, ArangoDB, Dgraph, SurrealDB are supported. File storage drivers cover Local, S3, GCS, Azure Blob, FTP, SFTP — useful when porting Laravel filesystem disks.

## Configuration

`.env` — same name, slightly different conventions. GoFr reads `configs/.env`, with environment-specific files (`configs/.env.production`) layered on via `APP_ENV`. Read in code with `app.Config.Get(key)`.

## Observability

Telescope and Horizon are application-bundled dashboards; GoFr instead exports OpenTelemetry traces and Prometheus metrics at `/metrics` to whatever stack you already run (Grafana, Datadog, Honeycomb, etc.). Structured JSON logs include trace IDs. Health is exposed at `/.well-known/health`. Log levels are changeable at runtime.

## Gradual adoption

Pick a bounded context (notifications, search, file processing) and rebuild it as a GoFr service. From Laravel call it over HTTP; from GoFr call back into Laravel with `app.AddHTTPService("laravel-api", baseURL)` — circuit breaker, retries, and rate limiting included.

{% faq %}

{% faq-item question="Can I run Laravel and GoFr in the same cluster?" %}
Yes. They are independent services. Bridge via HTTP (with GoFr's resilient HTTP client) or via Pub/Sub topics shared with Laravel queue workers (e.g. SQS).
{% /faq-item %}

{% faq-item question="Is there a Blade equivalent?" %}
GoFr is API-first and doesn't ship a templating engine. For server-rendered HTML, Go's `html/template` works inside handlers, but most teams pair GoFr with a separate frontend.
{% /faq-item %}

{% faq-item question="What about Laravel Echo / WebSockets?" %}
GoFr supports WebSocket and Server-Sent Events directly. Laravel Echo's broadcast pattern translates to a Pub/Sub backend fanning out to GoFr WebSocket connections.
{% /faq-item %}

{% /faq %}
