package exporters

import (
	"context"
	"strings"

	"github.com/prometheus/otlptranslator"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"gofr.dev/pkg/gofr/version"
)

// Build assembles the application MeterProvider from the always-on Prometheus
// pull reader plus any push reader selected by cfg.Exporter, and returns the
// provider handle so the caller can flush and shut it down on exit.
//
// Multiple readers share the same instruments with no double-counting, so the
// Prometheus /metrics endpoint and an OTLP push exporter can run together.
// Build never returns a nil MeterProvider; failures degrade to a working
// provider (Prometheus-only, or no readers) rather than crashing app start.
func Build(ctx context.Context, cfg *Config, logger Logger) (*metricSdk.MeterProvider, metric.Meter) {
	opts := []metricSdk.Option{metricSdk.WithResource(buildResource(cfg))}

	if r := prometheusReader(logger); r != nil {
		opts = append(opts, metricSdk.WithReader(r))
	}

	if r := pushReader(ctx, cfg, logger); r != nil {
		opts = append(opts, metricSdk.WithReader(r))
	}

	mp := metricSdk.NewMeterProvider(opts...)
	meter := mp.Meter(cfg.AppName, metric.WithInstrumentationVersion(cfg.AppVersion))

	return mp, meter
}

func buildResource(cfg *Config) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(cfg.AppName),
		attribute.String("framework_version", version.Framework),
	)
}

func prometheusReader(logger Logger) metricSdk.Reader {
	exporter, err := prometheus.New(
		prometheus.WithoutTargetInfo(),
		prometheus.WithTranslationStrategy(otlptranslator.NoTranslation))
	if err != nil {
		logger.Errorf("failed to initialize prometheus metrics exporter: %v", err)
		return nil
	}

	return exporter
}

func pushReader(ctx context.Context, cfg *Config, logger Logger) metricSdk.Reader {
	name := strings.ToLower(strings.TrimSpace(cfg.Exporter))
	if name == "" || name == exporterPrometheus {
		return nil
	}

	build, ok := lookup(name)
	if !ok {
		logger.Errorf("unsupported METRICS_EXPORTER %q; using prometheus-only metrics", cfg.Exporter)
		return nil
	}

	r, err := build(ctx, cfg, logger)
	if err != nil {
		logger.Errorf("failed to initialize %q metrics exporter: %v; using prometheus-only metrics", cfg.Exporter, err)
		return nil
	}

	return r
}
