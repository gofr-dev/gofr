package gofr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/zipkin" //nolint:staticcheck // deprecated but kept for backward compatibility
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"gofr.dev/pkg/gofr/logging"
)

func (a *App) initTracer() {
	// Install GoFr's default W3C TraceContext + Baggage propagator only if
	// the user has not already configured one (e.g. B3, Jaeger). Detect the
	// default OTel propagator by its empty Fields() — every user-configured
	// propagator advertises at least one header field.
	if existing := otel.GetTextMapPropagator(); len(existing.Fields()) == 0 {
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{},
		))
	} else {
		a.container.Logger.Warnf(
			"custom OTel TextMap propagator already installed (fields=%v); GoFr will not override it",
			existing.Fields(),
		)
	}

	otel.SetErrorHandler(&otelErrorHandler{
		logger: a.container.Logger,
	})

	traceExporter := a.Config.Get("TRACE_EXPORTER")
	tracerURL := a.Config.Get("TRACER_URL")

	// deprecated : tracer_host and tracer_port are deprecated and will be removed in upcoming versions.
	tracerHost := a.Config.Get("TRACER_HOST")
	tracerPort := a.Config.GetOrDefault("TRACER_PORT", "9411")

	if !isValidConfig(a.Logger(), traceExporter, tracerURL, tracerHost, tracerPort) {
		// No exporter configured — install a minimal SDK provider with
		// NeverSample. Spans get a valid TraceID/SpanID so X-Correlation-ID
		// and the trace_id log field stay unique per request, but the SDK
		// short-circuits at the sampler: no attributes/events stored, no
		// batch processor, no exporter. A previous attempt to install a
		// pure noop provider here zeroed out correlation IDs across every
		// request on the default (no-exporter) deployment.
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.NeverSample()),
		)
		otel.SetTracerProvider(tp)

		return
	}

	traceRatio, err := strconv.ParseFloat(a.Config.GetOrDefault("TRACER_RATIO", "1"), 64)
	if err != nil {
		a.container.Error(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(a.container.GetAppName()),
		)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(traceRatio))),
	)
	otel.SetTracerProvider(tp)

	exporter, err := a.getExporter(traceExporter, tracerHost, tracerPort, tracerURL)
	if err != nil {
		a.container.Error(err)
	}

	batcher := sdktrace.NewBatchSpanProcessor(exporter)
	tp.RegisterSpanProcessor(batcher)
}

func isValidConfig(logger logging.Logger, name, url, host, port string) bool {
	if url == "" && name == "" {
		logger.Debug("tracing is disabled, as configs are not provided")
		return false
	}

	if url != "" && name == "" {
		logger.Error("missing TRACE_EXPORTER config, should be provided with TRACER_URL to enable tracing")
		return false
	}

	//nolint:revive // early-return is not possible here, as below is the intentional logging flow
	if url == "" && name != "" && !strings.EqualFold(name, "gofr") {
		if host != "" && port != "" {
			logger.Warn("TRACER_HOST and TRACER_PORT are deprecated, use TRACER_URL instead")
		} else {
			logger.Error("missing TRACER_URL config, should be provided with TRACE_EXPORTER to enable tracing")
			return false
		}
	}

	return true
}

// parseHeaders converts comma-separated key=value pairs to headers map.
// Format follows OTEL standard: "Key1=Value1,Key2=Value2".
// Splits only on first '=' to allow '=' in values.
func parseHeaders(headerStr string) map[string]string {
	headers := make(map[string]string)

	if headerStr == "" {
		return headers
	}

	const keyValueParts = 2

	// Split by comma
	pairs := strings.Split(headerStr, ",")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)

		// Split only on first '=' to allow '=' in values
		kv := strings.SplitN(pair, "=", keyValueParts)

		if len(kv) == keyValueParts {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			if key != "" && value != "" {
				headers[key] = value
			}
		}
	}

	return headers
}

// getTracerHeaders returns headers map from TRACER_HEADERS or TRACER_AUTH_KEY config.
func (a *App) getTracerHeaders() map[string]string {
	headers := make(map[string]string)

	// Check for TRACER_HEADERS first (supports multiple custom headers)
	if headerStr := a.Config.Get("TRACER_HEADERS"); headerStr != "" {
		headers = parseHeaders(headerStr)
	} else if authKey := a.Config.Get("TRACER_AUTH_KEY"); authKey != "" {
		headers["Authorization"] = authKey
	}

	return headers
}

// tracerInsecure resolves TRACER_INSECURE, which controls transport security for
// a schemeless TRACER_URL (host:port). It reports the flag and whether it was
// explicitly configured — a scheme-bearing TRACER_URL derives its security from
// the scheme and warns when the flag was set and therefore ignored.
//
// It defaults to true (plaintext), unlike METRICS_INSECURE, which defaults to
// false. See resolveOtlpTransport for why the two diverge.
func (a *App) tracerInsecure() (insecure, set bool) {
	v := strings.TrimSpace(a.Config.Get("TRACER_INSECURE"))
	if v == "" {
		return true, false
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		// The raw value is not echoed. Tracer config reaches a log only through
		// redactURL or as a matched constant.
		a.Logger().Warn("invalid TRACER_INSECURE: expected true or false; defaulting to plaintext for a " +
			"schemeless TRACER_URL")

		return true, false
	}

	return b, true
}

