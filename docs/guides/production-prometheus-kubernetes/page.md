---
description: "Scrape GoFr's /metrics endpoint with kube-prometheus-stack ServiceMonitor, write golden-signal alerts, and link metrics to traces with exemplars."
nextjs:
  metadata:
    title: "GoFr Prometheus on Kubernetes - ServiceMonitor & Alerts"
    description: "Scrape GoFr's /metrics endpoint with kube-prometheus-stack ServiceMonitor, write golden-signal alerts, and link metrics to traces with exemplars."
---

# GoFr Prometheus on Kubernetes

{% answer %}
GoFr exposes Prometheus metrics on a separate port (`METRICS_PORT`, default `2121`) at `/metrics`. In Kubernetes, scrape it either with legacy `prometheus.io/*` pod annotations or — preferred — a `ServiceMonitor` from kube-prometheus-stack. Build alerts on the four golden signals (latency, errors, traffic, saturation) using the metrics GoFr emits by default plus any custom counters and histograms you register.
{% /answer %}

## When to use this guide

You have GoFr running in Kubernetes (see {% new-tab-link newtab=false title="Deploying to Kubernetes" href="/docs/guides/deploying-to-kubernetes" /%}) and either kube-prometheus-stack or a Prometheus instance scraping the cluster. This page covers the *operational* side — scraping, alerting, dashboards. For instrumenting code, see {% new-tab-link newtab=false title="Publishing Custom Metrics" href="/docs/advanced-guide/publishing-custom-metrics" /%}.

## What `/metrics` looks like

GoFr starts a separate HTTP server on `METRICS_PORT` (default `2121`) that serves Prometheus-format metrics at `/metrics`. Setting `METRICS_PORT=0` disables the server entirely — useful for short-lived CLI commands.

