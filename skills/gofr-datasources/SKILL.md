---
name: gofr-datasources
description: Connect a GoFr service to a datasource — Postgres, MySQL, Redis, MongoDB, Cassandra, ClickHouse, Elasticsearch, Kafka and others — with tracing, metrics, and health checks wired in automatically. Use when adding a database, cache, or message broker to a Go service.
license: Apache-2.0
---

# Connecting datasources in GoFr

GoFr configures datasource clients from environment variables and attaches
tracing, metrics, and a health check to each one. You do not construct the
client, and you do not write a health endpoint for it.

## SQL (Postgres, MySQL, CockroachDB)

Set the connection in `configs/.env`:

```
DB_DIALECT=postgres
DB_HOST=localhost
DB_PORT=5432
DB_USER=app
DB_PASSWORD=secret
DB_NAME=app
```

Then use `c.SQL` — it is a `*sql.DB` wrapper, so the standard library API applies:

```go
func listUsers(c *gofr.Context) (any, error) {
    rows, err := c.SQL.QueryContext(c, "SELECT id, name FROM users WHERE deleted_at IS NULL")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var u User
        if err := rows.Scan(&u.ID, &u.Name); err != nil {
            return nil, err
        }
        users = append(users, u)
    }

    return users, rows.Err()
}
```

Always: parameterised queries only, `defer rows.Close()`, and check `rows.Err()`
after the loop.

## Redis

```
REDIS_HOST=localhost
REDIS_PORT=6379
```

```go
val, err := c.Redis.Get(c.Context, "session:"+id).Result()
```

## Everything else

MongoDB, Cassandra, ClickHouse, Elasticsearch, ScyllaDB, SurrealDB, DGraph,
Couchbase, InfluxDB, Oracle, Solr, OpenTSDB and the KV stores each live in their
own module under `gofr.dev/pkg/gofr/datasource/...`. Add the module, then
register the client with `app.AddMongo(...)`, `app.AddCassandra(...)` and so on.

Pub/Sub — Kafka, Google Pub/Sub, MQTT, NATS, EventHub, SQS — is configured the
same way and consumed with `app.Subscribe(topic, handler)`.

## Rules

- **Pass `c` as the context** to every call so the query joins the request trace.
- Connection details come from config, never from code.
- Do not add your own health check — `/.well-known/health` already reports every
  registered datasource.
- Migrations belong in `migrations/`, run by `app.Migrate(...)`.

## Read next

- <https://gofr.dev/docs/datasources/llms.txt> — every supported datasource
- <https://gofr.dev/docs/quick-start/connecting-mysql.md>
- <https://gofr.dev/docs/quick-start/connecting-redis.md>
- <https://gofr.dev/docs/advanced-guide/handling-data-migrations.md>
