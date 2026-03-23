# CMD Example

This GoFr example demonstrates a CLI application with multiple subcommands and Pushgateway-based metrics.

## Subcommands

| Command | Behavior |
|---------|----------|
| `hello` | Prints a greeting (fast success) |
| `fail` | Returns an error immediately |
| `batch -duration=3s` | Simulates a long-running batch job |
| `progress` | Simulates a ~5s job with progress output |

## Run Locally

```console
go run main.go hello
go run main.go fail
go run main.go batch -duration=3s
go run main.go progress
```

## Metrics with Pushgateway

GoFr CLI apps push metrics to a Prometheus Pushgateway on shutdown using a **read-modify-write** strategy:
counters and histograms accumulate across runs, while gauges reflect the latest value.

Set the `METRICS_PUSH_GATEWAY_URL` environment variable to enable:

```console
export METRICS_PUSH_GATEWAY_URL=http://localhost:9091
go run main.go hello
```

### Metrics Emitted

| Metric | Type | Description |
|--------|------|-------------|
| `app_cmd_success` | Counter | Successful command executions (cumulative) |
| `app_cmd_failures` | Counter | Failed command executions (cumulative) |
| `app_cmd_duration_seconds` | Histogram | Command execution duration |
| `app_cmd_last_success_timestamp` | Gauge | Unix timestamp of last success |

## Observability Stack

Use the shared [Docker setup](https://github.com/gofr-dev/gofr/tree/development/examples/http-server/docker) which includes Pushgateway, Prometheus, and Grafana
with a pre-configured [dashboard](https://github.com/gofr-dev/gofr/tree/development/examples/http-server/docker/provisioning/dashboards/gofr-dashboard):

```console
cd examples/http-server/docker
docker-compose up -d
```

This starts:
- **Grafana** at http://localhost:3000 (admin / password)
- **Prometheus** at http://localhost:9090
- **Pushgateway** at http://localhost:9091

Then run CLI commands pointing at the Pushgateway:

```console
METRICS_PUSH_GATEWAY_URL=http://localhost:9091 go run examples/sample-cmd/main.go hello
METRICS_PUSH_GATEWAY_URL=http://localhost:9091 go run examples/sample-cmd/main.go hello
METRICS_PUSH_GATEWAY_URL=http://localhost:9091 go run examples/sample-cmd/main.go fail
```

### Dashboard

Open the **GoFr - Application Services Monitoring** dashboard in Grafana and expand the **CMD Metrics** row.
It shows:
- Jobs tracked, last push age, total successes/failures, success rate
- Per-job breakdown table with duration and last success timestamp
- Duration analysis with p50/p90/p95/p99 percentiles
