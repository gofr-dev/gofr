package gofr

import (
	"context"
	"strconv"
	"strings"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

type exporter struct {
	name    string
	url     string
	appName string
}

func tracerProvider(c Config, logger log.Logger) (err error) {
	appName := c.GetOrDefault("APP_NAME", "gofr")
	exporterName := strings.ToLower(c.Get("TRACER_EXPORTER"))

	e := exporter{
		name:    exporterName,
		url:     c.Get("TRACER_URL"),
		appName: appName,
	}

	var tp *trace.TracerProvider

	switch exporterName {
	case "zipkin":
		tp, err = e.getZipkinExporter(c, logger)
	case "gcp":
		tp, err = getGCPExporter(c, logger)
	default:
		return errors.Error("invalid exporter")
	}

	if err != nil {
		return
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return
}

func (e *exporter) getZipkinExporter(c Config, logger log.Logger) (*trace.TracerProvider, error) {
	var tp *trace.TracerProvider

	url := e.url + "/api/v2/spans"

	exporter, err := zipkin.New(url)
	if err != nil {
		return nil, err
	}

	batcher := trace.NewBatchSpanProcessor(exporter)

	r, err := getResource(c)
	if err != nil {
		return nil, err
	}

	isAlwaysSample, err := strconv.ParseBool(c.Get("TRACER_ALWAYS_SAMPLE"))
	if err != nil {
		logger.Warn("TRACER_ALWAYS_SAMPLE is not set.'false' will be used by default")
	}

	// if isAlwaysSample is set true for any service, it will sample all the trace
	// else it will be sampled based on parent of the span.
	if isAlwaysSample {
		tp = trace.NewTracerProvider(trace.WithSampler(trace.AlwaysSample()), trace.WithSpanProcessor(batcher), trace.WithResource(r))
	} else {
		tracerRatio, err := strconv.ParseFloat(c.Get("TRACER_RATIO"), 64)
		if err != nil {
			tracerRatio = 0.1

			logger.Warn("TRACER_RATIO is not set.'0.1' will be used by default")
		}
		tp = trace.NewTracerProvider(trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(tracerRatio))),
			trace.WithSpanProcessor(batcher), trace.WithResource(r))
	}

	return tp, nil
}

func getGCPExporter(c Config, logger log.Logger) (*trace.TracerProvider, error) {
	var tp *trace.TracerProvider

	exporter, err := cloudtrace.New(cloudtrace.WithProjectID(c.Get("GCP_PROJECT_ID")))
	if err != nil {
		return nil, err
	}

	r, err := getResource(c)
	if err != nil {
		return nil, err
	}

	isAlwaysSample, err := strconv.ParseBool(c.Get("TRACER_ALWAYS_SAMPLE"))
	if err != nil {
		logger.Warn("TRACER_ALWAYS_SAMPLE is not set.'false' will be used by default")
	}

	if isAlwaysSample {
		tp = trace.NewTracerProvider(
			trace.WithSampler(trace.AlwaysSample()),
			trace.WithBatcher(exporter),
			trace.WithResource(r))
	} else {
		tracerRatio, err := strconv.ParseFloat(c.Get("TRACER_RATIO"), 64)
		if err != nil {
			tracerRatio = 0.1

			logger.Warn("TRACER_RATIO is not set.'0.1' will be used by default")
		}
		tp = trace.NewTracerProvider(
			trace.WithSampler(trace.TraceIDRatioBased(tracerRatio)),
			trace.WithBatcher(exporter),
			trace.WithResource(r))
	}

	return tp, nil
}

func getResource(c Config) (*resource.Resource, error) {
	attributes := []attribute.KeyValue{
		attribute.String(string(semconv.TelemetrySDKLanguageKey), "go"),
		attribute.String(string(semconv.TelemetrySDKVersionKey), c.GetOrDefault("APP_VERSION", "Dev")),
		attribute.String(string(semconv.ServiceNameKey), c.GetOrDefault("APP_NAME", "Gofr-App")),
	}

	return resource.New(context.Background(), resource.WithAttributes(attributes...))
}