A truncated sample (label sets and HELP strings match the framework's actual output as of the current `pkg/gofr/container/container.go` registrations):

```text
# HELP app_http_response Response time of HTTP requests in seconds.
# TYPE app_http_response histogram
app_http_response_bucket{path="/orders",method="GET",status="200",le="0.005"} 412
app_http_response_bucket{path="/orders",method="GET",status="200",le="0.01"}  580
app_http_response_bucket{path="/orders",method="GET",status="200",le="+Inf"} 612
app_http_response_sum{path="/orders",method="GET",status="200"} 4.21
app_http_response_count{path="/orders",method="GET",status="200"} 612

# HELP app_sql_open_connections Number of open SQL connections.
# TYPE app_sql_open_connections gauge
app_sql_open_connections 4

# HELP transaction_success used to track the count of successful transactions
# TYPE transaction_success counter
transaction_success_total 87
```

Default metric names are stable (`app_http_response`, `app_sql_*`, `app_redis_*`, etc.). The OpenTelemetry-to-Prometheus exporter additionally adds `otel_scope_*` labels to every series. Custom metrics you register via `app.Metrics().NewCounter(...)` appear with the name and labels you supplied. To see the live label set against your own service, run `curl http://localhost:2121/metrics` and inspect the output directly.

## Option 1: pod annotations (older clusters / vanilla Prometheus)

If you run a single Prometheus that uses kubernetes_sd with the legacy annotation pattern, add these to your Deployment's pod template:

```yaml
spec:
  template:
    metadata:
      labels:
        app.kubernetes.io/name: orders
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "2121"
        prometheus.io/path: "/metrics"
```

These annotations only work if your Prometheus's `scrape_configs` actually relabels off them — they are a convention, not a Kubernetes feature. kube-prometheus-stack ignores them by default, which is why ServiceMonitor exists.

## Option 2: ServiceMonitor (kube-prometheus-stack — preferred)

kube-prometheus-stack ships the Prometheus Operator, which discovers scrape targets via `ServiceMonitor` and `PodMonitor` CRDs. Assuming the Service from the deployment guide names its metrics port `metrics`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: orders
  namespace: default
  labels:
    # Must match the label kube-prometheus-stack's Prometheus selects on.
    # Default helm value is `release: <release-name>`.
    release: kube-prometheus-stack
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: orders
  namespaceSelector:
    matchNames:
      - default
  endpoints:
    - port: metrics
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s
      honorLabels: true
```

Two gotchas worth knowing in advance:

- **The `release` label is not magic.** The Prometheus CR selects ServiceMonitors via `serviceMonitorSelector`. Inspect your install with `kubectl get prometheus -A -o yaml` and use whichever label that selector requires.
- **`namespaceSelector`** must include the namespace the Service lives in — otherwise the operator silently ignores the ServiceMonitor.

## Recording rules and alerts (golden signals)

The four golden signals from the SRE book — latency, traffic, errors, saturation — map cleanly onto GoFr's defaults:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: orders-rules
  namespace: default
  labels:
    release: kube-prometheus-stack
spec:
  groups:
    - name: orders.recording
      interval: 30s
      rules:
        # Traffic: requests per second.
        - record: orders:http_requests:rate1m
          expr: sum by (path, method, status) (rate(app_http_response_count{job="orders"}[1m]))

        # Errors: 5xx as a fraction of total.
        - record: orders:http_5xx_ratio:rate5m
          expr: |
            sum by (path) (rate(app_http_response_count{job="orders",status=~"5.."}[5m]))
            /
            sum by (path) (rate(app_http_response_count{job="orders"}[5m]))

        # Latency: p95 in seconds.
        - record: orders:http_p95_seconds:rate5m
          expr: |
            histogram_quantile(
              0.95,
              sum by (le, path) (rate(app_http_response_bucket{job="orders"}[5m]))
            )

    - name: orders.alerts
      rules:
        - alert: OrdersHighErrorRate
          expr: orders:http_5xx_ratio:rate5m > 0.05
          for: 10m
          labels:
            severity: page
          annotations:
            summary: "orders 5xx ratio > 5% on {{ $labels.path }}"
            description: "5xx ratio is {{ $value | humanizePercentage }} for the last 10m."

        - alert: OrdersHighLatency
          expr: orders:http_p95_seconds:rate5m > 0.5
          for: 10m
          labels:
            severity: ticket
          annotations:
            summary: "orders p95 latency > 500ms on {{ $labels.path }}"

        - alert: OrdersSaturationCPU
          expr: |
            sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="default",pod=~"orders-.*"}[5m]))
            /
            sum by (pod) (kube_pod_container_resource_limits{namespace="default",pod=~"orders-.*",resource="cpu"})
            > 0.85
          for: 15m
          labels:
            severity: ticket
          annotations:
            summary: "orders pod {{ $labels.pod }} CPU > 85% of limit"

        - alert: OrdersDown
          expr: up{job="orders"} == 0
          for: 2m
          labels:
            severity: page
          annotations:
            summary: "Prometheus cannot scrape orders"
```

Thresholds (5% errors, 500ms p95, 85% CPU saturation) are starting points — calibrate against your actual traffic pattern before paging on them.

## Dashboards

Don't ship hand-rolled dashboards if a community one will do. Good starting points:

- **Go runtime:** search the [Grafana dashboard library](https://grafana.com/grafana/dashboards/?dataSource=prometheus&search=go) for an OpenTelemetry / `go_*` runtime dashboard covering GC, goroutines, and heap.
- **Kubernetes pod resources:** kube-prometheus-stack ships `kubernetes-mixin` dashboards out of the box.
- **HTTP RED method:** any RED-method dashboard (rate / errors / duration) works against `app_http_response_*`.

For application-specific dashboards, build one panel per custom metric you register. Use the same labels in panels that you use in alerts to keep cardinality predictable.

## Exemplars: linking metrics to traces

If your Prometheus is built with exemplar support and the OpenTelemetry Collector is configured to attach exemplars to histogram buckets (via the OTLP `exemplars` feature in the SDK), you can click from a slow `histogram_quantile` panel in Grafana directly to the trace in Tempo or Jaeger. Wiring this end-to-end requires:

1. GoFr exporting traces to a Collector (see {% new-tab-link newtab=false title="Production Tracing" href="/docs/guides/production-tracing" /%}).
2. The Collector forwarding metrics + exemplars to Prometheus.
3. Grafana with the trace datasource correlated to the metrics datasource.

GoFr's HTTP histogram (`app_http_response`) records under a span, so when exemplar emission is enabled in the pipeline the `traceID` rides along with the histogram observation.

## Production tips

- **Cardinality first.** A counter labeled by `user_id` will explode your Prometheus. Stick to bounded labels — `path`, `method`, `status`, `endpoint`. See the cardinality note in {% new-tab-link newtab=false title="Publishing Custom Metrics" href="/docs/advanced-guide/publishing-custom-metrics" /%}.
- **NetworkPolicy on `2121`.** Only allow Prometheus pods to reach the metrics port. There's no auth on `/metrics` by design, and there shouldn't be — keep it network-isolated instead.
- **`honorLabels: true`** prevents Prometheus from overwriting labels your app sets (e.g., when you've used `instance` as a custom label).
- **Scrape interval == alert window divisor.** If you scrape every 30s, don't write `rate(...[10s])` — you'll get NaNs.
- **Disable metrics in CLI mode.** For one-shot CLI commands using the same binary, set `METRICS_PORT=0` so you don't bind a server you don't need.

## Verification

```bash
# Check the metrics endpoint directly.
kubectl port-forward svc/orders 2121:2121
curl -s http://localhost:2121/metrics | grep -E "^app_http_response|^transaction_success"

# Confirm Prometheus picked up the target.
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
# Open http://localhost:9090/targets — search for "orders".

# Run a query.
curl -s 'http://localhost:9090/api/v1/query?query=up{job="orders"}'

# Confirm rules loaded.
curl -s http://localhost:9090/api/v1/rules | jq '.data.groups[].name'
```

{% faq %}
{% faq-item question="My ServiceMonitor is created but Prometheus doesn't scrape." %}
Three checks in order. First, `kubectl get servicemonitor orders -o yaml` and confirm the `labels` match what your Prometheus CR's `serviceMonitorSelector` expects (often `release: <helm-release>`). Second, `namespaceSelector.matchNames` must include the Service's namespace, or use `any: true`. Third, the Service's port must have a `name` (not just a number) and the ServiceMonitor's `endpoints[].port` must reference that name.
{% /faq-item %}
{% faq-item question="Should I use ServiceMonitor or PodMonitor?" %}
Use `ServiceMonitor` when there's already a Service for your app — that's the common case, and it's the right level of indirection. Use `PodMonitor` for headless workloads or when you want to scrape every pod independently (e.g., per-pod custom counters that aren't aggregated through the Service).
{% /faq-item %}
{% faq-item question="How do I expose only `/metrics` and not `/.well-known/health` to Prometheus?" %}
GoFr already runs them on different ports — `2121` (metrics) vs `8000` (HTTP including `/.well-known/*`). Apply a NetworkPolicy that allows Prometheus to reach `2121` only, and keep ingress traffic on `8000`.
{% /faq-item %}
{% /faq %}
