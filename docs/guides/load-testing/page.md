---
description: "Load test GoFr services with k6 and vegeta: scripts, what to measure (p50/p95/p99, error rate, throughput), Prometheus capture, and bottleneck triage."
nextjs:
  metadata:
    title: "GoFr Load Testing: k6, vegeta, and Performance Baselines"
    description: "Load test GoFr services with k6 and vegeta: scripts, what to measure (p50/p95/p99, error rate, throughput), Prometheus capture, and bottleneck triage."
---

# Load Testing

{% answer %}
Load test GoFr services with k6 or vegeta from outside the cluster, scrape Prometheus during the run for server-side truth, and look at p50/p95/p99 latency, error rate, and throughput together — never just an average. The framework gives you the metrics surface; the test is your responsibility.
{% /answer %}

## What to measure

A single number ("we did 5k RPS") is not enough. Always report the tuple:

- **Latency percentiles** — p50, p95, p99. Averages hide tails.
- **Error rate** — non-2xx and timeouts as a percentage of total.
- **Throughput** — RPS sustained without error rate climbing.
- **Saturation** — CPU, memory, DB connections in use, GC pause time. Latency degrades sharply once any of these saturate.

Run the test long enough to see whether numbers are stable. The first 30 seconds usually contain JIT/warmup artifacts.

## k6 example

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },   // ramp up
    { duration: '2m',  target: 50 },   // steady
    { duration: '30s', target: 200 },  // step up
    { duration: '2m',  target: 200 },  // steady
    { duration: '30s', target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<300', 'p(99)<800'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const res = http.get('https://api.example.com/orders/42');
  check(res, { 'status is 200': (r) => r.status === 200 });
  sleep(1);
}
```

Run with `k6 run --out json=results.json script.js`. Thresholds turn the run into a pass/fail.

## vegeta example

For simpler GET-heavy tests, vegeta is one shell command:

```bash
echo "GET https://api.example.com/orders/42" | \
  vegeta attack -rate=100 -duration=2m | \
  vegeta report -type=hist[0,10ms,50ms,100ms,500ms,1s]
```

vegeta also writes raw results that you can replay through `vegeta plot` for visual inspection.

## Capturing GoFr metrics during the test

GoFr exposes Prometheus metrics on `METRICS_PORT` (default 2121, see `pkg/gofr/factory.go`). Scrape them during the run for server-side truth. Useful series:

- HTTP request latency histograms (p50/p95/p99 per route).
- Request count and status code distribution.
- Outbound HTTP service circuit breaker state — `app_http_circuit_breaker_state` (see [Circuit Breaker](/docs/advanced-guide/circuit-breaker)).
- Go runtime: `go_goroutines`, `go_gc_duration_seconds`, `process_resident_memory_bytes`.

A 3-minute test should be reflected in a Grafana dashboard with at least these panels open. If client-side and server-side latency diverge, suspect the network or the load generator.

## Bottleneck triage

When latency rises, look in this order:

1. **Application CPU** — saturated CPU means you are compute-bound or doing too much per request. Profile with `pprof`. Add `app.GET("/debug/pprof/profile", ...)` only in non-prod.
2. **Database** — slow queries, connection pool exhaustion, lock waits. Check the SQL datasource's pool stats and DB-side metrics. `MaxOpenConns` is often the culprit.
3. **Downstream services** — GoFr's outbound HTTP client metrics show which downstream is slowing. Circuit breaker transitions are visible via `app_http_circuit_breaker_state`.
4. **GC** — long GC pauses correlate with allocation in hot paths. `go_gc_duration_seconds` and `runtime/metrics` show this.
5. **Network** — load generator can't push more, or NLB/ALB connection limits. Run the generator from inside the cluster to compare.

## Establish a baseline before changes

Run the same scenario monthly and on every major release. Save the k6/vegeta output and the Prometheus snapshots. Regressions become obvious only when there is a baseline to compare against. This guide deliberately does not publish baseline numbers — they depend entirely on your hardware, payload, and dependencies.

## Test from a realistic location

Running k6 from a developer laptop hits the public Internet path. That is fine for end-to-end SLO checks, but if you want to know "how fast is GoFr itself", run the generator inside the same cluster on a pod with no resource limits, hitting the Service ClusterIP. That isolates the application from edge variability.

## Avoiding self-DoS

- Do not point load tests at a shared production database.
- Spin up a dedicated namespace with the same Helm values as production but a separate datasource.
- Cap `vus` (virtual users) below the connection ceiling of any rate-limited downstream you cannot mock.

## Reporting

Capture, for each run:

- Test scenario (request mix, ramp profile, total duration).
- Service version (image SHA).
- p50/p95/p99 latency, error rate, throughput.
- Resource usage at peak (CPU%, memory, DB connections).
- The git commit and any feature flags toggled.

This metadata is what makes a regression diagnosable a month later.

{% faq %}
{% faq-item question="Where do I scrape GoFr's metrics during a load test?" %}
On the metrics port, default 2121, configurable via `METRICS_PORT`. Path is `/metrics`. Set `METRICS_PORT=0` to disable.
{% /faq-item %}
{% faq-item question="Should I report average latency?" %}
No. Average hides tail latency, which is where users feel pain. Always report p50, p95, and p99 together, plus error rate.
{% /faq-item %}
{% faq-item question="k6 or vegeta?" %}
k6 is better for scripted scenarios with thresholds and assertions. vegeta is faster to set up for steady-rate GET load. Both work fine against GoFr.
{% /faq-item %}
{% /faq %}
