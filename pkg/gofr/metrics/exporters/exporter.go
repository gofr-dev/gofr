package exporters

import (
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/otlptranslator"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"gofr.dev/pkg/gofr/version"
)

// DefaultCardinalityLimit is the per-instrument attribute-set limit applied to
// the metrics MeterProvider. It mirrors the OpenTelemetry SDK default (2000):
// once an instrument exceeds this many distinct attribute sets in a collection
// cycle, further series are aggregated into a single overflow series labeled
// otel.metric.overflow=true. Set METRICS_CARDINALITY_LIMIT to override it
// (0 or negative = unlimited).
const DefaultCardinalityLimit = 2000

type promConfig struct {
	cardinalityLimit int
}

// Option configures the Prometheus meter.
type Option func(*promConfig)

// WithCardinalityLimit sets the per-instrument cardinality limit. A value of 0
// or below disables the limit (unlimited cardinality).
func WithCardinalityLimit(limit int) Option {
	return func(c *promConfig) {
		c.cardinalityLimit = limit
	}
}

func Prometheus(appName, appVersion string, opts ...Option) metric.Meter {
	cfg := promConfig{cardinalityLimit: DefaultCardinalityLimit}
	for _, opt := range opts {
		opt(&cfg)
	}

	return newPrometheusMeter(appName, appVersion, cfg, promclient.DefaultRegisterer)
}

// newPrometheusMeter builds the Prometheus-backed meter against the given
// registerer. Splitting this out lets tests scrape an isolated registry to
// assert the cardinality limit is actually applied; production uses the
// default registerer via Prometheus.
func newPrometheusMeter(appName, appVersion string, cfg promConfig, registerer promclient.Registerer) metric.Meter {
	exporter, err := prometheus.New(
		prometheus.WithRegisterer(registerer),
		prometheus.WithoutTargetInfo(),
		prometheus.WithTranslationStrategy(otlptranslator.NoTranslation))
	if err != nil {
		return nil
	}

	meter := metricSdk.NewMeterProvider(
		metricSdk.WithReader(exporter),
		metricSdk.WithCardinalityLimit(cfg.cardinalityLimit),
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
