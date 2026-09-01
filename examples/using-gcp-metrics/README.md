# using-gcp-metrics

Push GoFr metrics **directly** to Google Cloud's Telemetry (OTLP) API — the
ingestion front door to Managed Service for Prometheus (GMP) — with **no key
file**, using the workload's attached service account (Application Default
Credentials). Ideal for Cloud Run scale-to-zero, where Prometheus scraping
misses ephemeral instances.

## Enable it

One blank import registers the exporter:

```go
import _ "gofr.dev/pkg/gofr/metrics/exporters/gcp"
```

and config (see `configs/.env`):

```
METRICS_EXPORTER=gcp
METRICS_PORT=0            # push-only; drop this to also serve /metrics
# METRICS_URL defaults to telemetry.googleapis.com:443
```

The `gcp` exporter authenticates with a **refreshing** OAuth2 token from ADC
(the ~1h Google token is renewed automatically — a static `METRICS_AUTH_KEY`
would not work here) and always uses TLS. It pins **cumulative** temporality, as
GMP requires.

## Deploy to Cloud Run (keyless)

1. Grant the service's runtime service account permission to write telemetry:

   ```bash
   gcloud projects add-iam-policy-binding <PROJECT_ID> \
     --member="serviceAccount:<RUNTIME_SA>@<PROJECT_ID>.iam.gserviceaccount.com" \
     --role="roles/telemetry.writer"
   ```

   `roles/telemetry.writer` is the role for `telemetry.googleapis.com`, which is
   what this exporter pushes to. `roles/monitoring.metricWriter` grants
   `monitoring.timeSeries.create` on `monitoring.googleapis.com` — a different
   API that this exporter never calls — so granting it alone leaves the push
   unauthorized.

   With a service account attached, the quota project is resolved automatically.
   If you authenticate with **user** credentials instead, also grant
   `roles/serviceusage.serviceUsageConsumer` on the quota project and set
   `GOOGLE_CLOUD_QUOTA_PROJECT`. Do not pass `x-goog-user-project` as a metrics
   header; Google documents that this is not the supported route.

2. Deploy — no credentials mounted, no `GOOGLE_APPLICATION_CREDENTIALS`:

   ```bash
   gcloud run deploy using-gcp-metrics --source . --region <REGION>
   ```

3. On Cloud Run the app resolves ADC from the metadata server automatically.
   Metrics appear in Cloud Monitoring / Managed Service for Prometheus.

> Cross-project GMP: grant `roles/telemetry.writer` on the **destination**
> project. No Workload Identity Federation is needed on Cloud Run itself — the
> attached service account is sufficient. WIF only applies to workloads running
> outside Google Cloud.

## Required resource labels

Google maps every point onto the `prometheus_target` monitored resource, which
requires a **`location`** and an **`instance`**, and *rejects the point* when
either is empty. A push that is missing them still authenticates and still
succeeds — the points are discarded afterwards, server-side, so nothing on the
client says the data went nowhere.

On Google Cloud both are detected for you: the metadata server supplies the
region/zone, and `host.id` backs `instance`.

Anywhere else — local runs, other clouds, CI — `location` has no source, and the
exporter warns at startup. Set it explicitly:

```
OTEL_RESOURCE_ATTRIBUTES=location=us-central1
```

`location` also accepts `cloud.region` or `cloud.availability_zone`.

`instance` is filled from `host.id`, which GoFr detects automatically. Some
environments -- minimal containers and some CI runners -- have no machine ID, and
there `instance` has no source either and points are dropped for the same reason.
Supply one the same way:

```
OTEL_RESOURCE_ATTRIBUTES=location=us-central1,service.instance.id=${HOSTNAME}
```

## Local run

Locally, ADC comes from `gcloud auth application-default login`. To avoid a real
backend, prefer the collector-based OTLP setup in `examples/using-custom-metrics`
(see its "Exporting via OTLP" section) for local development, and use this example
for GCP deployment.

## Alternative: collector sidecar (fully GA)

If you prefer not to authenticate in-process, run the
[run-gmp-sidecar](https://github.com/GoogleCloudPlatform/run-gmp-sidecar) and
point plain OTLP at it — no `gcp` import needed:

```
METRICS_EXPORTER=otlp
METRICS_URL=localhost:4317
```

The sidecar handles GMP auth using the same attached service account.
