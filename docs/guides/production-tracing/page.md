---
description: "Configure GoFr OpenTelemetry tracing in production with OTLP, Jaeger, or Tempo, sampling via TRACER_RATIO, and propagation across services."
nextjs:
  metadata:
    title: "GoFr Production Tracing - OpenTelemetry, OTLP, Jaeger"
    description: "Configure GoFr OpenTelemetry tracing in production with OTLP, Jaeger, or Tempo, sampling via TRACER_RATIO, and propagation across services."
---

# Production Tracing for GoFr

{% answer %}
GoFr ships built-in OpenTelemetry tracing — every HTTP request, gRPC call, and datasource operation is traced automatically. Configure the exporter via `TRACE_EXPORTER` (`otlp`, `jaeger`, `zipkin`, or `gofr`) and `TRACER_URL`, set `TRACER_RATIO` for head-based sampling, and W3C Trace Context propagation flows through GoFr's HTTP service client without extra code.
{% /answer %}

## When to use this guide

You have GoFr running in Kubernetes (or any container platform) and want traces flowing into a backend — Jaeger, Grafana Tempo, an OpenTelemetry Collector, or a vendor that accepts OTLP. This guide covers exporter configuration, sampling, and propagation across multiple services.

For adding application-level spans inside handlers, see {% new-tab-link newtab=false title="Custom Spans In Tracing" href="/docs/advanced-guide/custom-spans-in-tracing" /%}.

## What GoFr traces automatically

Once tracing is enabled, GoFr instruments without code changes:

- **HTTP server** — every incoming request becomes a root span (or a child if upstream sent W3C trace headers).
- **HTTP client** — outgoing calls via the GoFr service client (with circuit breaker / retry / rate limit) are traced and propagate context.
- **gRPC** — server and client interceptors emit spans.
- **Datasources** — SQL, Redis, Mongo, Cassandra, Pub/Sub publishers and subscribers (Kafka, NATS, SQS, Google Pub/Sub) emit spans for each operation.
- **Migrations** — recorded as spans, useful for debugging long-running schema changes.

What custom spans add (`ctx.Trace("name")`) is application logic — business operations that span multiple datasource calls or pure-CPU work you want to time.

## Configuration

GoFr reads tracing config from environment variables. The relevant keys (verified against `pkg/gofr/otel.go`):

| Variable | Purpose | Default |
|---|---|---|
| `TRACE_EXPORTER` | One of `otlp`, `jaeger`, `zipkin`, `gofr` | unset (tracing disabled) |
| `TRACER_URL` | Endpoint for the chosen exporter | unset |
| `TRACER_HOST` | **Deprecated** — use `TRACER_URL` | unset |
| `TRACER_PORT` | **Deprecated** — use `TRACER_URL` | `9411` |
| `TRACER_RATIO` | Head-based sampling ratio (0.0–1.0) | `1` |
| `TRACER_HEADERS` | Custom OTLP headers, `Key1=Value1,Key2=Value2` | unset |
| `TRACER_AUTH_KEY` | Shortcut for `Authorization` header | unset |

Tracing is **disabled** if neither `TRACE_EXPORTER` nor `TRACER_URL` is set — GoFr logs `tracing is disabled, as configs are not provided` at debug level. The sampler is `ParentBased(TraceIDRatioBased(TRACER_RATIO))`, so a sampling decision made upstream is honored.

`zipkin` is supported but deprecated; the framework logs a warning recommending `otlp` instead. The `gofr` exporter ships traces to GoFr's hosted tracer at `https://tracer-api.gofr.dev/api/spans` (override with `TRACER_URL`).

## Backend recipes

### Jaeger (OTLP gRPC)

Modern Jaeger (1.35+) accepts OTLP natively on port `4317`:

```yaml
# ConfigMap fragment
TRACE_EXPORTER: "jaeger"
TRACER_URL: "jaeger-collector.observability.svc.cluster.local:4317"
TRACER_RATIO: "0.1"
```

`jaeger` and `otlp` use the same OTLP gRPC exporter under the hood — they differ only in log labeling.

### Grafana Tempo / OpenTelemetry Collector

Point at any OTLP gRPC endpoint:

```yaml
TRACE_EXPORTER: "otlp"
TRACER_URL: "otel-collector.observability.svc.cluster.local:4317"
TRACER_RATIO: "0.1"
```

Running an OTel Collector as a sidecar or DaemonSet is the recommended pattern: it does tail-based sampling, batching, and can fan out to multiple backends without changing the app.

### Honeycomb / Datadog / Vendor OTLP

For SaaS backends that accept OTLP and require an API key:

```yaml
TRACE_EXPORTER: "otlp"
TRACER_URL: "api.honeycomb.io:443"
TRACER_HEADERS: "x-honeycomb-team=YOUR_API_KEY,x-honeycomb-dataset=orders"
TRACER_RATIO: "0.1"
```

