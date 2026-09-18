//go:build !gofr_nootlp

package exporters

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
)

// The OTLP wire transports, split out of otlp.go so that -tags gofr_nootlp can
// leave them out. Everything else about the exporter -- registration, config
// validation, temporality, the periodic reader -- stays shared, so the tag
// changes how metrics leave the process and nothing else.
//
// Both transports have to go together. otlpmetrichttp imports
// google.golang.org/grpc for its status codes, so keeping the HTTP one would
// keep the gRPC tree that this tag exists to drop.

func buildOTLPExporter(ctx context.Context, cfg *Config) (metricSdk.Exporter, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	hasScheme := strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://")

	if strings.EqualFold(cfg.Protocol, protocolHTTP) {
		opts := otlpOptions(cfg, endpoint, hasScheme, otlpOptionFuncs[otlpmetrichttp.Option]{
			temporality: otlpmetrichttp.WithTemporalitySelector,
			endpointURL: otlpmetrichttp.WithEndpointURL,
			endpoint:    otlpmetrichttp.WithEndpoint,
			insecure:    otlpmetrichttp.WithInsecure,
			headers:     otlpmetrichttp.WithHeaders,
		})

		return otlpmetrichttp.New(ctx, opts...)
	}

	opts := otlpOptions(cfg, endpoint, hasScheme, otlpOptionFuncs[otlpmetricgrpc.Option]{
		temporality: otlpmetricgrpc.WithTemporalitySelector,
		endpointURL: otlpmetricgrpc.WithEndpointURL,
		endpoint:    otlpmetricgrpc.WithEndpoint,
		insecure:    otlpmetricgrpc.WithInsecure,
		headers:     otlpmetricgrpc.WithHeaders,
	})

	return otlpmetricgrpc.New(ctx, opts...)
}

// otlpOptionFuncs adapts the (structurally identical, but distinctly typed)
// option constructors of otlpmetrichttp and otlpmetricgrpc to a common shape,
// so the option-selection logic in otlpOptions is written once instead of
// duplicated per protocol.
type otlpOptionFuncs[T any] struct {
	temporality func(metricSdk.TemporalitySelector) T
	endpointURL func(string) T
	endpoint    func(string) T
	insecure    func() T
	headers     func(map[string]string) T
}

// otlpOptions builds the shared set of OTLP exporter options (temporality,
// endpoint, insecure, headers) for either transport, given its option
// constructors via f.
func otlpOptions[T any](cfg *Config, endpoint string, hasScheme bool, f otlpOptionFuncs[T]) []T {
	opts := []T{f.temporality(temporalitySelector(cfg.Temporality))}

	if hasScheme {
		opts = append(opts, f.endpointURL(endpoint))
	} else {
		opts = append(opts, f.endpoint(endpoint))
	}

	// WithEndpointURL derives Insecure from the URL scheme (http vs https); an
	// unconditional WithInsecure() appended afterwards would override that and
	// silently downgrade an https:// endpoint to plaintext. Only apply the
	// METRICS_INSECURE override for schemeless host:port endpoints, where
	// there is no scheme to derive security from.
	if !hasScheme && cfg.Insecure {
		opts = append(opts, f.insecure())
	}

	if len(cfg.Headers) > 0 {
		opts = append(opts, f.headers(cfg.Headers))
	}

	return opts
}
