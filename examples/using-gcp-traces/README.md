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

For the same reason `TRACER_URL` must be a schemeless `host:port`: there is no
transport to select, so a `http://`/`https://` value is rejected at startup
rather than interpreted. Unlike the `otlp` exporter, where the scheme is
meaningful, here it would be silently dropped by the gRPC dialer. Surrounding
whitespace is trimmed before either check, because nothing between the
environment and this exporter trims it and a single stray space is enough to
make the value unusable as a gRPC target with no startup diagnostic.

## Startup cost

Selecting this exporter puts two calls to the GCE metadata server on the startup
path, before the HTTP server binds its port: the platform resource detector, and
the Application Default Credentials lookup. Off Google Cloud the connection is
refused instantly and neither costs anything measurable.

Each is bounded at 5 seconds, so a metadata server that accepts the connection
and never replies cannot hold the app short of serving. A container that never
binds its port never passes its startup probe, which is a worse failure than the
spans it would have exported.

On a timeout the app binds either way, and the two degrade differently:

- the **detector** timing out logs `traces: resource detection was incomplete`
  and tracing continues on a partial resource — the destination project is still
  resolvable from `GOOGLE_CLOUD_PROJECT`;
- the **credentials** lookup timing out logs `failed to initialize "gcp" trace
  exporter … tracing is disabled`, since there is no token to authenticate with.

## Deploy to Cloud Run (keyless)

1. Grant the service's runtime service account permission to write traces:

   ```bash
   gcloud projects add-iam-policy-binding <PROJECT_ID> \
     --member="serviceAccount:<RUNTIME_SA>@<PROJECT_ID>.iam.gserviceaccount.com" \
     --role="roles/telemetry.tracesWriter"
   ```

   `telemetry.googleapis.com` — what this exporter pushes to — checks the
   `telemetry.traces.write` permission. `roles/telemetry.tracesWriter` is the
   least-privilege predefined role carrying it; `roles/telemetry.writer` and
   `roles/cloudtrace.agent` carry it as well and grant more besides. Verified
   with `gcloud iam roles describe` on 2026-09-21:

   ```bash
   gcloud iam roles describe roles/telemetry.tracesWriter   # telemetry.traces.write
   gcloud iam roles describe roles/telemetry.writer         # + logging, monitoring writes
   gcloud iam roles describe roles/cloudtrace.agent         # + cloudtrace.traces.patch
   ```

   With a service account attached, the quota project is resolved automatically.
   If you authenticate with **user** credentials instead, also grant
   `roles/serviceusage.serviceUsageConsumer` on the quota project and set
   `GOOGLE_CLOUD_QUOTA_PROJECT`. Do not pass `x-goog-user-project` as a tracer
   header; Google documents that this is not the supported route, and the
   exporter warns if you do.

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
