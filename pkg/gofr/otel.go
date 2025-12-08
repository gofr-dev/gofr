package gofr

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
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
	otel.SetErrorHandler(&otelErrorHandler{
		logger:          a.container.Logger,
		statusCodeRegex: regexp.MustCompile(`status (\d+)`),
	})

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

func (a *App) getExporter(name, host, port, url string) (sdktrace.SpanExporter, error) {
	var (
		exporter sdktrace.SpanExporter
		err      error
	)

	authHeader := a.Config.Get("TRACER_AUTH_KEY")

	switch strings.ToLower(name) {
	case "otlp", "jaeger":
		exporter, err = buildOtlpExporter(a.Logger(), name, url, host, port, authHeader)
	case "zipkin":
		exporter, err = buildZipkinExporter(a.Logger(), url, host, port, authHeader)
	case gofrTraceExporter:
		exporter = buildGoFrExporter(a.Logger(), url)
	default:
		a.container.Errorf("unsupported TRACE_EXPORTER: %s", name)
	}

	return exporter, err
}

// buildOpenTelemetryProtocol using OpenTelemetryProtocol as the trace exporter
// jaeger accept OpenTelemetry Protocol (OTLP) over gRPC to upload trace data.
func buildOtlpExporter(logger logging.Logger, name, url, host, port, authHeader string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("%s:%s", host, port)
	}

	logger.Infof("Exporting traces to %s at %s", strings.ToLower(name), url)

	opts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(url)}

	if authHeader != "" {
		opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{"Authorization": authHeader}))
	}

	return otlptracegrpc.New(context.Background(), opts...)
}

func buildZipkinExporter(logger logging.Logger, url, host, port, authHeader string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("http://%s:%s/api/v2/spans", host, port)
	}

	logger.Infof("Exporting traces to zipkin at %s", url)

	var opts []zipkin.Option
	if authHeader != "" {
		opts = append(opts, zipkin.WithHeaders(map[string]string{"Authorization": authHeader}))
	}

	return zipkin.New(url, opts...)
}

func buildGoFrExporter(logger logging.Logger, url string) sdktrace.SpanExporter {
	if url == "" {
		url = "https://tracer-api.gofr.dev/api/spans"
	}

	logger.Infof("Exporting traces to GoFr at %s", gofrTracerURL)

	return NewExporter(url, logging.NewLogger(logging.INFO))
}

type otelErrorHandler struct {
	logger          logging.Logger
	statusCodeRegex *regexp.Regexp
}

func (o *otelErrorHandler) Handle(e error) {
	if e == nil {
		return
	}

	// Try to unwrap and check for specific error types
	// (Check OpenTelemetry/Zipkin error types if they exist)

	msg := e.Error()

	// Use regex for reliable status code extraction
	matches := o.statusCodeRegex.FindStringSubmatch(msg)
	if len(matches) >= 2 {
		if code, err := strconv.Atoi(matches[1]); err == nil {
			// Ignore success codes (201 Created, 202 Accepted, 204 No Content)
			if code >= http.StatusOK && code < 300 {
				return
			}
		}
	}

	o.logger.Error(e.Error())
}
