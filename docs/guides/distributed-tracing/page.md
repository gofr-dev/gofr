---
description: "Distributed tracing in GoFr: W3C TraceContext propagation across HTTP and gRPC, sampling, and stitching the request path together in Jaeger or Tempo."
nextjs:
  metadata:
    title: "GoFr Distributed Tracing: W3C TraceContext Across Services"
    description: "Distributed tracing in GoFr: W3C TraceContext propagation across HTTP and gRPC, sampling, and stitching the request path together in Jaeger or Tempo."
---

# Distributed Tracing

{% answer %}
GoFr uses OpenTelemetry with W3C TraceContext + Baggage propagators by default. Inbound HTTP requests have their `traceparent` extracted, and GoFr's outbound HTTP service client injects the same headers downstream — so a request crossing five GoFr services shows up as one trace in Jaeger when they all export to the same backend.
{% /answer %}

## What GoFr propagates

OpenTelemetry setup in GoFr (verified in `pkg/gofr/otel.go`) configures:

```go
otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{}, propagation.Baggage{}))
```

That means every GoFr binary, on boot, registers two propagators:

- **W3C TraceContext** — the standard `traceparent` and `tracestate` headers.
- **W3C Baggage** — the `baggage` header for cross-service key/value context.

The HTTP server middleware extracts both on every inbound request (`pkg/gofr/http/middleware/tracer.go`). The HTTP service client injects both on every outbound request (`pkg/gofr/service/new.go`).

## Trace ID format

W3C trace IDs are 16-byte (32 hex character) values; span IDs are 8-byte (16 hex). When a request carries a trace context, GoFr's logger writes the trace ID to the top-level `trace_id` field on the JSON log envelope (the field is `omitempty`, so it only appears when a trace context is set). On HTTP request logs specifically, the `message` field is itself a `RequestLog` struct that carries its own nested `trace_id` and `span_id` alongside `method`, `uri`, etc. — that nested copy is only present on the HTTP middleware's request log line, not on every log entry. Correlate logs and traces using either occurrence (see [Production Logging](/docs/advanced-guide/production-logging) for the exact log shape and shipper configuration).

A trace looks the same across HTTP and gRPC: the same trace ID, with each service contributing one or more spans.

## Configuration

Tracing is opt-in. Set:

| Env var          | Purpose                                              | Notes                                    |
|------------------|------------------------------------------------------|------------------------------------------|
| `TRACE_EXPORTER` | `otlp`, `jaeger`, `zipkin` (deprecated)              | Required to enable                       |
| `TRACER_URL`     | Endpoint URL or `host:port`                          | Required when exporter is set            |
| `TRACER_RATIO`   | Sample ratio (0.0–1.0)                               | Defaults to `1` (100%)                   |
| `TRACER_HEADERS` | Custom headers (e.g., for SaaS auth)                 | Comma-separated `key=value` pairs        |
| `TRACER_AUTH_KEY`| Single auth header value                             | Use `TRACER_HEADERS` for multiple        |

The `zipkin` value emits a deprecation warning at startup and recommends switching to `otlp` (verified in `pkg/gofr/otel.go`).

## End-to-end example

Service A receives an HTTP request, calls Service B over HTTP, which writes to a database. With GoFr defaults, the trace contains:

1. Server span on A (from the HTTP middleware).
2. Custom application spans on A if you call `c.Trace("step-name")` — see [Custom Spans in Tracing](/docs/advanced-guide/custom-spans-in-tracing).
3. Client span on A's outbound HTTP call to B.
4. Server span on B from the same `traceparent`.
5. Spans for B's database call (when using GoFr's instrumented datasources).

Both A and B point `TRACE_EXPORTER=otlp` and `TRACER_URL` at the same collector. In Jaeger or Tempo, a single search by trace ID shows the whole path.

## gRPC

The gRPC server in GoFr also participates in tracing. Cross-protocol traces — HTTP into gRPC and back — work because both servers use the same OpenTelemetry SDK and propagators.

## Pub/Sub

Trace propagation through a message bus is partial. The Google Pub/Sub datasource injects trace context into message attributes (verified in `pkg/gofr/datasource/pubsub/google/tracing.go`), which means a producer span and consumer span share the same trace ID. Other Pub/Sub backends (Kafka, NATS, SQS, MQTT, EventHub) may or may not propagate trace context end-to-end; if your span graph breaks at the bus, log the trace ID into the message payload manually as a fallback so downstream logs can still be correlated.

## Sampling: keep it consistent

If A samples at 10% and B samples at 100%, you'll have lots of B-only traces with no parent — useless. Two rules:

- Use head-based sampling (`TRACER_RATIO`) consistently across all services in the request path.
- Or use tail-based sampling at the collector level, where the decision happens after the trace is assembled.

For a typical OTLP collector setup, configure tail sampling once in the collector and set `TRACER_RATIO=1` in every service so all spans are exported and the collector decides what to keep.

## Visualizing in Jaeger

Run Jaeger in OTLP-receiver mode, point GoFr at the gRPC OTLP endpoint (typically `host:4317`), and traces appear in the Jaeger UI within a second or two.

```dotenv
TRACE_EXPORTER=otlp
TRACER_URL=jaeger.observability.svc.cluster.local:4317
TRACER_RATIO=1
```

For Tempo or Honeycomb, point at their OTLP gRPC endpoints and add `TRACER_HEADERS` for any required auth.

## Custom application spans

For business-level operations inside a handler, wrap them with `c.Trace("name")`. This is the recommended way to add span granularity without touching the OTel SDK directly. See [Custom Spans in Tracing](/docs/advanced-guide/custom-spans-in-tracing).

## Common gotchas

- **Mismatched propagators** — if a non-GoFr service in the chain uses B3 instead of W3C, traces split. Standardize on W3C TraceContext across the fleet.
- **Sidecar tracing** — Istio and Linkerd inject their own spans. Configure them to use the same backend, not a parallel one.
- **Logs without trace IDs** — if `trace_id` is empty in a log, the request didn't carry a `traceparent`. Likely the entry point (Ingress, gateway) is not adding one.
- **High cardinality span names** — never put a path parameter (e.g., `/orders/12345`) directly in a span name. Use the route template.

## What spans cost

Each exported span is a few hundred bytes over the network plus storage. At 100% sampling and high RPS, span volume can dominate egress. Sample the way you sample logs: aggressively for routine traffic, fully for errors and slow requests (tail-based sampling).

{% faq %}
{% faq-item question="Which trace propagation format does GoFr use?" %}
W3C TraceContext (`traceparent`, `tracestate`) and W3C Baggage. They are registered as the global OpenTelemetry propagators on app startup.
{% /faq-item %}
{% faq-item question="Are HTTP and gRPC traces stitched together automatically?" %}
Yes. Both protocols run through the same OpenTelemetry SDK and propagators in GoFr, so a trace that hops HTTP → gRPC → HTTP shows up as one trace in the backend.
{% /faq-item %}
{% faq-item question="How do I correlate logs with a trace?" %}
When a request has a trace context, GoFr writes the trace ID into the JSON log envelope's top-level `trace_id` field and into the nested `message` object on HTTP request logs. Search your logging backend for that value, after configuring your shipper to extract it from both spots, and you get every log entry for the request.
{% /faq-item %}
{% /faq %}
