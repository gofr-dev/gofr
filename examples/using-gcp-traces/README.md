# using-gcp-traces

Export GoFr traces **directly** to Google Cloud's Telemetry (OTLP) API — the
ingestion front door to Cloud Trace — with **no key file and no Collector
sidecar**, using the workload's attached service account (Application Default
Credentials).

## Enable it

One blank import registers the exporter:

```go
import _ "gofr.dev/pkg/gofr/traces/exporters/gcp"
```

and config (see `configs/.env`):

```
TRACE_EXPORTER=gcp
TRACER_RATIO=1
# TRACER_URL defaults to telemetry.googleapis.com:443
```

The `gcp` exporter authenticates with a **refreshing** OAuth2 token from ADC.
That refresh is the reason a vendor exporter is needed at all: Google's tokens
last about an hour, so a static `TRACER_AUTH_KEY` or `TRACER_HEADERS` value
authenticates once and then stops working. TLS is always on, and
`TRACER_INSECURE` does not apply.

## Deploy to Cloud Run (keyless)

1. Grant the service's runtime service account permission to write traces:

   ```bash
   gcloud projects add-iam-policy-binding <PROJECT_ID> \
     --member="serviceAccount:<RUNTIME_SA>@<PROJECT_ID>.iam.gserviceaccount.com" \
     --role="roles/telemetry.tracesWriter"
   ```

   `roles/telemetry.tracesWriter` (or the broader `roles/telemetry.writer`) is
   the role for `telemetry.googleapis.com`, which is what this exporter pushes
   to. **`roles/cloudtrace.agent` is not sufficient** — it authorizes the older
   Cloud Trace API (`cloudtrace.googleapis.com`), a different API that this
   exporter never calls.

   With a service account attached, the quota project is resolved automatically.
   Do not pass `x-goog-user-project` as a tracer header; Google documents that
   this is not the supported route, and the exporter warns if you do.

2. Deploy — no credentials mounted, no `GOOGLE_APPLICATION_CREDENTIALS`:

   ```bash
   gcloud run deploy using-gcp-traces --source . --region <REGION>
   ```

3. Hit `/hello` and the trace appears in Cloud Trace.

> Cross-project: grant `roles/telemetry.tracesWriter` on the **destination**
> project. No Workload Identity Federation is needed on Cloud Run itself — the
> attached service account is sufficient. WIF only applies to workloads running
> outside Google Cloud.

## Destination project

Spans are routed to a project, and the exporter resolves it in this order:

1. `gcp.project_id` from the ambient credentials (`google.FindDefaultCredentials`) —
   the normal path on Cloud Run, GKE and GCE;
2. the `GOOGLE_CLOUD_PROJECT` environment variable.

The resolved value is published as the `gcp.project_id` resource attribute on
every span. If neither resolves, the exporter warns at startup rather than
failing — spans may then not reach a project.

Local runs with `gcloud auth application-default login` produce an
`authorized_user` credential, which carries no project, so set
`GOOGLE_CLOUD_PROJECT` explicitly there.

## Other resource attributes

Anything the framework cannot infer goes through the OpenTelemetry-standard
variable, which GoFr merges into the trace resource:

```
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=staging
```

On Cloud Run, GKE and GCE the platform detector fills in the service, revision
and region for you.

## Sampling

`TRACER_RATIO` is head-based and parent-respecting
(`ParentBased(TraceIDRatioBased(ratio))`), so an upstream sampling decision is
honored. Cloud Trace bills on ingested spans — lower the ratio on a busy
service.

## Alternative: Collector sidecar

If you prefer not to authenticate in-process, run an OpenTelemetry Collector
alongside the service and point plain OTLP at it — no `gcp` import needed:

```
TRACE_EXPORTER=otlp
TRACER_URL=localhost:4317
```

Google recommends the direct OTLP route used here for services that are not
already running a Collector.