func (a *App) getExporter(name, host, port, url string) (sdktrace.SpanExporter, error) {
	var (
		exporter sdktrace.SpanExporter
		err      error
	)

	headers := a.getTracerHeaders()

	switch strings.ToLower(name) {
	case otlpTraceExporter:
		exporter, err = a.buildOtlpFamilyExporter(otlpTraceExporter, url, host, port, headers)
	case jaegerTraceExporter:
		exporter, err = a.buildOtlpFamilyExporter(jaegerTraceExporter, url, host, port, headers)
	case "zipkin":
		a.Logger().Warn("TRACE_EXPORTER=zipkin is deprecated and will be removed in a future release. " +
			"Zipkin supports OTLP natively (v2.24+) — to migrate, switch to TRACE_EXPORTER=otlp " +
			"and point TRACER_URL to your Zipkin OTLP gRPC endpoint (default: <host>:4317)")

		exporter, err = buildZipkinExporter(a.Logger(), url, host, port, headers)
	case gofrTraceExporter:
		exporter = buildGoFrExporter(a.Logger(), url)
	default:
		a.container.Error("unsupported TRACE_EXPORTER: expected one of otlp, jaeger, zipkin or gofr")
	}

	return exporter, err
}

// buildOtlpFamilyExporter builds the OTLP exporter for otlp and jaeger. The
// exporter name is passed as the matched constant rather than the configured
// string, so only a known name is ever logged.
func (a *App) buildOtlpFamilyExporter(name, endpoint, host, port string, headers map[string]string) (sdktrace.SpanExporter, error) {
	// Resolved here, not in getExporter above the switch: TRACER_INSECURE means
	// nothing to zipkin or gofr, and evaluating it for them would warn about a
	// malformed value under an exporter that never reads it.
	insecure, insecureSet := a.tracerInsecure()

	return buildOtlpExporter(a.Logger(), name, endpoint, host, port, headers, insecure, insecureSet)
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
				"from the URL scheme", redactURL(url))
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
			"headers (including auth credentials) will be sent in the clear", redactURL(url))
	}

	logger.Infof("Exporting traces to %s at %s", name, redactURL(url))

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

func buildZipkinExporter(logger logging.Logger, url, host, port string, headers map[string]string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("http://%s:%s/api/v2/spans", host, port)
	}

	logger.Infof("Exporting traces to zipkin at %s", redactURL(url))

	var opts []zipkin.Option
	if len(headers) > 0 {
		opts = append(opts, zipkin.WithHeaders(headers))
	}

	return zipkin.New(url, opts...)
}

func buildGoFrExporter(logger logging.Logger, url string) sdktrace.SpanExporter {
	if url == "" {
		url = "https://tracer-api.gofr.dev/api/spans"
	}

	logger.Infof("Exporting traces to GoFr at %s", redactURL(url))

	return NewExporter(url, logging.NewLogger(logging.INFO))
}

// redactedPlaceholder stands in for credentials when a tracer endpoint is logged.
const redactedPlaceholder = "REDACTED"

// redactURL returns a tracer endpoint that is safe to write to a log.
//
// A scheme-bearing TRACER_URL is a real URL, so it can carry credentials in its
// userinfo (https://user:token@collector:4317) or its query
// (https://collector/api/v2/spans?api-key=...). Both are replaced rather than
// dropped, so the log still shows that something was configured there.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return redactUnparsedURL(raw)
	}

	if u.User != nil {
		u.User = url.User(redactedPlaceholder)
	}

	if u.RawQuery != "" {
		u.RawQuery = redactedPlaceholder
	}

	u.Fragment = ""

	return u.String()
}

// redactUnparsedURL redacts an endpoint that url.Parse cannot split into a host
// and the rest: a schemeless host:port, host/path or user:pass@host, or input
// that does not parse at all. It works on the raw string and over-redacts
// rather than risk a leak.
//
// Userinfo is handled first, up to the last '@'. A '?' in a password, or an
// '@' in a query value, then falls inside the replaced prefix instead of
// splitting the credential. The query and fragment are handled after that.
func redactUnparsedURL(raw string) string {
	redacted := raw

	if i := strings.LastIndex(redacted, "@"); i >= 0 {
		redacted = redactedPlaceholder + redacted[i:]
	}

	if i := strings.IndexByte(redacted, '#'); i >= 0 {
		redacted = redacted[:i]
	}

	if i := strings.IndexByte(redacted, '?'); i >= 0 {
		redacted = redacted[:i+1] + redactedPlaceholder
	}

	return redacted
}

type otelErrorHandler struct {
	logger logging.Logger
}

func (o *otelErrorHandler) Handle(e error) {
	if e == nil {
		return
	}

	msg := e.Error()

	// Fast check: if a message contains "status 2", it's a 2xx code.
	if strings.Contains(msg, "status 2") {
		return
	}

	o.logger.Error(msg)
}
