# Custom Metrics Example

This GoFr example demonstrates the use of custom metrics through a simple HTTP server that creates and populates metrics.
GoFr by default pushes metrics to port `2121` on `/metrics` endpoint.

### To run the example use the command below:
```console
go run main.go
```

## Exporting via OTLP (push)

The same application can **push** these metrics to any OTLP collector or backend
without touching the Go code — it's purely a `configs/.env` change. GoFr builds
its meter with **both** a Prometheus pull reader and (when `METRICS_EXPORTER=otlp`)
an OTLP push reader on the same instruments, so `/metrics` stays scrapeable *and*
metrics are pushed — no double-counting.

| Config | Meaning |
|--------|---------|
| `METRICS_EXPORTER` | `otlp` to enable push (unset/`prometheus` = pull only) |
| `METRICS_URL` | collector/backend endpoint (`host:port` for gRPC, URL for HTTP) |
| `METRICS_PROTOCOL` | `grpc` (default) or `http` |
| `METRICS_EXPORT_INTERVAL` | push interval, seconds (default `30`) |
| `METRICS_TEMPORALITY` | `cumulative` (default), `delta`, or `lowmemory` |
| `METRICS_HEADERS` | `key=value,key=value` request headers (auth, routing) |
| `METRICS_AUTH_KEY` | shorthand: sets the `Authorization` header |
| `METRICS_INSECURE` | `false` (default) keeps TLS; set `true` only for a local plaintext collector |
| `METRICS_PORT` | pull endpoint port; `0` disables it (push-only) |

`METRICS_PORT` controls pull and `METRICS_EXPORTER` controls push, fully
independently. Push-only serverless mode = `METRICS_EXPORTER=otlp` + `METRICS_PORT=0`.

### Run locally

Start a collector (prints received metrics, re-exposes them for Prometheus):

```bash
cd docker && docker compose up
```

Add the OTLP settings to `configs/.env` — a local collector listens in plaintext,
so this is one of the few cases where `METRICS_INSECURE=true` is appropriate:

```
METRICS_EXPORTER=otlp
METRICS_URL=localhost:4317
METRICS_INSECURE=true
```

Then run the app and generate some metrics:

```bash
go run .
curl -XPOST localhost:9011/transaction
```

Watch them arrive in the collector's logs, or browse Prometheus at
`localhost:9090`. If nothing shows up, confirm `METRICS_EXPORTER=otlp` is set —
without it the app only serves the pull endpoint.

### Backend recipes (env only — no code changes)

Managed backends terminate TLS, so keep the secure default (`METRICS_INSECURE`
unset/`false`).

**Datadog** — HTTP intake, delta temporality required, static API key:
```
METRICS_EXPORTER=otlp
METRICS_PROTOCOL=http
METRICS_URL=https://api.datadoghq.com/api/intake/otlp/v1/metrics
METRICS_TEMPORALITY=delta
METRICS_HEADERS=dd-api-key=<YOUR_DD_API_KEY>
```

**Grafana Cloud** — OTLP gateway, Basic auth:
```
METRICS_EXPORTER=otlp
METRICS_PROTOCOL=http
METRICS_URL=https://otlp-gateway-<region>.grafana.net/otlp
METRICS_AUTH_KEY=Basic <base64(instanceID:token)>
```

**New Relic** — OTLP endpoint, license-key header:
```
METRICS_EXPORTER=otlp
METRICS_URL=otlp.nr-data.net:4317
METRICS_HEADERS=api-key=<YOUR_NR_LICENSE_KEY>
```

**Google Managed Prometheus (keyless)** — see `examples/using-gcp-metrics`.
