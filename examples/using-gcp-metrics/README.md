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

1. Grant the service's runtime service account permission to write metrics:

   ```bash
   gcloud projects add-iam-policy-binding <PROJECT_ID> \
     --member="serviceAccount:<RUNTIME_SA>@<PROJECT_ID>.iam.gserviceaccount.com" \
     --role="roles/monitoring.metricWriter"
   ```

2. Deploy — no credentials mounted, no `GOOGLE_APPLICATION_CREDENTIALS`:

   ```bash
   gcloud run deploy using-gcp-metrics --source . --region <REGION>
   ```

3. On Cloud Run the app resolves ADC from the metadata server automatically.
   Metrics appear in Cloud Monitoring / Managed Service for Prometheus.

> Cross-project GMP: grant `roles/monitoring.metricWriter` on the **destination**
> project. No Workload Identity Federation is needed on Cloud Run itself — the
> attached service account is sufficient. WIF only applies to workloads running
> outside Google Cloud.

## Local run

Locally, ADC comes from `gcloud auth application-default login`. To avoid a real
backend, prefer the collector-based `examples/using-otlp-metrics` for local
development, and use this example for GCP deployment.

## Alternative: collector sidecar (fully GA)

If you prefer not to authenticate in-process, run the
[run-gmp-sidecar](https://github.com/GoogleCloudPlatform/run-gmp-sidecar) and
point plain OTLP at it — no `gcp` import needed:

```
METRICS_EXPORTER=otlp
METRICS_URL=localhost:4317
```

The sidecar handles GMP auth using the same attached service account.
