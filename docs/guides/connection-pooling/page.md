---
description: "Size SQL, Redis, and HTTP connection pools in GoFr so replica count times per-pod pool stays under your database's max_connections, with concrete env-var math."
nextjs:
  metadata:
    title: "GoFr Connection Pooling: SQL, Redis, HTTP Sizing"
    description: "Size SQL, Redis, and HTTP connection pools in GoFr so replica count times per-pod pool stays under your database's max_connections, with concrete env-var math."
---

# Connection Pooling

{% answer %}
GoFr exposes per-datasource pool knobs as environment variables — `DB_MAX_OPEN_CONNECTION` and `DB_MAX_IDLE_CONNECTION` for SQL, plus a programmatic `service.ConnectionPoolConfig` for outbound HTTP calls. On Kubernetes the rule that matters is `replicas × per_pod_pool ≤ database max_connections`. Size pools using the formula `target_connections = peak_qps × p99_latency_seconds` so each pod has just enough capacity to absorb its share of traffic.
{% /answer %}

## When to use

Default pool sizes are tuned for development. Under production load on Kubernetes you will hit one of two failure modes: pool exhaustion (request hangs waiting for a connection) or database overload (`FATAL: too many connections` from PostgreSQL, `Too many connections` from MySQL). Both are operational, not code, problems — they're solved by sizing the pool against measured traffic and the database's hard ceiling.

## SQL connection pool

GoFr reads two env vars in `pkg/gofr/datasource/sql` and applies them with `database/sql`:

| Env var | Default | Behavior |
|---|---|---|
| `DB_MAX_OPEN_CONNECTION` | `0` (unlimited) | Maps to `SetMaxOpenConns` |
| `DB_MAX_IDLE_CONNECTION` | `2` | Maps to `SetMaxIdleConns` |

```dotenv
DB_DIALECT=postgres
DB_HOST=postgres-primary
DB_PORT=5432
DB_NAME=orders
DB_USER=orders_app
DB_PASSWORD=...

DB_MAX_OPEN_CONNECTION=20
DB_MAX_IDLE_CONNECTION=5
```

GoFr does not currently expose a knob for `SetConnMaxLifetime` / `SetConnMaxIdleTime`; rely on the database's own idle-timeout to recycle stale connections.

For replica reads, the framework also recognizes `DB_REPLICA_*` variables (hosts, ports, users, passwords, plus pool sizing). See [GoFr Configuration Options](/docs/references/configs) for the full list.

### Sizing math

Start from measured load:

```text
target_connections_per_pod = ceil(peak_qps_per_pod × p99_query_latency_seconds)
```

A pod serving 200 QPS with 50 ms P99 query latency needs `200 × 0.05 = 10` connections to keep the queue empty. Add 50% headroom for spikes → `DB_MAX_OPEN_CONNECTION=15`.

Then verify against the database ceiling:

```text
total = replicas × DB_MAX_OPEN_CONNECTION + other_consumers
total ≤ database.max_connections × 0.8   # leave 20% for admins, migrations
```

For PostgreSQL with `max_connections=200`, 10 replicas, `DB_MAX_OPEN_CONNECTION=15` → 150 connections, fits inside the 80% budget. If your replica count outgrows this, put PgBouncer in front and have GoFr connect through it.

### Symptoms of exhaustion

- Latency suddenly spikes for *some* requests while others stay normal — handlers blocked on `db.Conn`.
- Logs show repeated `dial tcp ...: i/o timeout` after a deploy that increased replica count.
- Postgres `pg_stat_activity` count is at or near `max_connections`.

GoFr's default datasource metrics include connection-pool gauges (active/idle); alert when `idle == 0` for more than a minute, or when active is at the configured max for sustained periods.

## Redis, Mongo, and other datasources

Each datasource driver in GoFr has its own defaults — see the [configuration reference](/docs/references/configs) for the env vars that exist for Redis, MongoDB, Cassandra, ClickHouse, and others. The same `replicas × per_pod_pool ≤ server_max` rule applies: a Redis cluster with `maxclients=10000` and 50 pods leaves 200 connections per pod.

## Outbound HTTP service pool

Service-to-service calls share connections through Go's `http.Transport`. GoFr exposes this as `service.ConnectionPoolConfig` on `AddHTTPService`:

```go
import (
    "time"

    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/service"
)

func main() {
    app := gofr.New()

    app.AddHTTPService("payments", "https://payments.internal",
        &service.ConnectionPoolConfig{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 20,
            IdleConnTimeout:     90 * time.Second,
        },
        &service.CircuitBreakerConfig{Threshold: 4, Interval: 5 * time.Second},
        &service.RetryConfig{MaxRetries: 3},
    )

    app.Run()
}
```

One pitfall to know:

- **Go's default `MaxIdleConnsPerHost` is 2.** That's almost always too low for a microservice talking to one downstream — bump it to 10–20 for typical traffic, higher for chatty services.

You can pass `ConnectionPoolConfig` in any position among the `AddHTTPService` options. The framework's `extractHTTPService` helper recursively unwraps the circuit-breaker, retry, and auth wrappers when applying pool config, so option order does not matter.

## Verification

```bash
# SQL — Postgres
psql -c "SELECT count(*) FROM pg_stat_activity WHERE datname='orders';"

# In-cluster — pool gauges via /metrics
curl http://orders-api.prod:2121/metrics | grep -E '(sql|redis)_open_connections'
```

After a deploy, watch the connection count climb to roughly `replicas × DB_MAX_OPEN_CONNECTION` under steady load. If it exceeds that, something is bypassing the framework's datasource (e.g., a hand-rolled `sql.Open`).

{% faq %}
{% faq-item question="What are the exact env vars for GoFr's SQL pool?" %}
`DB_MAX_OPEN_CONNECTION` (default 0 = unlimited) and `DB_MAX_IDLE_CONNECTION` (default 2). They map to `SetMaxOpenConns` and `SetMaxIdleConns` respectively.
{% /faq-item %}
{% faq-item question="Why is my HTTP client only using 2 connections per host?" %}
That's Go's `DefaultMaxIdleConnsPerHost`. Pass a `service.ConnectionPoolConfig` with `MaxIdleConnsPerHost: 20` (in any position among the `AddHTTPService` options — the framework unwraps circuit-breaker/retry/auth wrappers regardless of order).
{% /faq-item %}
{% faq-item question="Should I cap DB_MAX_OPEN_CONNECTION on every service?" %}
Yes. The default of `0` (unlimited) is fine for local development but lets a single misbehaving pod exhaust the database. Always set an explicit limit in production.
{% /faq-item %}
{% /faq %}
