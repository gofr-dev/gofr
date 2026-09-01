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
// resource, whose "location" label it fills from the first of these that is
// present — and rejects the point outright when none is. "instance" has its own
// chain, ending at host.id, which the core exporters package always detects.
//
//nolint:gochecknoglobals // static lookup table.
var locationAttributes = []string{"location", "cloud.availability_zone", "cloud.region"}

//nolint:gochecknoinits // self-registration on blank import is the intended usage.
func init() {
	d := &cachingDetector{}

	exporters.Register("gcp", d.buildReader)
	exporters.RegisterResourceDetector("gcp", d)
}

// cachingDetector runs the GCP metadata detector once and keeps the result, so
// that buildReader can report on what was resolved without a second round trip
// to the metadata server. Build populates the resource before it builds the
// reader, so the cache is always warm by the time buildReader reads it.
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

// hasLocation reports whether anything the detector found, or the environment
// supplied, can populate the required prometheus_target "location" label.
//
// env is parsed as W3C Baggage-style "k=v,k=v" rather than substring-matched: a
// key such as "custom.location" ends in "location" and would otherwise be read
// as supplying one.
func hasLocation(res *resource.Resource, env string) bool {
	for _, pair := range strings.Split(env, ",") {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}

		if slices.Contains(locationAttributes, strings.TrimSpace(key)) && strings.TrimSpace(value) != "" {
			return true
		}
	}

	if res == nil {
		return false
	}

	for _, kv := range res.Attributes() {
		if slices.Contains(locationAttributes, string(kv.Key)) && kv.Value.String() != "" {
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
func (d *cachingDetector) buildReader(
	ctx context.Context, cfg *exporters.Config, logger exporters.Logger,
) (metricSdk.Reader, error) {
	if t := strings.ToLower(cfg.Temporality); t == temporalityDelta || t == temporalityLowMemory {
		logger.Warnf("METRICS_TEMPORALITY=%s is ignored by the gcp exporter; "+
			"Google Managed Prometheus requires cumulative", cfg.Temporality)
	}

	// A push that authenticates and connects but carries no location is accepted
	// by the API and then discarded per-point, server-side, with nothing on the
	// wire to notice. Saying so at startup is the only place an operator can be
	// told before the data silently goes missing.
	if res, _ := d.Detect(ctx); !hasLocation(res, os.Getenv(otelResourceAttrsEnv)) {
		logger.Warnf("gcp metrics: no %q resource attribute could be resolved and this host is not on "+
			"Google Cloud; Google's OTLP ingest rejects every point whose prometheus_target has no location. "+
			"Set OTEL_RESOURCE_ATTRIBUTES=location=<region> (e.g. us-central1)", locationAttributes[0])
	}

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
