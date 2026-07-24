# using-otlp-metrics

Push GoFr metrics to any OTLP collector or backend, configured entirely through
environment variables. The application code is identical to a pull-based
Prometheus setup — only `configs/.env` changes.

## How it works

GoFr builds its meter with **both** a Prometheus pull reader and (when
`METRICS_EXPORTER=otlp`) an OTLP push reader on the same instruments — no
double-counting. So `/metrics` stays scrapeable *and* metrics are pushed.

| Config | Meaning |
|--------|---------|
| `METRICS_EXPORTER` | `otlp` to enable push (unset/`prometheus` = pull only) |
| `METRICS_URL` | collector/backend endpoint (`host:port` for gRPC, URL for HTTP) |
| `METRICS_PROTOCOL` | `grpc` (default) or `http` |
| `METRICS_EXPORT_INTERVAL` | push interval, seconds (default `30`) |
| `METRICS_TEMPORALITY` | `cumulative` (default), `delta`, or `lowmemory` |
| `METRICS_HEADERS` | `key=value,key=value` request headers (auth, routing) |
| `METRICS_AUTH_KEY` | shorthand: sets the `Authorization` header |
| `METRICS_INSECURE` | `true` (default) disables TLS; set `false` for managed backends |
| `METRICS_PORT` | pull endpoint port; `0` disables it (push-only) |

Two switches, fully independent: `METRICS_PORT` controls pull, `METRICS_EXPORTER`
controls push. Push-only serverless mode = `METRICS_EXPORTER=otlp` + `METRICS_PORT=0`.

## Run locally

Start a collector (prints received metrics, re-exposes them for Prometheus):

```bash
cd docker && docker compose up
```

Then run the app and generate some metrics:

```bash
go run .
curl -XPOST localhost:9016/order
```

Watch them arrive in the collector's logs, or browse Prometheus at
`localhost:9090` (query `orders_processed`).

## Backend recipes (env only — no code changes)

**OpenTelemetry Collector / sidecar** (as above)
```
METRICS_EXPORTER=otlp
METRICS_URL=localhost:4317
```

**Datadog** — HTTP intake, delta temporality required, static API key:
```
METRICS_EXPORTER=otlp
METRICS_PROTOCOL=http
METRICS_URL=https://api.datadoghq.com/api/intake/otlp/v1/metrics
METRICS_TEMPORALITY=delta
METRICS_HEADERS=dd-api-key=<YOUR_DD_API_KEY>
METRICS_INSECURE=false
```

**Grafana Cloud** — OTLP gateway, Basic auth:
```
METRICS_EXPORTER=otlp
METRICS_PROTOCOL=http
METRICS_URL=https://otlp-gateway-<region>.grafana.net/otlp
METRICS_AUTH_KEY=Basic <base64(instanceID:token)>
METRICS_INSECURE=false
```

**New Relic** — OTLP endpoint, license-key header:
```
METRICS_EXPORTER=otlp
METRICS_URL=otlp.nr-data.net:4317
METRICS_HEADERS=api-key=<YOUR_NR_LICENSE_KEY>
METRICS_INSECURE=false
```

**Google Managed Prometheus (keyless)** — see `examples/using-gcp-metrics`.
