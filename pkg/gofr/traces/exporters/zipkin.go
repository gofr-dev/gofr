package exporters

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/zipkin" //nolint:staticcheck // deprecated but kept for backward compatibility
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

//nolint:gochecknoinits // self-registration of the built-in exporters is the intended pattern.
func init() {
	Register(exporterZipkin, buildZipkinExporter)
}

// zipkinEndpoint resolves the spans URL: TRACER_URL when set, otherwise the
// deprecated TRACER_HOST/TRACER_PORT pair on Zipkin's default path.
func zipkinEndpoint(cfg *Config) string {
	if cfg.Endpoint != "" {
		return cfg.Endpoint
	}

	return fmt.Sprintf("http://%s:%s/api/v2/spans", cfg.Host, cfg.Port)
}

func buildZipkinExporter(_ context.Context, cfg *Config, logger Logger) (sdktrace.SpanExporter, error) {
	if cfg.Endpoint == "" && (cfg.Host == "" || cfg.Port == "") {
		return nil, ErrMissingEndpoint
	}

	logger.Warnf("TRACE_EXPORTER=zipkin is deprecated and will be removed in a future release. " +
		"Zipkin supports OTLP natively (v2.24+) — to migrate, switch to TRACE_EXPORTER=otlp " +
		"and point TRACER_URL to your Zipkin OTLP gRPC endpoint (default: <host>:4317)")

	endpoint := zipkinEndpoint(cfg)

	logger.Infof("Exporting traces to %s at %s", exporterZipkin, RedactURL(endpoint))

	var opts []zipkin.Option
	if len(cfg.Headers) > 0 {
		opts = append(opts, zipkin.WithHeaders(cfg.Headers))
	}

	// zipkin.New reports an unparsable endpoint as `invalid collector URL "<raw>"`,
	// so its error needs the same redaction as the log line above.
	exporter, err := zipkin.New(endpoint, opts...)

	return exporter, redactEndpointInError(err, endpoint)
}
