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
	// Resolve the resource first and publish it on cfg: pushReader runs after this,
	// and a builder needs to see the resource its backend will actually receive.
	cfg.Resource = buildResource(ctx, cfg, logger)

	opts := []metricSdk.Option{metricSdk.WithResource(cfg.Resource)}

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
		// OTEL_RESOURCE_ATTRIBUTES carries attributes a backend requires but the
		// framework cannot know -- Google fills prometheus_target.location from it,
		// and only the operator knows the region.
		//
		// metricSdk.WithResource merges resource.Environment() in as well, so this
		// is not what makes the variable work. It is here so buildResource returns a
		// complete resource on its own rather than one that is only correct once a
		// particular consumer merges it.
		resource.WithFromEnv(),
		resource.WithAttributes(attrs...),
	}

	// Attributes resolved through the GoFr config layer are applied after
	// WithFromEnv, so they win per key. WithFromEnv only sees the process
	// environment; these also carry values from configs/.env and anywhere else
	// config.Config reads from, which is where a GoFr deployment usually puts
	// them.
	if cfgAttrs := parseResourceAttributes(cfg.ResourceAttributes); len(cfgAttrs) > 0 {
		opts = append(opts, resource.WithAttributes(cfgAttrs...))
	}

	// Everything below is only meaningful to a push backend, and host.id is not
	// free: on darwin the SDK shells out to ioreg, which costs ~10ms. A
	// prometheus-only app -- the default -- must not pay that at every start for
	// a label only OTLP ingest reads.
	if name := strings.ToLower(strings.TrimSpace(cfg.Exporter)); name != "" && name != exporterPrometheus {
		// host.id is the last fallback Google accepts for the required "instance"
		// label, so detecting it keeps points from being dropped on hosts where
		// nothing else supplies one.
		opts = append(opts, resource.WithHostID())

		if d, ok := lookupDetector(name); ok {
			opts = append(opts, resource.WithDetectors(d))
		}
	}

	// resource.New returns a usable resource alongside a non-nil error for
	// partial failures (a detector that cannot reach a metadata server off-GCP,
	// say). Degrade loudly but keep whatever was resolved rather than dropping
	// the service name with it.
	res, err := resource.New(ctx, opts...)

	switch {
	case err == nil:
	// A vendor detector pinning an older semconv than the SDK's own is the normal
	// case, not a fault: contrib/detectors/gcp v1.44.0 carries semconv 1.41.0
	// while the SDK's host detector carries 1.43.0. Merge keeps every attribute
	// and only drops the schema URL, so warning here would fire on every healthy
	// boot on Google Cloud -- the one environment this exporter targets.
	case errors.Is(err, resource.ErrSchemaURLConflict):
		logger.Debug("metrics: resource schema URLs differ between detectors; " +
			"attributes are unaffected, schema URL omitted")
	default:
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
