package exporters

import (
	"context"
	"errors"
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

// ShutdownFunc flushes pending metrics and shuts down the underlying
// MeterProvider. It is safe to call more than once (later calls are no-ops).
type ShutdownFunc func(ctx context.Context) error

// Build assembles the application MeterProvider from the always-on Prometheus
// pull reader plus any push reader selected by cfg.Exporter, and returns a
// ShutdownFunc so the caller can flush and shut it down on exit.
//
// The concrete *MeterProvider (and the OTel SDK's internal ErrReaderShutdown
// sentinel) stays encapsulated here: the caller holds an opaque ShutdownFunc and
// never imports the SDK. The closure flushes pending telemetry — push exporters
// (e.g. OTLP) rely on it to emit their final window before the process exits —
// and is idempotent: MeterProvider.Shutdown is internally sync.Once guarded and
// returns ErrReaderShutdown on any call after the first (App.Shutdown is public,
// so a manual call plus the signal handler can invoke it twice), so that sentinel
// is swallowed as a no-op.
//
// Multiple readers share the same instruments with no double-counting, so the
// Prometheus /metrics endpoint and an OTLP push exporter can run together.
// Build never returns a nil ShutdownFunc; failures degrade to a working provider
// (Prometheus-only, or no readers) rather than crashing app start.
func Build(ctx context.Context, cfg *Config, logger Logger) (ShutdownFunc, metric.Meter) {
	opts := []metricSdk.Option{metricSdk.WithResource(buildResource(ctx, cfg, logger))}

	if r := prometheusReader(logger); r != nil {
		opts = append(opts, metricSdk.WithReader(r))
	}

	if r := pushReader(ctx, cfg, logger); r != nil {
		opts = append(opts, metricSdk.WithReader(r))
	}

	mp := metricSdk.NewMeterProvider(opts...)
	meter := mp.Meter(cfg.AppName, metric.WithInstrumentationVersion(cfg.AppVersion))

	shutdown := func(shutdownCtx context.Context) error {
		if err := mp.Shutdown(shutdownCtx); err != nil && !errors.Is(err, metricSdk.ErrReaderShutdown) {
			return err
		}

		return nil
	}

	return shutdown, meter
}

func buildResource(ctx context.Context, cfg *Config, logger Logger) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(cfg.AppName),
		attribute.String("framework_version", version.Framework),
	}

	opts := []resource.Option{
		// OTEL_RESOURCE_ATTRIBUTES is the standard way to supply attributes a
		// backend requires but the framework cannot know -- Google's OTLP ingest
		// rejects any point whose prometheus_target has no "location", and only
		// the operator knows the region.
		resource.WithFromEnv(),
		// host.id is the last fallback Google accepts for the required "instance"
		// label, so detecting it here is what keeps points from being dropped on
		// hosts where nothing else supplies one.
		resource.WithHostID(),
		resource.WithAttributes(attrs...),
	}

	if d, ok := lookupDetector(strings.ToLower(strings.TrimSpace(cfg.Exporter))); ok {
		opts = append(opts, resource.WithDetectors(d))
	}

	// resource.New returns a usable resource alongside a non-nil error for
	// partial failures (a detector that cannot reach a metadata server off-GCP,
	// say). Degrade loudly but keep whatever was resolved rather than dropping
	// the service name with it.
	res, err := resource.New(ctx, opts...)
	if err != nil {
		logger.Warnf("metrics: resource detection was incomplete: %v", err)
	}

	if res == nil {
		return resource.NewWithAttributes(semconv.SchemaURL, attrs...)
	}

	return res
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
		if importPath, known := knownExternalExporters[name]; known {
			logger.Errorf("METRICS_EXPORTER=%q is not registered: add a blank import to enable it "+
				"(import _ %q); using prometheus-only metrics until then", cfg.Exporter, importPath)
		} else {
			logger.Errorf("unsupported METRICS_EXPORTER %q; using prometheus-only metrics", cfg.Exporter)
		}

		return nil
	}

	r, err := build(ctx, cfg, logger)
	if err != nil {
		logger.Errorf("failed to initialize %q metrics exporter: %v; using prometheus-only metrics", cfg.Exporter, err)
		return nil
	}

	return r
}
