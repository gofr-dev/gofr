// Package gcp registers a keyless OTLP metrics exporter that pushes directly to
// Google Cloud's Telemetry (OTLP) API — the ingestion front door to Managed
// Service for Prometheus (GMP) — using Application Default Credentials.
//
// Blank-import it and set METRICS_EXPORTER=gcp:
//
//	import _ "gofr.dev/pkg/gofr/metrics/exporters/gcp"
//
// On Cloud Run this authenticates via the attached service account (no key
// file). Grant that service account roles/telemetry.writer on the project
// receiving the data, plus roles/serviceusage.serviceUsageConsumer on the quota
// project. roles/monitoring.metricWriter is not sufficient: it authorizes
// monitoring.googleapis.com, which this exporter never calls.
package gcp

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	gcpdetect "go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"gofr.dev/pkg/gofr/metrics/exporters"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

const (
	defaultEndpoint    = "telemetry.googleapis.com:443"
	cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

	temporalityDelta     = "delta"
	temporalityLowMemory = "lowmemory"

	// otelResourceAttrsEnv is the OpenTelemetry-standard carrier for attributes the
	// framework cannot infer; the core exporters package feeds it into the resource
	// via resource.WithFromEnv.
	otelResourceAttrsEnv = "OTEL_RESOURCE_ATTRIBUTES"
)

// Google's OTLP ingest maps every point onto the prometheus_target monitored
// resource. Both of these labels are documented "reject the point if empty", and
// each is filled from the first attribute in its list that is present, so a
// resource carrying none of them loses every point it describes.
//
//nolint:gochecknoglobals // static lookup tables.
var (
	locationAttributes = []string{"location", "cloud.availability_zone", "cloud.region"}
	instanceAttributes = []string{
		"instance", "service.instance.id", "faas.instance",
		"k8s.pod.name", "host.id",
	}
)

//nolint:gochecknoinits // self-registration on blank import is the intended usage.
func init() {
	exporters.Register("gcp", buildReader)
	exporters.RegisterResourceDetector("gcp", &cachingDetector{})
}

// cachingDetector runs the GCP metadata detector at most once per process and
// keeps the result. Build is called once per application, but a process may hold
// more than one, and there is no reason for the second to make another round trip
// to the metadata server for an answer that cannot have changed.
type cachingDetector struct {
	once sync.Once
	res  *resource.Resource
	err  error
}

// Detect satisfies resource.Detector. Off Google Cloud the underlying detector
// fails; the error is returned so the SDK can report a partial resource, and the
// exporter still starts — a metric that is dropped by the backend is strictly
// better than an application that will not boot.
func (d *cachingDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	d.once.Do(func() {
		d.res, d.err = gcpdetect.NewDetector().Detect(ctx)
	})

	return d.res, d.err
}

// resolves reports whether res, or the environment, can populate one of the
// attributes a required prometheus_target label is derived from.
//
// An attribute set to the empty string counts as absent, because that is what
// Google sees. It is a real case rather than a defensive one: the SDK's host
// detector reads /etc/machine-id and returns success on an empty file, which is
// exactly what ubuntu base images ship, so host.id arrives present and blank.
//
// env is parsed as "k=v,k=v" rather than substring-matched: a key such as
// "custom.location" ends in "location" and would otherwise be read as supplying
// one.
func resolves(res *resource.Resource, env string, names []string) bool {
	for _, pair := range strings.Split(env, ",") {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}

		if slices.Contains(names, strings.TrimSpace(key)) && strings.TrimSpace(value) != "" {
			return true
		}
	}

	if res == nil {
		return false
	}

	for _, kv := range res.Attributes() {
		if slices.Contains(names, string(kv.Key)) && kv.Value.AsString() != "" {
			return true
		}
	}

	return false
}

// buildReader builds a periodic OTLP push reader authenticated with Google
// Application Default Credentials (ADC). On Cloud Run this uses the attached
// service account via the metadata server — no key file. The token source
// refreshes automatically, which Google's direct OTLP ingest requires
// (~1h token lifetime). GMP ingests cumulative temporality, so this exporter
// pins cumulative regardless of METRICS_TEMPORALITY.
func buildReader(ctx context.Context, cfg *exporters.Config, logger exporters.Logger) (metricSdk.Reader, error) {
	if t := strings.ToLower(cfg.Temporality); t == temporalityDelta || t == temporalityLowMemory {
		logger.Warnf("METRICS_TEMPORALITY=%s is ignored by the gcp exporter; "+
			"Google Managed Prometheus requires cumulative", cfg.Temporality)
	}

	// A push that authenticates and connects but carries neither label is accepted
	// by the API and then discarded per-point, server-side, with nothing on the
	// wire to notice. Saying so at startup is the only place an operator can be
	// told before the data silently goes missing.
	warnUnresolved(cfg, logger)

	creds, err := google.FindDefaultCredentials(ctx, cloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("gcp metrics: resolving application default credentials: %w", err)
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(nil)),
		// Pin cumulative explicitly. GMP only ingests cumulative; the SDK default
		// happens to be cumulative, but setting it here makes the invariant a code
		// fact a future edit must consciously change rather than a silent default.
		otlpmetricgrpc.WithTemporalitySelector(metricSdk.DefaultTemporalitySelector),
		otlpmetricgrpc.WithDialOption(
			grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: creds.TokenSource}),
		),
	}

	if len(cfg.Headers) > 0 {
		// Google resolves the quota project from the service account itself and
		// documents that x-goog-user-project must not be supplied this way.
		if _, ok := cfg.Headers["x-goog-user-project"]; ok {
			logger.Warnf("gcp metrics: ignore-listed header x-goog-user-project is set; " +
				"set GOOGLE_CLOUD_QUOTA_PROJECT instead, or attach a service account")
		}

		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gcp metrics: creating OTLP exporter: %w", err)
	}

	logger.Infof("exporting metrics to Google Cloud at %s every %s via keyless ADC", endpoint, cfg.Interval)

	return metricSdk.NewPeriodicReader(exporter, metricSdk.WithInterval(cfg.Interval)), nil
}

// warnUnresolved reports any required prometheus_target label the resource
// cannot fill. cfg.Resource is the one the MeterProvider will actually export,
// so this sees host.id and anything else the core package detected, not just
// what this exporter's own detector found.
func warnUnresolved(cfg *exporters.Config, logger exporters.Logger) {
	env := os.Getenv(otelResourceAttrsEnv)

	if !resolves(cfg.Resource, env, locationAttributes) {
		logger.Warnf("gcp metrics: no location could be resolved and this host is not on Google Cloud; "+
			"Google's OTLP ingest rejects every point whose prometheus_target has no location. "+
			"Set %s=location=<region> (e.g. location=us-central1)", otelResourceAttrsEnv)
	}

	// Containers rarely carry a host id: the image supplies /etc/machine-id, not
	// the node, and it is absent on debian and alpine and present-but-empty on
	// ubuntu. On Kubernetes that leaves instance unfilled unless the pod name is
	// passed in, so this is the common case there rather than an edge one.
	if !resolves(cfg.Resource, env, instanceAttributes) {
		logger.Warnf("gcp metrics: no instance could be resolved; Google's OTLP ingest rejects every point "+
			"whose prometheus_target has no instance. Set %s=service.instance.id=<unique-per-process> "+
			"(on Kubernetes, the pod name via the downward API)", otelResourceAttrsEnv)
	}
}
