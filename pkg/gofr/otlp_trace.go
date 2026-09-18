//go:build !gofr_nootlp

package gofr

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
)

// The OTLP trace transport, split out of otel.go so that -tags gofr_nootlp can
// leave it out. TRACE_EXPORTER=zipkin and TRACE_EXPORTER=gofr are unaffected --
// neither goes through OTLP -- so a build with the tag can still trace.

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
// Before this change the exporter passed the raw TRACER_URL to WithEndpoint,
// which stores it verbatim as the gRPC target. A scheme-bearing value is not a
// valid target ("too many colons in address"), so the dialer never opened a
// socket and no span was ever exported — the endpoint was unusable rather than
// downgraded.
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
func resolveOtlpTransport(logger logging.Logger, url string, insecure, insecureSet bool) otlpTransport {
	const (
		schemeHTTP  = "http://"
		schemeHTTPS = "https://"
	)

	// Scheme comparison is case-insensitive per RFC 3986 §3.1, and url.Parse
	// inside WithEndpointURL treats it that way — so HTTPS://host:4317 must not
	// fall through to WithEndpoint, where it would be an invalid gRPC target.
	scheme := strings.ToLower(url)

	if strings.HasPrefix(scheme, schemeHTTP) || strings.HasPrefix(scheme, schemeHTTPS) {
		if insecureSet {
			logger.Warnf("TRACER_INSECURE is ignored for TRACER_URL=%q: transport security is derived "+
				"from the URL scheme", url)
		}

		return otlpTransport{useEndpointURL: true, plaintext: strings.HasPrefix(scheme, schemeHTTP)}
	}

	return otlpTransport{insecure: insecure, plaintext: insecure}
}

// buildOpenTelemetryProtocol using OpenTelemetryProtocol as the trace exporter
// jaeger accept OpenTelemetry Protocol (OTLP) over gRPC to upload trace data.
func buildOtlpExporter(logger logging.Logger, name, url, host, port string, headers map[string]string,
	insecure, insecureSet bool) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("%s:%s", host, port)
	}

	transport := resolveOtlpTransport(logger, url, insecure, insecureSet)

	if transport.plaintext && len(headers) > 0 {
		logger.Warnf("traces are exported to %s over plaintext with headers configured: "+
			"headers (including auth credentials) will be sent in the clear", url)
	}

	logger.Infof("Exporting traces to %s at %s", strings.ToLower(name), url)

	var opts []otlptracegrpc.Option

	if transport.useEndpointURL {
		opts = append(opts, otlptracegrpc.WithEndpointURL(url))
	} else {
		opts = append(opts, otlptracegrpc.WithEndpoint(url))

		if transport.insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
	}

	if len(headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(headers))
	}

	return otlptracegrpc.New(context.Background(), opts...)
}
