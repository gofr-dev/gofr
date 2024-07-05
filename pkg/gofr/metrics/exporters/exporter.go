package exporters

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"gofr.dev/pkg/gofr/version"
)

func Prometheus(appName, appVersion string) metric.Meter {
	exporter, err := prometheus.New(prometheus.WithoutTargetInfo())
	if err != nil {
		return nil
	}

	meter := metricSdk.NewMeterProvider(
		metricSdk.WithReader(exporter),
		metricSdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(appName),
			attribute.String("framework_version", version.Framework),
		))).Meter(appName, metric.WithInstrumentationVersion(appVersion))

	return meter
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
