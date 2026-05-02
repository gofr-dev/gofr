---
description: "Production logging with GoFr: structured JSON fields, log levels via env, trace ID correlation, and aggregation with Loki, CloudWatch, or Stackdriver."
nextjs:
  metadata:
    title: "GoFr Production Logging: Structured Logs & Aggregation"
    description: "Production logging with GoFr: structured JSON fields, log levels via env, trace ID correlation, and aggregation with Loki, CloudWatch, or Stackdriver."
---

# Production Logging

{% answer %}
GoFr emits structured JSON logs to stdout when not attached to a TTY, with fields `level`, `time`, `message`, `trace_id`, and `gofrVersion`. Set the threshold via `LOG_LEVEL` (DEBUG, INFO, NOTICE, WARN, ERROR, FATAL), correlate by `trace_id`, and ship to Loki, CloudWatch, or Stackdriver from the container's stdout — no in-app shippers needed.
{% /answer %}

## The log line shape

GoFr's logger writes JSON when the output is not a terminal (verified in `pkg/gofr/logging/logger.go`). The fields are:

| Field         | Source                                      |
|---------------|---------------------------------------------|
| `level`       | One of DEBUG, INFO, NOTICE, WARN, ERROR, FATAL |
| `time`        | RFC3339 timestamp                           |
| `message`     | The argument(s) passed to the logger        |
| `trace_id`    | W3C trace ID injected by the HTTP middleware (omitted if unset) |
| `gofrVersion` | Framework version baked into the binary     |

In a TTY (local development), the output is human-readable colored text; in containers it is one JSON object per line — what every log shipper expects.

## Log levels

GoFr's `Level` type and string mapping live in `pkg/gofr/logging/level.go`. Set the level at startup with the `LOG_LEVEL` environment variable. The default is INFO. Available values:

- `DEBUG` — verbose; use during incident investigation.
- `INFO` — normal operations.
- `NOTICE` — significant non-error events.
- `WARN` — recoverable problems.
- `ERROR` — errors that need attention.
- `FATAL` — process exits.

Changing the level normally requires a redeploy. To avoid that, GoFr supports remote runtime updates — see [Remote Log Level Change](/docs/advanced-guide/remote-log-level-change). Configure `REMOTE_LOG_URL` and `REMOTE_LOG_FETCH_INTERVAL` and the level can be flipped to DEBUG mid-incident without restarting pods.

## Correlating logs with traces

Every HTTP request flows through the tracer middleware (`pkg/gofr/http/middleware/tracer.go`), which extracts the W3C TraceContext from the inbound request. GoFr's logger then attaches the resulting trace ID to each log line as the `trace_id` field. This means a single click in Jaeger or Tempo can pivot to the corresponding logs in Loki via a `{trace_id="..."}` query.

## Aggregation patterns

### Loki + Promtail (any Kubernetes)

Promtail tails container stdout and ships to Loki. Because GoFr already emits JSON, use Promtail's `json` pipeline stage to extract `level` and `trace_id` as labels.

```yaml
pipeline_stages:
  - json:
      expressions:
        level: level
        trace_id: trace_id
  - labels:
      level:
```

Avoid making `trace_id` a label (high cardinality). Keep it as a field and search via `|= "trace_id"` matches.

### CloudWatch Logs (EKS)

The CloudWatch Logs agent (Fluent Bit on Fargate, `fluentd`/`fluent-bit` on managed nodes) ships container stdout. Configure the parser as `json` so the structured fields become CloudWatch Logs Insights columns. Query with:

```text
fields @timestamp, level, trace_id, message
| filter level = "ERROR"
```

### Cloud Logging / Stackdriver (GKE)

GKE forwards container stdout automatically. Map `level` → `severity` so the Cloud Logging UI colors entries correctly. The container needs no changes; configure the agent in the cluster.

## Volume control

Logging is cheap until it is not. Practical defaults:

- Run production at `INFO`. Drop to `DEBUG` only via the remote log-level mechanism, scoped to a single service.
- Rate-limit hot paths in your own code: log a sample (1 in N) for routine 200s.
- Redact PII before it leaves the process. Do not rely on the aggregator to scrub.

## Secrets redaction

If a secret might appear in a log line (rare for GoFr's auth middleware, which uses `subtle.ConstantTimeCompare` for credentials), redact at the application level before logging. Sidecars that scrub regexes after the fact are a fallback, not a primary control.

## Multi-line tracebacks

Go's panic stack traces are multi-line. Use a parser that joins continuation lines (Promtail's `multiline` stage, Fluent Bit's `Multiline_Parser go`) so a panic shows up as a single event.

## Probe noise

`/.well-known/alive` and `/.well-known/health` get hit several times per second by the kubelet. They will dominate access logs if you log every request. Configure your platform to suppress probe spam, or use sampled logging on those routes.

{% faq %}
{% faq-item question="What format does GoFr log in production?" %}
JSON, one object per line, on stdout. Fields are `level`, `time`, `message`, `trace_id`, and `gofrVersion`. The TTY path produces colored text only when stdout is a terminal.
{% /faq-item %}
{% faq-item question="How do I correlate a log line with a trace?" %}
The `trace_id` field is the W3C trace ID. Search for it in your tracing backend or join logs and traces by that field in Grafana, Loki, or your APM.
{% /faq-item %}
{% faq-item question="Can I change the log level without redeploying?" %}
Yes. Set `REMOTE_LOG_URL` to an endpoint that returns the desired level and GoFr will pick up the change at the configured fetch interval. See the Remote Log Level Change guide.
{% /faq-item %}
{% /faq %}
