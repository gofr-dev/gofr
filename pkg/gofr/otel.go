package gofr

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"gofr.dev/pkg/gofr/logging"
)

func (a *App) initTracer() {
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
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetErrorHandler(&otelErrorHandler{logger: a.container.Logger})

	traceExporter := a.Config.Get("TRACE_EXPORTER")
	tracerURL := a.Config.Get("TRACER_URL")

	// deprecated : tracer_host and tracer_port are deprecated and will be removed in upcoming versions.
	tracerHost := a.Config.Get("TRACER_HOST")
	tracerPort := a.Config.GetOrDefault("TRACER_PORT", "9411")

	if !isValidConfig(a.Logger(), traceExporter, tracerURL, tracerHost, tracerPort) {
		return
	}

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

func (a *App) getExporter(name, host, port, url string) (sdktrace.SpanExporter, error) {
	var (
		exporter sdktrace.SpanExporter
		err      error
	)

	headers := a.getTracerHeaders()

	switch strings.ToLower(name) {
	case "otlp", "jaeger":
		exporter, err = buildOtlpExporter(a.Logger(), name, url, host, port, headers)
	case "zipkin":
		exporter, err = buildZipkinExporter(a.Logger(), url, host, port, headers)
	case gofrTraceExporter:
		exporter = buildGoFrExporter(a.Logger(), url)
	default:
		a.container.Errorf("unsupported TRACE_EXPORTER: %s", name)
	}

	return exporter, err
}

// buildOpenTelemetryProtocol using OpenTelemetryProtocol as the trace exporter
// jaeger accept OpenTelemetry Protocol (OTLP) over gRPC to upload trace data.
func buildOtlpExporter(logger logging.Logger, name, url, host, port string, headers map[string]string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("%s:%s", host, port)
	}

	logger.Infof("Exporting traces to %s at %s", strings.ToLower(name), url)

	opts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(url)}

	if len(headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(headers))
	}

	return otlptracegrpc.New(context.Background(), opts...)
}

func buildZipkinExporter(logger logging.Logger, url, host, port string, headers map[string]string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("http://%s:%s/api/v2/spans", host, port)
	}

	logger.Infof("Exporting traces to zipkin at %s", url)

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

	logger.Infof("Exporting traces to GoFr at %s", url)

	return NewExporter(url, logging.NewLogger(logging.INFO))
}

type otelErrorHandler struct {
	logger logging.Logger
}

func (o *otelErrorHandler) Handle(e error) {
	o.logger.Error(e.Error())
}
