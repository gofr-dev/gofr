---
description: "Scale GoFr services on Kubernetes with HPA v2 using CPU and custom Prometheus metrics like requests-per-second from GoFr's metrics endpoint."
nextjs:
  metadata:
    title: "GoFr Horizontal Pod Autoscaler: HPA with Custom Metrics"
    description: "Scale GoFr services on Kubernetes with HPA v2 using CPU and custom Prometheus metrics like requests-per-second from GoFr's metrics endpoint."
---

# Horizontal Pod Autoscaler for GoFr

{% answer %}
GoFr exposes Prometheus metrics on `METRICS_PORT` (default 2121), which Kubernetes HPA v2 can read through prometheus-adapter. You can scale on CPU plus custom application signals, such as requests-per-second derived from GoFr's default HTTP histogram, by writing a discovery rule in the adapter and a `HorizontalPodAutoscaler` manifest that references it.
{% /answer %}

## When to use

Reach for HPA when traffic is bursty and a fixed replica count either over-provisions during quiet periods or under-serves during spikes. CPU autoscaling alone tends to lag behind I/O-bound workloads — a GoFr service waiting on a downstream HTTP call has low CPU but a long queue. Custom-metric HPA on QPS or latency closes that gap. For event-driven workloads (Kafka, NATS, MQTT) HPA cannot scale to zero; use [KEDA](https://keda.sh) for that.

## GoFr metrics that drive HPA

GoFr publishes a {% new-tab-link newtab=true title="default set of HTTP, datasource, and runtime metrics" href="/docs/quick-start/observability" /%} on `METRICS_PORT` at `/metrics`. The HTTP server records `app_http_response` (a histogram), so requests-per-second can be derived as `rate(app_http_response_count[1m])`. You can also publish your own counters and histograms — see [Publishing Custom Metrics](/docs/advanced-guide/publishing-custom-metrics).

Make sure your Pod template advertises the metrics port and a Prometheus scrape annotation (or a `ServiceMonitor` if you run prometheus-operator):

```yaml
ports:
  - name: http
    containerPort: 8000
  - name: metrics
    containerPort: 2121
```

## prometheus-adapter rule

prometheus-adapter exposes Prometheus series as `custom.metrics.k8s.io` so HPA can query them. A minimal rule that surfaces per-pod RPS for a GoFr Deployment looks like:

```yaml
rules:
  - seriesQuery: 'app_http_response_count{namespace!="",pod!=""}'
    resources:
      overrides:
        namespace: { resource: namespace }
        pod:       { resource: pod }
    name:
      matches: "^app_http_response_count$"
      as: "http_requests_per_second"
    metricsQuery: |
      sum(rate(<<.Series>>{<<.LabelMatchers>>}[1m])) by (<<.GroupBy>>)
```

Verify with `kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/<ns>/pods/*/http_requests_per_second"`.

## HPA v2 manifest

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: orders-api
  namespace: prod
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: orders-api
  minReplicas: 3
  maxReplicas: 30
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Pods
      pods:
        metric:
          name: http_requests_per_second
        target:
          type: AverageValue
          averageValue: "50"
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 30
      policies:
        - type: Percent
          value: 100
          periodSeconds: 30
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Percent
          value: 25
          periodSeconds: 60
```

The `behavior` block is the difference between an HPA that flaps and one that holds. Short `scaleUp.stabilizationWindowSeconds` reacts to bursts; long `scaleDown.stabilizationWindowSeconds` prevents thrashing when traffic drops momentarily.

## Gotchas

- **Cold starts.** A new GoFr pod must finish [OnStart hooks](/docs/advanced-guide/startup-hooks) (cache warmup, migrations) before serving. Set `minReadySeconds` on the Deployment and a `readinessProbe` against `/.well-known/health` so HPA doesn't count not-ready pods toward capacity.
- **Resource requests are mandatory.** HPA's CPU calculation is `usage / request`. If the Deployment omits `resources.requests.cpu`, CPU-based scaling is silently disabled.
- **HPA cannot scale to zero.** `minReplicas: 0` is rejected by the API server. If you need scale-to-zero for cron-like workloads, use KEDA.
- **Adapter discovery interval.** prometheus-adapter polls Prometheus every 30s by default. New metric series take up to a minute to appear in `custom.metrics.k8s.io`.

## Verification

```bash
kubectl get hpa orders-api -n prod
kubectl describe hpa orders-api -n prod
kubectl top pods -n prod -l app=orders-api
```

`describe` prints the `Metrics` block with current vs target values; mismatched units (e.g., `m` vs whole numbers) are the most common reason HPA reports `unknown`.

{% faq %}
{% faq-item question="Does GoFr need any code changes for HPA to work?" %}
No. GoFr already exposes Prometheus-format metrics on `METRICS_PORT` (default 2121). HPA configuration lives entirely in the adapter rule and the HPA manifest.
{% /faq-item %}
{% faq-item question="Can I scale a GoFr Pub/Sub subscriber with HPA?" %}
HPA can scale on CPU, but consumer-lag-based scaling is better handled by KEDA's Kafka or NATS scalers, which can also scale to zero between batches.
{% /faq-item %}
{% faq-item question="Why does my HPA show <unknown> for the custom metric?" %}
Either prometheus-adapter has not discovered the series yet, the metric name in the HPA manifest does not match the rule's `as:` value, or the labels (`namespace`, `pod`) are missing on the Prometheus series.
{% /faq-item %}
{% /faq %}
