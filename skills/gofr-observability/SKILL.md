---
name: gofr-observability
description: Instrument a GoFr service — structured logs, Prometheus metrics, custom OpenTelemetry spans, health and readiness endpoints, remote log-level changes, and pprof profiling. Use when adding monitoring to a Go service or debugging one in production.
license: Apache-2.0
---

# Observability in GoFr

`gofr.New()` already emits structured logs, OpenTelemetry traces, and Prometheus
metrics for every request, every datasource call, and every outbound HTTP call.
The work is adding your own signals on top — not wiring the pipeline.

Do not import `go.opentelemetry.io/otel` and build a TracerProvider by hand. It
is already running, and a second one will silently split your traces.

## Logging

```go
c.Logger.Info("processing order", "orderID", id)
c.Logger.Errorf("failed to reserve stock: %v", err)
```

Logs are JSON in production and human-readable locally, and every line carries
the trace ID, so a log line links back to its request.

Change the level of a running service without redeploying — see the
remote-log-level guide below.

## Custom metrics

Register once at startup, then record from handlers:

```go
app.Metrics().NewCounter("orders_placed_total", "Number of orders placed")
app.Metrics().NewHistogram("order_value_dollars", "Order value", 10, 50, 100, 500)

// in a handler
c.Metrics().IncrementCounter(c, "orders_placed_total")
c.Metrics().RecordHistogram(c, "order_value_dollars", value)
```

Metrics are exposed on the metrics port (default 2121) at `/metrics`.

## Custom spans

```go
span := c.Trace("reserve-stock")
defer span.End()
```

Pass `c` to downstream calls so their spans nest under the request rather than
starting a new trace.

## Health

`/.well-known/health` reports the service and every registered datasource.
`/.well-known/alive` is the liveness probe. Both exist already — point your
Kubernetes probes at them rather than writing `/healthz`.

## Read next

- <https://gofr.dev/docs/quick-start/observability.md>
- <https://gofr.dev/docs/advanced-guide/publishing-custom-metrics.md>
- <https://gofr.dev/docs/advanced-guide/custom-spans-in-tracing.md>
- <https://gofr.dev/docs/advanced-guide/remote-log-level-change.md>
- <https://gofr.dev/docs/advanced-guide/debugging.md>
