# GoFr — guide for AI coding assistants

For Claude Code, Cursor, Aider, Codex, etc. Hand
[https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your assistant
when working on a GoFr project. It points at the specific docs you'll
actually need rather than inlining everything.

GoFr is an opinionated Go framework for microservices. Apache 2.0.
Requires Go 1.24+. Repo: <https://github.com/gofr-dev/gofr>.

## Core rules an assistant must know

1. **One handler signature** for HTTP / gRPC / GraphQL / WebSocket / cron / CLI:
   `func(c *gofr.Context) (any, error)`. Return value → `{"data": ...}`. Return error → error response.
2. **Path params use `{name}`**, not `:name`. `app.GET("/users/{id}", h)`.
3. **`c.PathParam("id")`** for path; **`c.Param("q")`** for query; **`c.Bind(&v)`** for body.
4. **Don't manually wire OpenTelemetry, Prometheus, or structured logging.** `gofr.New()` does it.
5. **Pass `*gofr.Context`** (not `context.Background()`) to every downstream call so trace propagation and cancellation work.
6. **Configuration comes from `configs/.env`** via `c.Config.Get(key)`. No hardcoded ports / hosts / secrets.
7. **No ORM bundled.** Use plain SQL via `c.SQL`, or pair with `sqlc`/`gorm` yourself.
8. **Middleware is standard `net/http`**: `func(http.Handler) http.Handler`, registered with `app.UseMiddleware(...)`.
9. **Health is auto-exposed** at `/.well-known/health` and `/.well-known/alive`. Don't hand-roll `/healthz`.
10. **Don't `panic()` for control flow.** Return errors.

## Minimal app

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

## Migrations (schema + seed data, versioned)

GoFr has a built-in migration runner — use it instead of `golang-migrate` / `goose`. Works for SQL, Redis, Mongo, Cassandra, ClickHouse, DGraph, SurrealDB.

**Before writing a migration, fetch the full reference:** <https://gofr.dev/docs/advanced-guide/handling-data-migrations>. Runnable shape: <https://github.com/gofr-dev/gofr/tree/main/examples/using-migrations>.

Five rules to keep in your head so you don't have to re-fetch on every change:

1. Files live in `migrations/<unix-ts>_<name>.go`. Versions are `int64`, monotonic. Use `date +%s` for new ones.
2. `migrations/all.go` exports `All() map[int64]migration.Migrate`. Wire with `a.Migrate(migrations.All())` once in `main.go`.
3. Each migration is `migration.Migrate{ UP: func(d migration.Datasource) error { ... } }`. Inside `UP`, act on `d.SQL`, `d.Redis`, `d.MongoDB`, `d.Cassandra`, `d.Clickhouse`, `d.DGraph`, `d.SurrealDB`, `d.Logger`.
4. Each migration runs **exactly once** per environment — don't add idempotent guards. There is **no down migration**; rollback = new forward migration.
5. One purpose per file. Don't combine "create table + backfill + drop column" in one `UP`.

## Where to look (fetch only what you need)

Each link is a self-contained doc page. Fetch lazily.

- Quick start: <https://gofr.dev/docs/quick-start/introduction>
- Configuration: <https://gofr.dev/docs/quick-start/configuration>
- Routing + REST handlers: <https://gofr.dev/docs/quick-start/add-rest-handlers>
- Context reference (the central abstraction): <https://gofr.dev/docs/references/context>
- Configs reference (every env var): <https://gofr.dev/docs/references/configs>
- Testing patterns + mocks: <https://gofr.dev/docs/references/testing>
- Observability (auto-wired): <https://gofr.dev/docs/quick-start/observability>
- Custom OTel spans: <https://gofr.dev/docs/advanced-guide/custom-spans-in-tracing>
- Custom Prometheus metrics: <https://gofr.dev/docs/advanced-guide/publishing-custom-metrics>
- Datasources index (MySQL, Postgres, Mongo, Redis, Cassandra, ClickHouse, etc.): <https://gofr.dev/docs/datasources/getting-started>
- Pub/Sub (Kafka, NATS, GCP, MQTT, SQS, Azure): <https://gofr.dev/docs/advanced-guide/using-publisher-subscriber>
- gRPC: <https://gofr.dev/docs/advanced-guide/grpc>
- GraphQL: <https://gofr.dev/docs/advanced-guide/graphql>
- WebSockets: <https://gofr.dev/docs/advanced-guide/websocket>
- Cron: <https://gofr.dev/docs/advanced-guide/using-cron>
- Service-to-service HTTP w/ circuit breaker: <https://gofr.dev/docs/advanced-guide/http-communication>
- Auth (Basic / API key / OAuth-JWT): <https://gofr.dev/docs/advanced-guide/authentication>
- RBAC (config-driven): <https://gofr.dev/docs/advanced-guide/rbac>
- Migrations (SQL + NoSQL, versioned): <https://gofr.dev/docs/advanced-guide/handling-data-migrations>
- File storage (local / S3 / GCS / Azure / FTP / SFTP): <https://gofr.dev/docs/advanced-guide/handling-file>
- Startup hooks: <https://gofr.dev/docs/advanced-guide/startup-hooks>
- Errors: <https://gofr.dev/docs/advanced-guide/gofr-errors>
- Custom DB drivers: <https://gofr.dev/docs/advanced-guide/injecting-databases-drivers>

## Migrating to GoFr

Each migration page has the full mapping table for that source framework — fetch only the one that matches:

- From Gin: <https://gofr.dev/migrate/from-gin>
- From Fiber: <https://gofr.dev/migrate/from-fiber>
- From Echo: handler shape similar to Gin; see <https://gofr.dev/comparison/gofr-vs-echo>
- From Chi: see <https://gofr.dev/comparison/gofr-vs-chi>
- From Express (Node.js): <https://gofr.dev/migrate/from-express>
- From Flask (Python): <https://gofr.dev/migrate/from-flask>
- From Spring Boot (Java): <https://gofr.dev/migrate/from-spring-boot>

## Other entry points

- Examples (runnable projects): <https://github.com/gofr-dev/gofr/tree/main/examples>
- Full docs index: <https://gofr.dev/docs>
- Concatenated full docs (one plaintext file): <https://gofr.dev/llms-full.txt>
- Curated link index: <https://gofr.dev/llms.txt>
- Changelog: <https://gofr.dev/changelog>
