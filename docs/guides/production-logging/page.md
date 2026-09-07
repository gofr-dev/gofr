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

GoFr's logger writes JSON when the output is not a terminal (verified in `pkg/gofr/logging/logger.go`). The top-level envelope has:

| Field         | Source                                      |
|---------------|---------------------------------------------|
| `level`       | One of DEBUG, INFO, NOTICE, WARN, ERROR, FATAL |
| `time`        | RFC3339Nano timestamp (Go's default `time.Time` JSON marshaling, per `pkg/gofr/logging/logger.go:55`) |
| `message`     | The argument passed to the logger — a string for app logs, or a structured object for HTTP request logs |
| `trace_id`    | W3C trace ID, `omitempty` — present only when the call site supplies a trace context |
| `gofrVersion` | Framework version baked into the binary     |

For HTTP request logs (emitted by the request-logging middleware), the value of `message` is itself a structured `RequestLog` object with the fields `trace_id`, `span_id`, `start_time`, `response_time`, `method`, `user_agent`, `ip`, `uri`, and `response`. So on a request log line you will see `trace_id` both at the top level and nested inside `message` — by design: the top-level field is for log aggregators, the nested copy is part of the request record.

A typical container log stream therefore looks like:

```json
{"level":"INFO","time":"...","message":"Loaded config from file: ./configs/.env","gofrVersion":"v1.46.0"}
{"level":"INFO","time":"...","message":{"trace_id":"7ca3...","span_id":"...","method":"GET","uri":"/orders","response":200},"gofrVersion":"v1.46.0"}
```

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

Every HTTP request flows through the tracer middleware (`pkg/gofr/http/middleware/tracer.go`), which extracts the W3C TraceContext from the inbound request. When there is a trace context, the request-logging middleware records the trace ID inside the `message` object of HTTP request logs (alongside `span_id`, `method`, `uri`, etc.). The top-level `trace_id` envelope field is also populated when the call site supplies a trace context; it is omitted on log lines without one (such as startup messages).

In practice this means: pivot from a trace in Jaeger/Tempo to logs by querying for the trace ID, but configure your shipper to extract it from the nested `message` for request logs (see the Promtail snippet below) so the field is searchable regardless of which path populated it.

## Aggregation patterns

### Loki + Promtail (any Kubernetes)

Promtail tails container stdout and ships to Loki. Because GoFr already emits JSON, use Promtail's `json` pipeline stage to extract `level` and `trace_id`. Note that for HTTP request logs the `trace_id` lives inside the nested `message` object, so extract from both locations:

```yaml
pipeline_stages:
  - json:
      expressions:
        level: level
        trace_id: trace_id            # populated on lines with a top-level trace_id
        message: message
  - json:
      source: message                  # parse the nested RequestLog object when message is JSON
      expressions:
        nested_trace_id: trace_id
  - template:
      source: trace_id
      template: '{{ or .trace_id .nested_trace_id }}'
  - labels:
      level:
```

Avoid making `trace_id` a label (high cardinality). Keep it as a field and search via `|= "trace_id"` matches, or use Loki's `json` LogQL parser to filter at query time.

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
JSON, one object per line, on stdout. Top-level fields are `level`, `time`, `message`, `trace_id` (omitempty), and `gofrVersion`. For HTTP request logs `message` is a nested object containing `trace_id`, `span_id`, `method`, `uri`, `response`, etc. The TTY path produces colored text only when stdout is a terminal.
{% /faq-item %}
{% faq-item question="How do I correlate a log line with a trace?" %}
Use the W3C trace ID. It appears at the top level when the call site has a trace context, and inside the nested `message` object on every HTTP request log. Configure your shipper to extract it from both spots so the field is searchable regardless of source.
{% /faq-item %}
{% faq-item question="Can I change the log level without redeploying?" %}
Yes. Set `REMOTE_LOG_URL` to an endpoint that returns the desired level and GoFr will pick up the change at the configured fetch interval. See the Remote Log Level Change guide.
{% /faq-item %}
{% /faq %}
