---
description: "GoFr vs Fiber: Fiber leads on raw HTTP throughput thanks to fasthttp; GoFr ships built-in observability, datasources, gRPC, GraphQL, Pub/Sub, and migrations. With code examples."
nextjs:
  metadata:
    title: "GoFr vs Fiber — Performance vs Production Stack"
    description: "GoFr vs Fiber: Fiber leads on raw HTTP throughput thanks to fasthttp; GoFr ships built-in observability, datasources, gRPC, GraphQL, Pub/Sub, and migrations. With code examples."
---

# GoFr vs Fiber

{% answer %}
**Fiber** is an Express-inspired HTTP framework built on `fasthttp` — a great choice when you want a familiar API for Node.js refugees and high HTTP throughput. **GoFr** sits on `net/http` and has a wider scope: alongside HTTP routing it bundles OpenTelemetry tracing, Prometheus metrics, datasource clients, gRPC, GraphQL, WebSockets, Pub/Sub, cron, migrations, and circuit breakers. Different trade-offs, both open source — pick whichever fits the work in front of you.
{% /answer %}

## What Fiber is great at

- **Performance** — built on `fasthttp`, regularly outperforms `net/http`-based frameworks on synthetic benchmarks.
- **Express-like API** — feels natural for developers from Node.js.
- **Built-in WebSocket and SSE** — rich HTTP feature set out of the box.
- **Active ecosystem** — many official middleware packages.

## Where they diverge

### HTTP foundation

Fiber's foundation is `fasthttp`, which is **not compatible with `net/http`**. Some Go libraries assume `http.ResponseWriter`/`http.Request` and won't drop into a Fiber handler without an adapter. GoFr is built on `net/http`, so the standard library and any `net/http`-compatible middleware works.

### Scope beyond HTTP

Fiber focuses on HTTP. For other protocols, you'd add separate libraries (which works well — the Go ecosystem has good options for each). GoFr bundles those protocols under the same configuration and observability:

```go
app.GRPCRegisterService(...)
app.GraphQLQuery("user", userResolver)
app.Subscribe("orders", orderHandler)
app.AddCronJob("@hourly", "billing", run)
```

### Observability and datasources

Fiber middleware exists for OpenTelemetry and Prometheus, but you wire them in. In GoFr, traces / metrics / structured logs are emitted by default with no setup beyond pointing at your collectors via env vars. GoFr ships clients for MySQL, PostgreSQL, Mongo, Redis, Cassandra, ClickHouse, Kafka, NATS, S3, GCS, and a dozen more — all auto-instrumented.

## When GoFr might be a good fit

- You'd like gRPC, Pub/Sub, GraphQL, WebSockets, or cron alongside HTTP without separately wiring them up.
- OpenTelemetry tracing and Prometheus metrics by default fit your operational model.
- Auto-instrumented database clients save you wiring time you'd rather spend elsewhere.
- You're maintaining several services and would prefer a single configuration model across them.

## Migration

Already on Fiber? See the [Migrate from Fiber guide](/migrate/from-fiber) for concrete code translations.

{% faq %}

{% faq-item question="Can Fiber use net/http middleware?" %}
With an adapter, yes — Fiber provides `adaptor.HTTPHandler` to wrap `net/http` middleware. There's a small overhead per call. GoFr uses `net/http` natively, so no adapter is needed.
{% /faq-item %}

{% faq-item question="Does GoFr have a fasthttp-based mode?" %}
No. GoFr is built on `net/http` and prioritizes ecosystem compatibility.
{% /faq-item %}

{% /faq %}
