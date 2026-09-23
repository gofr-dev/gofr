package exporters

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	schemeHTTP  = "http://"
	schemeHTTPS = "https://"
)

//nolint:gochecknoinits // self-registration of the built-in exporters is the intended pattern.
func init() {
	// jaeger accepts OTLP over gRPC natively (1.35+); the two names differ only
	// in how the endpoint is logged. Each builder closes over the name it was
	// registered under, so the log line names a matched constant rather than the
	// configured string — which is operator input of any shape.
	Register(exporterOTLP, otlpBuilder(exporterOTLP))
	Register(exporterJaeger, otlpBuilder(exporterJaeger))
}

// otlpTransport is the resolved transport-security decision for the OTLP trace
// exporter: which endpoint option to use, whether to apply WithInsecure, and
// whether the resulting connection carries traffic in the clear.
type otlpTransport struct {
	// useEndpointURL selects WithEndpointURL (the endpoint carries a scheme)
	// over WithEndpoint. The two are never combined.
	useEndpointURL bool

	// insecure applies WithInsecure(). Only ever true for a schemeless endpoint —
	// WithEndpointURL derives transport security from the scheme itself.
	insecure bool

	// plaintext reports whether spans will leave the process unencrypted,
	// regardless of which option expressed it.
	plaintext bool
}

// resolveOtlpTransport derives transport security from the TRACER_URL scheme
// when it has one, and from TRACER_INSECURE only when it does not.
//
// Before gofr-dev/gofr#4204 the exporter passed the raw TRACER_URL to
// WithEndpoint, which stores it verbatim as the gRPC target. A scheme-bearing
// value is not a valid target ("too many colons in address"), so the dialer
// never opened a socket and no span was ever exported — the endpoint was
// unusable rather than downgraded.
//
// WithEndpointURL is what reads the scheme, and it must never be paired with
// WithInsecure(): the SDK applies options in slice order, so a WithInsecure()
// appended afterwards would override the scheme and downgrade the connection to
// plaintext. Hence the two are mutually exclusive below.
//
// Precedence: the SDK applies OTEL_EXPORTER_OTLP_INSECURE (and
// OTEL_EXPORTER_OTLP_TRACES_INSECURE) before any explicit option, so whatever
// GoFr passes here wins over them. TRACER_URL's scheme, then TRACER_INSECURE,
// then the OTel standard variables.
//
// Deliberate divergence from the metrics exporter (metrics/exporters/otlp.go),
// which defaults a schemeless METRICS_URL to TLS: every TRACER_URL=host:4317
// deployment in the wild points at a plaintext collector today, because this
// exporter hardcoded WithInsecure(). Defaulting a schemeless endpoint to TLS
// would break all of them silently in a minor release, so traces default a
// schemeless endpoint to plaintext and TRACER_INSECURE=false is the opt-in.
func resolveOtlpTransport(cfg *Config, endpoint string, logger Logger) otlpTransport {
	// Scheme comparison is case-insensitive per RFC 3986 §3.1, and url.Parse
	// inside WithEndpointURL treats it that way — so HTTPS://host:4317 must not
	// fall through to WithEndpoint, where it would be an invalid gRPC target.
	scheme := strings.ToLower(endpoint)

	if strings.HasPrefix(scheme, schemeHTTP) || strings.HasPrefix(scheme, schemeHTTPS) {
		if cfg.InsecureSet {
			logger.Warnf("TRACER_INSECURE is ignored for TRACER_URL=%q: transport security is derived "+
				"from the URL scheme", RedactURL(endpoint))
		}

		return otlpTransport{useEndpointURL: true, plaintext: strings.HasPrefix(scheme, schemeHTTP)}
	}

	return otlpTransport{insecure: cfg.Insecure, plaintext: cfg.Insecure}
}

// otlpEndpoint resolves the OTLP target: TRACER_URL when set, otherwise the
// deprecated TRACER_HOST/TRACER_PORT pair.
func otlpEndpoint(cfg *Config) string {
	if cfg.Endpoint != "" {
		return cfg.Endpoint
	}

	return fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
}

// hasEndpoint reports whether a target was configured at all. Without one the
// exporter would dial ":4317" lazily and fail on every batch, silently.
func hasEndpoint(cfg *Config) bool {
	return cfg.Endpoint != "" || (cfg.Host != "" && cfg.Port != "")
}

// otlpBuilder returns the OTLP builder as registered under name. jaeger accepts
// the same protocol, so both names resolve to the same exporter and differ only
// in the name logged.
func otlpBuilder(name string) Builder {
	return func(ctx context.Context, cfg *Config, logger Logger) (sdktrace.SpanExporter, error) {
		return buildOtlpExporter(ctx, name, cfg, logger)
	}
}

// buildOtlpExporter exports spans over OTLP gRPC.
//
// name is the registered exporter name, passed in as the matched constant rather
// than read from cfg.Exporter: the configured string is operator input, and the
// two differ whenever TRACE_EXPORTER carries padding or mixed case.
func buildOtlpExporter(ctx context.Context, name string, cfg *Config, logger Logger) (sdktrace.SpanExporter, error) {
	if !hasEndpoint(cfg) {
		return nil, ErrMissingEndpoint
	}

	endpoint := otlpEndpoint(cfg)
	transport := resolveOtlpTransport(cfg, endpoint, logger)

	if transport.plaintext && len(cfg.Headers) > 0 {
		logger.Warnf("traces are exported to %s over plaintext with headers configured: "+
			"headers (including auth credentials) will be sent in the clear", RedactURL(endpoint))
	}

	logger.Infof("Exporting traces to %s at %s", name, RedactURL(endpoint))

	var opts []otlptracegrpc.Option

	if transport.useEndpointURL {
		opts = append(opts, otlptracegrpc.WithEndpointURL(endpoint))
	} else {
		opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))

		if transport.insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
	}

	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)

	return exporter, redactEndpointInError(err, endpoint)
}
