package exporters

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/otlptranslator"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"gofr.dev/pkg/gofr/version"
)

type Logger interface {
	Infof(format string, args ...any)
}

func Prometheus(appName, appVersion string, logger Logger) (metric.Meter, func(context.Context) error, prometheus.Gatherer) {
	registry := prometheus.NewRegistry()

	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
		otelprom.WithoutTargetInfo(),
		otelprom.WithTranslationStrategy(otlptranslator.NoTranslation))
	if err != nil {
		return nil, func(context.Context) error { return nil }, nil
	}

	meter := metricSdk.NewMeterProvider(
		metricSdk.WithReader(exporter),
		metricSdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(appName),
			attribute.String("framework_version", version.Framework),
		))).Meter(appName, metric.WithInstrumentationVersion(appVersion))

	flush := func(_ context.Context) error {
		metricFamilies, err := registry.Gather()
		if err != nil {
			return err
		}

		metrics := make(map[string]any)

		for _, mf := range metricFamilies {
			metrics[mf.GetName()] = convertMetricFamilyToMap(mf)
		}

		if len(metrics) == 0 {
			return nil
		}

		b, _ := json.Marshal(metrics)
		logger.Infof("[GOFR_METRICS] %s", string(b))

		return nil
	}

	return meter, flush, registry
}

func convertMetricFamilyToMap(mf *dto.MetricFamily) []any {
	samples := make([]any, 0)

	for _, m := range mf.GetMetric() {
		sample := make(map[string]any)
		labels := make(map[string]string)

		for _, lp := range m.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}

		if len(labels) > 0 {
			sample["labels"] = labels
		}

		switch {
		case m.Gauge != nil:
			sample["value"] = m.GetGauge().GetValue()
		case m.Counter != nil:
			sample["value"] = m.GetCounter().GetValue()
		case m.Untyped != nil:
			sample["value"] = m.GetUntyped().GetValue()
		case m.Histogram != nil:
			h := m.GetHistogram()
			sample["count"] = h.GetSampleCount()
			sample["sum"] = h.GetSampleSum()

			buckets := make(map[string]uint64)
			for _, b := range h.GetBucket() {
				buckets[strconv.FormatFloat(b.GetUpperBound(), 'f', -1, 64)] = b.GetCumulativeCount()
			}

			sample["buckets"] = buckets
		}

		samples = append(samples, sample)
	}

	return samples
}

// TODO : OTLPStdOut and OTLPMetricHTTP are not being used but has to be modified such that user can decide the exporter.

// func OTLPStdOut(appName, appVersion string) metric.Meter {
// 	exporter, err := stdoutmetric.New()
// 	if err != nil {
// 		return nil
// 	}
//
// 	meter := metricSdk.NewMeterProvider(
// 		metricSdk.WithResource(resource.NewSchemaless(semconv.ServiceName(appName))),
// 		metricSdk.WithReader(metricSdk.NewPeriodicReader(exporter,
// 			metricSdk.WithInterval(3*time.Second)))).Meter(appName, metric.WithInstrumentationVersion(appVersion))
//
// 	return meter
// }
//
// func OTLPMetricHTTP(appName, appVersion string) metric.Meter {
// 	exporter, err := otlpmetrichttp.New(nil,
// 		otlpmetrichttp.WithInsecure(),
// 		otlpmetrichttp.WithURLPath("/metrics"),
// 		otlpmetrichttp.WithEndpoint("localhost:8000"))
// 	if err != nil {
// 		return nil
// 	}
//
// 	meter := metricSdk.NewMeterProvider(metricSdk.WithReader(metricSdk.NewPeriodicReader(exporter,
// 		metricSdk.WithInterval(3*time.Second)))).Meter(appName, metric.WithInstrumentationVersion(appVersion))
//
// 	return meter
// }
