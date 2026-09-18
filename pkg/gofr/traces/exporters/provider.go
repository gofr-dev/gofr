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

	name := exporterName(cfg.Exporter)
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

// exporterName normalizes a configured TRACE_EXPORTER into the key the registry
// is looked up by. Every log line that names an exporter uses this form rather
// than the raw config value: a name that matches a registered key is known to be
// safe to echo, while the raw value is operator input of any shape.
func exporterName(configured string) string {
	return strings.ToLower(strings.TrimSpace(configured))
}

// spanExporter looks up and constructs the configured exporter, returning nil
// (having logged why) when it cannot be built.
//
// name is the normalized, registry-matched form; cfg.Exporter is the raw config
// value and reaches a log only through redactExporterName.
func spanExporter(ctx context.Context, name string, cfg *Config, logger Logger) sdktrace.SpanExporter {
	build, ok := lookup(name)
	if !ok {
		if importPath, known := knownExternalExporters[name]; known {
			logger.Errorf("TRACE_EXPORTER=%q is not registered: add a blank import to enable it "+
				"(import _ %q); tracing is disabled until then", name, importPath)
		} else {
			logger.Errorf("unsupported TRACE_EXPORTER: %s; tracing is disabled", redactExporterName(cfg.Exporter))
		}

		return nil
	}

	exporter, err := build(ctx, cfg, logger)
	if err != nil {
		logger.Errorf("failed to initialize %q trace exporter: %v; tracing is disabled", name, err)
		return nil
	}

	return exporter
}

// resolveServiceName returns the service name the resource will carry. The
// environment wins: OTEL_SERVICE_NAME, or service.name inside
// OTEL_RESOURCE_ATTRIBUTES, overrides APP_NAME — the SDK already ranks those two
// against each other (OTEL_SERVICE_NAME first), so reading resource.Environment()
// inherits that precedence rather than reimplementing it. Environment() runs only
// the fromEnv detector, so it carries no unknown_service: default to mistake for
// an operator's value.
//
// An empty environment value is not an override: OTEL_RESOURCE_ATTRIBUTES=
// "service.name=" parses to a valid attribute with an empty value, and shipping
// that would leave the backend with a nameless service.
//
// Kept identical to metrics/exporters.resolveServiceName — the two must agree, or
// a service reports one name to its trace backend and another to its metric
// backend, breaking the join between them.
func resolveServiceName(appName string, logger Logger) string {
	for _, kv := range resource.Environment().Attributes() {
		// GoFr pins semconv v1.17.0 while the SDK's fromEnv detector pins a newer
		// one; ServiceNameKey is the identical attribute.Key("service.name") in
		// both, so comparing across the two versions is safe.
		if kv.Key != semconv.ServiceNameKey {
			continue
		}

		name := strings.TrimSpace(kv.Value.AsString())
		if name == "" {
			continue
		}

		if name != appName {
			logger.Infof("traces: service.name=%q from the environment overrides APP_NAME (%q)",
				name, appName)
		}

		return name
	}

	return appName
}

// buildResource assembles the resource attached to every exported span.
//
// Unlike the metrics equivalent it does not call resource.WithHostID: no trace
// backend documents a requirement for host.id, and detecting it costs ~10ms on
// darwin (the SDK shells out to ioreg) at every application start.
func buildResource(ctx context.Context, cfg *Config, logger Logger) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(resolveServiceName(cfg.AppName, logger)),
		attribute.String("framework_version", version.Framework),
	}

	opts := []resource.Option{
		// OTEL_RESOURCE_ATTRIBUTES carries attributes a backend needs but the
		// framework cannot know — the Cloud Run revision a span belongs to, the
		// deployment environment. Before this, the trace resource carried
		// service.name and nothing else, so the variable was silently ignored.
		//
		// The order here is load-bearing, not incidental: resource.New merges each
		// later option as the winner, so WithAttributes after WithFromEnv is what
		// keeps framework_version out of the environment's reach. service.name is
		// deliberately *not* defended that way — it is resolved above, where the
		// environment wins, and re-merging the same value here is a no-op.
		// Test_buildResource pins both halves, so a reshuffle of this slice fails.
		resource.WithFromEnv(),
		resource.WithAttributes(attrs...),
	}

	if d, ok := lookupDetector(exporterName(cfg.Exporter)); ok {
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