Or with a single auth header:

```yaml
TRACER_AUTH_KEY: "Bearer YOUR_TOKEN"
```

GoFr's OTLP exporter currently uses an insecure (cleartext) gRPC connection inside the cluster — for SaaS endpoints over the public internet, route through an OTel Collector that terminates TLS, or rely on a service mesh.

## Sampling: head-based vs tail-based

`TRACER_RATIO` is **head-based**: the sampling decision is made when the trace starts. With `TRACER_RATIO=0.1`, 10% of root spans are kept; the other 90% are dropped at the source. Cheap, predictable, but you cannot retroactively keep a slow or errored trace that wasn't sampled.

For production-grade observability, **tail-based** sampling — done in an OpenTelemetry Collector with the `tail_sampling` processor — lets you keep all traces that contain errors or exceed a latency threshold while sub-sampling the happy path. The pattern is: app sends 100% (or a high ratio) to the local collector; collector decides what to ship onward.

A starting matrix:

| Environment | `TRACER_RATIO` | Notes |
|---|---|---|
| Local dev | `1` | See everything |
| Staging | `1` | Catch issues before prod |
| Production (low traffic, < 50 RPS) | `1` | Volume is fine |
| Production (high traffic) | `0.05`–`0.1` | Or sample 100% to a collector and tail-sample there |

## Propagation across services

GoFr sets up a `CompositeTextMapPropagator(TraceContext{}, Baggage{})`, so the W3C `traceparent` and `baggage` headers are honored on incoming requests and written on outgoing requests through the GoFr HTTP service client. No extra code is needed:

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"
)

func main() {
	app := gofr.New()

	app.AddHTTPService("payments", "http://payments.default.svc.cluster.local")

	app.GET("/checkout", func(ctx *gofr.Context) (any, error) {
		span := ctx.Trace("checkout.compute-total")
		defer span.End()

		// The downstream span on payments will be a child of this trace.
		var resp any
		err := ctx.GetHTTPService("payments").
			GetWithHeaders(ctx, "/charge", nil, nil, &resp)
		if err != nil {
			return nil, err
		}

		return resp, nil
	})

	app.Run()
}
```

The downstream `payments` service — also a GoFr app pointed at the same exporter — will record its spans as children of the same trace. In Jaeger or Tempo, you'll see the full chain end-to-end.

## Production tips

- **One exporter, many services:** point all your services at the same collector. Querying a trace that hops services is the whole point.
- **Resource attributes:** GoFr sets `service.name` from `APP_NAME` (default `gofr-app`). Set `APP_NAME` per-deployment so traces are attributable.
- **Don't sample on the client when you can sample on the collector** — once dropped at the source, a trace is gone forever.
- **Watch the exporter error log:** GoFr installs a custom OTel error handler (`otelErrorHandler`) that logs exporter failures via the standard logger. If you see these in volume, your collector is unreachable or overwhelmed.
- **Trace IDs in logs:** include the trace ID in your logs to jump from a noisy log line to its trace. GoFr's structured logger and trace context share `*gofr.Context`, so you can read `span.SpanContext().TraceID()` and log it.

## Verification

```bash
# 1. Confirm env is set inside the pod.
kubectl exec deploy/orders -- env | grep -E "TRACE_|TRACER_"

# 2. Generate traffic.
kubectl port-forward svc/orders 8080:80
for i in $(seq 1 50); do curl -s http://localhost:8080/checkout > /dev/null; done

# 3. Confirm spans are flowing in the collector or backend logs.
kubectl logs -n observability deploy/otel-collector | grep -i orders

# 4. Open Jaeger UI and search service=orders.
kubectl port-forward -n observability svc/jaeger-query 16686:16686
# http://localhost:16686
```

{% faq %}
{% faq-item question="Tracing is configured but I see no spans in the backend." %}
Check three things in order. First, GoFr logs `Exporting traces to <name> at <url>` on startup — if absent, the exporter never initialized; verify `TRACE_EXPORTER` is one of `otlp`, `jaeger`, `zipkin`, or `gofr`. Second, port-forward to the collector and confirm gRPC `4317` is reachable from the pod. Third, check `TRACER_RATIO` — `0` would silently drop everything.
{% /faq-item %}
{% faq-item question="Why are my downstream service's spans showing up as separate traces?" %}
The downstream call must go through GoFr's HTTP service client (`app.AddHTTPService` + `ctx.GetHTTPService`). A raw `http.Client` will not inject the `traceparent` header. If you must use a custom client, wrap its transport with `otelhttp.NewTransport`.
{% /faq-item %}
{% faq-item question="Do I need a Collector, or can I send directly to Jaeger/vendor?" %}
You can send directly — GoFr's OTLP exporter speaks OTLP gRPC to anything that accepts it. A Collector becomes worth it when you want tail-based sampling, batching across many services, or to swap backends without redeploying every service.
{% /faq-item %}
{% /faq %}
