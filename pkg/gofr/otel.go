package gofr

import (
	"context"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/traces/exporters"
	"gofr.dev/pkg/gofr/version"
)

// The gofr exporter stays in this package rather than moving to
// traces/exporters: gofr.NewExporter is exported API, and moving it would be a
// breaking change. Registering it here keeps the registry the single lookup
// path without an import cycle — traces/exporters imports nothing from pkg/gofr.
//
//nolint:gochecknoinits // self-registration mirrors the built-in exporters.
func init() {
	exporters.Register(gofrTraceExporter, buildGoFrExporter)
}

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

	traceExporter := a.Config.Get("TRACE_EXPORTER")
	tracerURL := a.Config.Get("TRACER_URL")

	// The handler is given the endpoint so it can redact it: the SDK reports a
	// failed export as `request to <TRACER_URL> failed: …`, at runtime rather
	// than at startup, and would otherwise put a credential in the log on every
	// batch a collector rejects.
	otel.SetErrorHandler(&otelErrorHandler{
		logger:   a.container.Logger,
		endpoint: tracerURL,
	})

	// deprecated : tracer_host and tracer_port are deprecated and will be removed in upcoming versions.
	tracerHost := a.Config.Get("TRACER_HOST")
	tracerPort := a.Config.GetOrDefault("TRACER_PORT", "9411")

	cfg := exporters.Config{
		AppName:    a.container.GetAppName(),
		AppVersion: version.Framework,
		Endpoint:   tracerURL,
		Host:       tracerHost,
		Port:       tracerPort,
		Headers:    a.getTracerHeaders(),
		Ratio:      a.tracerRatio(),
	}

	// An empty Exporter is what makes Build install the NeverSample provider, so
	// an invalid configuration reaches the same place as no configuration at all.
	if isValidConfig(a.Logger(), traceExporter, tracerURL, tracerHost, tracerPort) {
		cfg.Exporter = traceExporter
		cfg.Insecure, cfg.InsecureSet = a.tracerInsecure()
	}

	shutdown, tp := exporters.Build(context.Background(), &cfg, a.Logger())

	a.shutdownTracer = shutdown

	otel.SetTracerProvider(tp)
}

// shutdownTraces flushes the pending span batch and shuts the TracerProvider
// down. It is safe to call when tracing was never initialized, and idempotent
// (see exporters.Build). Without it the final batch is dropped, which is routine
// for a container scaling to zero.
func (a *App) shutdownTraces(ctx context.Context) error {
	if a.shutdownTracer == nil {
		return nil
	}

	return a.shutdownTracer(ctx)
}

// tracerRatio resolves TRACER_RATIO, the head-based sampling ratio.
//
// An unparsable value falls back to 1 (sample everything), matching the
// documented default of the config itself. The alternative — treating a typo as
// 0 — silently disables tracing on the deployment that most needs it, and the
// operator sees the same effect as a working exporter with no traffic. Over-
// sampling is visible and costs money; under-sampling is invisible. The error is
// logged naming both the rejected value and the ratio actually applied, so the
// volume jump is attributable.
func (a *App) tracerRatio() float64 {
	value := a.Config.GetOrDefault("TRACER_RATIO", "1")

	ratio, err := strconv.ParseFloat(value, 64)
	if err != nil {
		a.container.Errorf("invalid TRACER_RATIO %q: %v; falling back to 1 (sampling every trace)", value, err)

		return 1
	}

	return ratio
}

func isValidConfig(logger logging.Logger, name, endpoint, host, port string) bool {
	if endpoint == "" && name == "" {
		logger.Debug("tracing is disabled, as configs are not provided")
		return false
	}

	if endpoint != "" && name == "" {
		logger.Error("missing TRACE_EXPORTER config, should be provided with TRACER_URL to enable tracing")
		return false
	}

	if endpoint == "" && name != "" && !strings.EqualFold(name, gofrTraceExporter) && host != "" && port != "" {
		logger.Warn("TRACER_HOST and TRACER_PORT are deprecated, use TRACER_URL instead")
	}

	// Whether a missing TRACER_URL is fatal is the exporter's own question, not
	// this function's: otlp and zipkin have no sane default target and say so
	// (exporters.ErrMissingEndpoint), while gofr and gcp resolve their own. The
	// check used to live here and rejected every exporter that defaults an
	// endpoint before its builder was ever consulted.
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
// false. See exporters.resolveOtlpTransport for why the two diverge.
func (a *App) tracerInsecure() (insecure, set bool) {
	v := strings.TrimSpace(a.Config.Get("TRACER_INSECURE"))
	if v == "" {
		return true, false
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		// The raw value is not echoed. Tracer config reaches a log only through
		// exporters.RedactURL, redactExporterName or as a matched constant.
		a.Logger().Warn("invalid TRACER_INSECURE: expected true or false; defaulting to plaintext for a " +
			"schemeless TRACER_URL")

		return true, false
	}

	return b, true
}

// buildGoFrExporter ships spans to GoFr's hosted tracer. It stays here rather
// than in traces/exporters because NewExporter is exported API of this package.
func buildGoFrExporter(_ context.Context, cfg *exporters.Config, logger exporters.Logger) (sdktrace.SpanExporter, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://tracer-api.gofr.dev/api/spans"
	}

	logger.Infof("Exporting traces to GoFr at %s", exporters.RedactURL(endpoint))

	return NewExporter(endpoint, logging.NewLogger(logging.INFO)), nil
}

type otelErrorHandler struct {
	logger logging.Logger

	// endpoint is the configured TRACER_URL, kept so it can be redacted out of
	// the SDK's own messages, which quote it verbatim.
	endpoint string
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

	o.logger.Error(exporters.RedactMessage(msg, o.endpoint))
}
