---
description: "Why use GoFr instead of Gin, Fiber, or net/http? Built-in observability, 15+ datasources, gRPC, GraphQL, WebSockets, and pub/sub for Go microservices."
nextjs:
  metadata:
    title: "Why GoFr — An Opinionated Go Framework for Microservices"
    description: "Why use GoFr instead of Gin, Fiber, or net/http? Built-in observability, 15+ datasources, gRPC, GraphQL, WebSockets, and pub/sub for Go microservices."
---

# Why GoFr?

{% answer %}
GoFr is an opinionated Go framework focused on microservices. Minimal routers like Gin, Fiber, and Chi keep their surface area small by design and let you assemble the rest of your stack the way you prefer. GoFr makes a different trade-off: it bundles a common production layer — OpenTelemetry tracing, Prometheus metrics, structured logging, datasource clients, migrations, Pub/Sub, gRPC, GraphQL, WebSockets, health checks, circuit breakers, graceful shutdown — with sensible defaults. Both approaches are valid; this page describes the situations where GoFr's trade-off tends to fit.
{% /answer %}

## See the difference in 20 lines

A REST handler that connects to MySQL, emits OpenTelemetry traces, exports Prometheus metrics, and writes structured logs:

**With `net/http` + your stack of choice:**

```go
// Init: tracer provider, exporter, propagator, sampler.
// Init: prometheus registry, HTTP histogram, label cardinality plan.
// Init: structured logger, request-id middleware, log-context plumbing.
// Init: sql.DB with connection pool, otelsql instrumentation.
// Per-handler: extract span from context, propagate to db query,
//              record metrics with labels, structured log with trace id.
// You write all of this. ~150 lines of glue before you write business logic.
```

**With GoFr:**

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()
    app.GET("/users/{id}", func(c *gofr.Context) (any, error) {
        var name string
        err := c.SQL.QueryRowContext(c, "SELECT name FROM users WHERE id=?", c.PathParam("id")).Scan(&name)
        return map[string]string{"name": name}, err
    })
    app.Run()
}
```

Tracing, metrics, structured logging with trace IDs, connection pooling, and DB span correlation are emitted automatically. Configuration is `.env` based.

## The trade-off behind opinionated frameworks

Microservices often share the same supporting needs: structured logging, request tracing, metrics, datasource clients, message brokers, health checks, circuit breakers, retries, environment-based config, graceful shutdown.

With a minimal router, you compose these yourself by bringing libraries like `zap`, `otel-go`, `prometheus/client_golang`, `sqlx`, `sarama`, or `gobreaker`. That's a strength when you want full control over each layer; it's a cost when teams keep wiring similar combinations across many services.

GoFr's wager is that this wiring is worth standardizing as a shared default. Some teams will appreciate the time saved; others will prefer the precision of composing their own stack.

## What's actually in GoFr

GoFr's positioning, [from the framework's README](https://github.com/gofr-dev/gofr), is:

> **An Opinionated Microservice Development Framework — designed to simplify microservice development, with a key focus on Kubernetes deployment and out-of-the-box observability.**

Concretely:

- **HTTP, gRPC, GraphQL, WebSockets, CLI** — one handler signature `func(*Context) (any, error)` across all of them.
- **Auto CRUD handlers** — `app.AddRESTHandlers(&Entity{})` generates Create / Get / GetAll / Update / Delete endpoints from a struct.
- **Observability built in** — OpenTelemetry traces (OTLP/Jaeger), Prometheus metrics, structured contextual logging. Configurable sampling. Remote log-level changes without restart.
- **15+ datasources** — MySQL, PostgreSQL, Oracle, SQLite, MongoDB, Redis, Cassandra, ScyllaDB, ClickHouse, CockroachDB, Couchbase, DGraph, SurrealDB, ArangoDB, Elasticsearch, Solr, InfluxDB, OpenTSDB. KV-store backends include Badger, DynamoDB, and NATS. All auto-instrumented.
- **Pub/Sub** — Kafka, NATS JetStream, Google Pub/Sub, AWS SQS, MQTT, Azure Event Hub.
- **File storage** — local filesystem, Amazon S3, Google Cloud Storage, Azure Blob, FTP, SFTP — one interface.
- **Service-to-service HTTP client** — circuit breaker, retry, rate limit, connection pool, Basic / API-key / OAuth auth — all configurable per service.
- **Migrations** — versioned for SQL, MongoDB, Redis, DGraph, and more.
- **Auth & RBAC** — Basic, API key, OAuth (JWKS-validated JWT), config-driven role/permission mappings.
- **Built-in Swagger UI** — drop your `openapi.json` in `static/` and `/.well-known/swagger` renders it.
- **Cron jobs** — 5- and 6-part expressions with auto-instrumented OpenTelemetry spans per job.
- **Graceful shutdown + startup hooks** — `OnStart` for warmup; clean teardown of connections.

Each of these is something you can also assemble yourself with libraries you trust. GoFr packages a common combination so you don't have to re-make those choices on every service.

## Who tends to like GoFr

- **Teams building microservices on Kubernetes** who want tracing, metrics, and structured logs available from the first commit.
- **Engineers coming from Spring Boot, Express, or NestJS** who are used to a "batteries-included" framework and prefer that style.
- **Gin / Fiber / Chi users** who find themselves repeatedly writing similar observability, datasource, and resilience plumbing across services and would rather standardize it.

## Where to go next

- [Quick Start: Build your first GoFr REST API](/docs/quick-start/introduction) — running in under 5 minutes.
- [GoFr vs Gin / Fiber / Echo / Chi](/comparison) — head-to-head on features.
- [Migrate from Gin / Fiber / Express / Flask / Spring Boot](/migrate) — concrete code translations.
- [Documentation](/docs) — full reference.

{% faq %}

{% faq-item question="Is GoFr free and open source?" %}
Yes. GoFr is licensed under Apache 2.0 and developed in the open at [github.com/gofr-dev/gofr](https://github.com/gofr-dev/gofr). There is no paid tier; the framework is fully usable without commercial licensing.
{% /faq-item %}

{% faq-item question="Does GoFr replace OpenTelemetry, Prometheus, or my logger?" %}
No. GoFr uses OpenTelemetry SDKs, Prometheus client libraries, and structured logging primitives directly. You still export to your existing OTel collector, Prometheus, or log aggregator. GoFr removes the wiring, not the standards.
{% /faq-item %}

{% faq-item question="Is GoFr production-ready?" %}
GoFr has been used in production microservices at companies like American Express, IBM, Walmart, and Mydbops. See the [showcase page](/showcase) for more.
{% /faq-item %}

{% faq-item question="Can I use GoFr alongside an existing Gin or Fiber service?" %}
Yes. GoFr is a separate Go module; you can run a new GoFr service in the same fleet as existing Gin / Fiber / Echo services. Most teams adopt GoFr for new services first, then migrate older ones gradually.
{% /faq-item %}

{% faq-item question="Does GoFr lock me into specific datasources?" %}
No. The datasource interfaces are open — see [Injecting Custom Database Drivers](/docs/advanced-guide/injecting-databases-drivers). Built-in support exists for the most common backends so you don't write that code yourself.
{% /faq-item %}

{% /faq %}
