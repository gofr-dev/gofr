// Package exporters resolves the span exporter behind TRACE_EXPORTER and
// assembles the application's TracerProvider.
//
// Built-in exporters (otlp, jaeger, zipkin) self-register here, and optional
// vendor exporters that would otherwise pull a cloud SDK into the core module
// live in their own submodules and register themselves from an init() triggered
// by a blank import:
//
//	import _ "gofr.dev/pkg/gofr/traces/exporters/gcp"
package exporters

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Builder constructs a SpanExporter for a single trace destination. Built-in
// exporters (otlp, jaeger, zipkin) and optional submodules (gcp) self-register
// via Register in an init() function; Build then selects one by name.
//
// Experimental: this is a new public extension point and its shape (returning an
// sdktrace.SpanExporter, the Config/Logger arguments) may change in a future
// minor release as more vendor exporters land. Pin your GoFr version if you
// register custom exporters against it.
type Builder func(ctx context.Context, cfg *Config, logger Logger) (sdktrace.SpanExporter, error)

//nolint:gochecknoglobals // package-level registry is the intended extension point, guarded by a mutex.
var (
	registryMu sync.RWMutex
	registry   = map[string]Builder{}
	detectors  = map[string]resource.Detector{}
)

// Register adds a named exporter builder. It must be called before the
// application is created — typically from an init() function in the exporter's
// package, triggered by a blank import. Registering an existing name overrides
// it. Build lowercases TRACE_EXPORTER before looking it up, so register
// lowercase names.
func Register(name string, b Builder) {
	registryMu.Lock()
	defer registryMu.Unlock()

	registry[name] = b
}

// knownExternalExporters maps exporter names that live in optional submodules to
// their import path, so a missing blank import yields an actionable error rather
// than a misleading "unsupported" one. Plain strings only — no dependency on the
// submodules is introduced.
//
//nolint:gochecknoglobals // static hint table.
var knownExternalExporters = map[string]string{
	exporterGCP: gcpExporterImportPath,
}

// RegisterResourceDetector associates a resource.Detector with a named exporter.
// Build runs it while assembling the TracerProvider's resource, but only when
// that exporter is the selected one, so an unused detector never reaches for a
// metadata server it cannot see.
//
// This exists because a backend may need resource attributes the framework
// cannot know: Google's OTLP trace ingest routes spans by destination project,
// which only a Google-specific detector can resolve from the ambient
// credentials. Such a detector must live in that vendor's submodule — the core
// module takes no dependency on any cloud SDK.
//
// Experimental: paired with Builder, whose shape may change in a future minor
// release.
func RegisterResourceDetector(name string, d resource.Detector) {
	registryMu.Lock()
	defer registryMu.Unlock()

	detectors[name] = d
}

func lookup(name string) (Builder, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	b, ok := registry[name]

	return b, ok
}

func lookupDetector(name string) (resource.Detector, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	d, ok := detectors[name]

	return d, ok
}
