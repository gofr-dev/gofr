// Package gcp registers a keyless OTLP metrics exporter that pushes directly to
// Google Cloud's Telemetry (OTLP) API — the ingestion front door to Managed
// Service for Prometheus (GMP) — using Application Default Credentials.
//
// Blank-import it and set METRICS_EXPORTER=gcp:
//
//	import _ "gofr.dev/pkg/gofr/metrics/exporters/gcp"
//
// On Cloud Run this authenticates via the attached service account (no key
// file). Grant that service account roles/monitoring.metricWriter.
package gcp

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
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
)

//nolint:gochecknoinits // self-registration on blank import is the intended usage.
func init() {
	exporters.Register("gcp", buildReader)
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
		otlpmetricgrpc.WithDialOption(
			grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: creds.TokenSource}),
		),
	}

	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gcp metrics: creating OTLP exporter: %w", err)
	}

	logger.Infof("exporting metrics to Google Cloud at %s every %s via keyless ADC", endpoint, cfg.Interval)

	return metricSdk.NewPeriodicReader(exporter, metricSdk.WithInterval(cfg.Interval)), nil
}
