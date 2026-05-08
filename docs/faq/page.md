---
description: "GoFr FAQ — pricing, production readiness, supported protocols (gRPC, GraphQL, WebSockets), datasources, deployment, observability, and testing."
nextjs:
  metadata:
    title: "GoFr FAQ — Frequently Asked Questions"
    description: "GoFr FAQ — pricing, production readiness, supported protocols (gRPC, GraphQL, WebSockets), datasources, deployment, observability, and testing."
---

# Frequently Asked Questions

{% answer %}
GoFr is a free, Apache 2.0–licensed, opinionated Go framework for production microservices. It includes built-in HTTP, gRPC, GraphQL, WebSockets, Pub/Sub, observability (OpenTelemetry traces, Prometheus metrics, structured logs), 15+ datasource clients, migrations, cron, RBAC, and a service-to-service HTTP client with circuit breakers.
{% /answer %}

## Pricing & licensing

{% faq %}

{% faq-item question="Is GoFr free to use?" %}
Yes. GoFr is open source and licensed under [Apache 2.0](https://github.com/gofr-dev/gofr/blob/main/LICENSE). There is no paid tier, no commercial license, and no usage limits.
{% /faq-item %}

{% faq-item question="Is GoFr open source?" %}
Yes. The full source is at [github.com/gofr-dev/gofr](https://github.com/gofr-dev/gofr) under the Apache 2.0 license.
{% /faq-item %}

{% faq-item question="Who maintains GoFr?" %}
GoFr is developed in the open by the GoFr team and a community of contributors. See the [team page](/team) for the current maintainers, and [github.com/gofr-dev/gofr](https://github.com/gofr-dev/gofr) for ways to get involved.
{% /faq-item %}

{% /faq %}

## Features and protocols

{% faq %}

{% faq-item question="Does GoFr support gRPC?" %}
Yes. Built-in gRPC server with unary and stream interceptors, custom server options, panic recovery, and integrated observability. See [Writing gRPC Servers and Clients](/docs/advanced-guide/grpc).
{% /faq-item %}

{% faq-item question="Does GoFr support GraphQL?" %}
Yes. GoFr supports schema-first GraphQL with queries, mutations, and an interactive playground. See [GraphQL in Go with GoFr](/docs/advanced-guide/graphql).
{% /faq-item %}

{% faq-item question="Does GoFr support WebSockets?" %}
Yes — both server and client. Auto-reconnect, custom upgrader, and integrated observability. See [WebSockets in Go with GoFr](/docs/advanced-guide/websocket).
{% /faq-item %}

{% faq-item question="Does GoFr support Pub/Sub?" %}
Yes. Built-in support for Apache Kafka, NATS JetStream, Google Pub/Sub, MQTT, AWS SQS, and Azure Event Hub through one unified `Subscribe` / `Publish` API.
{% /faq-item %}

{% faq-item question="Does GoFr include cron jobs?" %}
Yes. Schedule recurring tasks with 5- or 6-part cron expressions. Each job execution gets an automatic OpenTelemetry span and metrics.
{% /faq-item %}

{% /faq %}

## Datasources

{% faq %}

{% faq-item question="Which databases does GoFr support?" %}
SQL: MySQL, PostgreSQL, Oracle, SQLite, SQL Server. NoSQL: MongoDB, Redis, Cassandra, ScyllaDB, Couchbase, DGraph, SurrealDB, ArangoDB. Search/Analytics: Elasticsearch, Solr, ClickHouse, OpenTSDB, InfluxDB. All ship with built-in observability.
{% /faq-item %}

{% faq-item question="Does GoFr have an ORM?" %}
GoFr's SQL client provides connection pooling, observability, and parameter binding, not an ORM. Many GoFr users pair it with [`sqlc`](https://sqlc.dev/) for type-safe queries; some use `gorm`. Both work fine inside GoFr handlers.
{% /faq-item %}

{% faq-item question="Can I plug in a custom database driver?" %}
Yes. Implement the GoFr datasource interface to inject your own backend with full observability.
{% /faq-item %}

{% /faq %}

## Observability

{% faq %}

{% faq-item question="Does GoFr support OpenTelemetry?" %}
Yes. OpenTelemetry tracing is built in with OTLP and Jaeger exporters, configurable sampling, trace-context propagation, and automatic span correlation across HTTP, gRPC, datasource calls, cron jobs, and Pub/Sub.
{% /faq-item %}

{% faq-item question="Does GoFr support Prometheus metrics?" %}
Yes. Built-in Prometheus metrics for HTTP requests, gRPC, cron jobs, GraphQL operations, and your own custom counters/histograms/gauges. The `/metrics` endpoint is auto-exposed.
{% /faq-item %}

{% faq-item question="Can I change log levels in production without restart?" %}
Yes. Point `REMOTE_LOG_URL` at an HTTP endpoint that returns the desired log level; GoFr's logger polls that URL and adjusts the in-process level on the fly (poll interval via `REMOTE_LOG_FETCH_INTERVAL`). The admin endpoint is one *you* operate — GoFr does not serve it on the service itself. See [Remote Log Level Change](/docs/advanced-guide/remote-log-level-change).
{% /faq-item %}

{% /faq %}

## Comparing to other frameworks

{% faq %}

{% faq-item question="How is GoFr different from net/http?" %}
`net/http` is the standard library — it gives you HTTP and nothing else. GoFr is built on `net/http` and adds opinionated production layers: routing helpers, observability, datasource clients, gRPC, GraphQL, WebSockets, Pub/Sub, migrations, RBAC, and a resilient service-to-service HTTP client.
{% /faq-item %}

{% faq-item question="How does GoFr compare to Gin, Fiber, Echo, and Chi?" %}
Gin, Fiber, Echo, and Chi are excellent minimal HTTP routers. GoFr has a wider scope — alongside HTTP it also includes observability, datasources, gRPC, GraphQL, Pub/Sub, migrations, and resilience patterns. See [GoFr vs Gin / Fiber / Echo / Chi](/comparison) for a side-by-side; both approaches have their place.
{% /faq-item %}

{% faq-item question="Can I migrate from Gin / Fiber / Express / Flask / Spring Boot to GoFr?" %}
Yes. Migration guides with concrete code translations are at [/migrate](/migrate).
{% /faq-item %}

{% /faq %}

## Deployment and operations

{% faq %}

{% faq-item question="Does GoFr work on Kubernetes?" %}
Yes. GoFr is designed for Kubernetes deployment. Health endpoints (`/.well-known/health`, `/.well-known/alive`) are auto-exposed for liveness and readiness probes. Logs go to stdout in JSON; metrics expose at `/metrics` for Prometheus scraping.
{% /faq-item %}

{% faq-item question="Does GoFr support graceful shutdown?" %}
Yes. `app.Shutdown(ctx)` closes the HTTP server, gRPC server, datasource connections, and loggers cleanly when the process receives a termination signal.
{% /faq-item %}

{% faq-item question="Can I run setup logic before the server starts?" %}
Yes. Register a function with `app.OnStart` to seed databases, warm caches, or perform initialization synchronously before traffic begins.
{% /faq-item %}

{% /faq %}

## Testing

{% faq %}

{% faq-item question="How do I test GoFr applications?" %}
GoFr provides built-in mocks for handlers, datasources (SQL, Redis, Mongo), HTTP services, and Pub/Sub. See [Testing GoFr Applications in Go](/docs/references/testing).
{% /faq-item %}

{% /faq %}
