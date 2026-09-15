package exporters

import (
	"context"
	"errors"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/version"
)

// ErrMissingEndpoint is returned by a builder that has no default target of its
// own when TRACER_URL (and the deprecated TRACER_HOST/TRACER_PORT) are unset.
// Exporters that resolve their own endpoint — gofr, gcp — never return it, which
// is why the check belongs to the builder rather than to config validation.
var ErrMissingEndpoint = errors.New(
	"missing TRACER_URL config, should be provided with TRACE_EXPORTER to enable tracing")

// ShutdownFunc flushes pending spans and shuts down the underlying
// TracerProvider. It is safe to call more than once (later calls are no-ops).
type ShutdownFunc func(ctx context.Context) error

// Build assembles the application TracerProvider from the span exporter selected
// by cfg.Exporter, and returns a ShutdownFunc so the caller can flush and shut it
// down on exit.
//
// The concrete *TracerProvider stays encapsulated here: the caller holds an
// opaque ShutdownFunc and a trace.TracerProvider interface. The closure flushes
// the BatchSpanProcessor's pending batch — without it the final spans of a
// process are simply dropped, which is routine for a container scaling to zero
// and makes a working exporter look broken.
//
// Build never returns a nil provider or a nil ShutdownFunc. Every failure —
// unknown exporter name, a builder that errors — degrades to the NeverSample
// provider described in neverSampleProvider rather than crashing app start. A
// nil logger is substituted with noopLogger, so a caller that does not want the
// diagnostics does not have to supply one.
func Build(ctx context.Context, cfg *Config, logger Logger) (ShutdownFunc, trace.TracerProvider) {
	if logger == nil {
		logger = noopLogger{}
	}

	name := strings.ToLower(strings.TrimSpace(cfg.Exporter))
	if name == "" {
		return neverSampleProvider()
	}

	// Resolve the resource first and publish it on cfg: the builder runs after
	// this, and needs to see the resource its backend will actually receive.
	cfg.Resource = buildResource(ctx, cfg, logger)

	exporter := spanExporter(ctx, name, cfg, logger)
	if exporter == nil {
		return neverSampleProvider()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(cfg.Resource),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.Ratio))),
		sdktrace.WithBatcher(exporter),
	)

	return shutdownFunc(tp), tp
}

// neverSampleProvider is the degrade path and the no-exporter default: a minimal
// SDK provider with NeverSample. Spans get a valid TraceID/SpanID so
// X-Correlation-ID and the trace_id log field stay unique per request, but the
// SDK short-circuits at the sampler: no attributes/events stored, no batch
// processor, no exporter. A previous attempt to install a pure noop provider
// here zeroed out correlation IDs across every request on the default
// (no-exporter) deployment.
func neverSampleProvider() (ShutdownFunc, trace.TracerProvider) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))

	return shutdownFunc(tp), tp
}

// shutdownFunc wraps TracerProvider.Shutdown so it is safe to call twice:
// App.Shutdown is public, so a manual call plus the signal handler can both
// reach it.
func shutdownFunc(tp *sdktrace.TracerProvider) ShutdownFunc {
	var once sync.Once

	return func(ctx context.Context) error {
		var err error

		once.Do(func() {
			err = tp.Shutdown(ctx)
		})

		return err
	}
}

// spanExporter looks up and constructs the configured exporter, returning nil
// (having logged why) when it cannot be built.
func spanExporter(ctx context.Context, name string, cfg *Config, logger Logger) sdktrace.SpanExporter {
	build, ok := lookup(name)
	if !ok {
		if importPath, known := knownExternalExporters[name]; known {
			logger.Errorf("TRACE_EXPORTER=%q is not registered: add a blank import to enable it "+
				"(import _ %q); tracing is disabled until then", cfg.Exporter, importPath)
		} else {
			logger.Errorf("unsupported TRACE_EXPORTER: %s; tracing is disabled", cfg.Exporter)
		}

		return nil
	}

	exporter, err := build(ctx, cfg, logger)
	if err != nil {
		logger.Errorf("failed to initialize %q trace exporter: %v; tracing is disabled", cfg.Exporter, err)
		return nil
	}

	return exporter
}

// buildResource assembles the resource attached to every exported span.
//
// Unlike the metrics equivalent it does not call resource.WithHostID: no trace
// backend documents a requirement for host.id, and detecting it costs ~10ms on
// darwin (the SDK shells out to ioreg) at every application start.
func buildResource(ctx context.Context, cfg *Config, logger Logger) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(cfg.AppName),
		attribute.String("framework_version", version.Framework),
	}

	opts := []resource.Option{
		// OTEL_RESOURCE_ATTRIBUTES carries attributes a backend needs but the
		// framework cannot know — the Cloud Run revision a span belongs to, the
		// deployment environment. Before this, the trace resource carried
		// service.name and nothing else, so the variable was silently ignored.
		resource.WithFromEnv(),
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

	switch {
	case err == nil:
	// A vendor detector pinning an older semconv than the SDK's own is the normal
	// case, not a fault. Merge keeps every attribute and only drops the schema
	// URL, so warning here would fire on every healthy boot on Google Cloud — the
	// one environment that detector targets.
	case errors.Is(err, resource.ErrSchemaURLConflict):
		logger.Debug("traces: resource schema URLs differ between detectors; " +
			"attributes are unaffected, schema URL omitted")
	default:
		logger.Warnf("traces: resource detection was incomplete: %v", err)
	}

	if res == nil {
		return resource.NewWithAttributes(semconv.SchemaURL, attrs...)
	}

	return res
}
