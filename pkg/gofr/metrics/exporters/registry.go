package exporters

import (
	"context"
	"sync"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Builder constructs a metric Reader for a single push-export destination.
// Built-in exporters (otlp) and optional submodules (gcp) self-register via
// Register in an init() function; Build then selects one by name.
//
// Experimental: this is a new public extension point and its shape (returning a
// metricSdk.Reader, the Config/Logger arguments) may change in a future minor
// release as more vendor exporters land. Pin your GoFr version if you register
// custom exporters against it.
type Builder func(ctx context.Context, cfg *Config, logger Logger) (metricSdk.Reader, error)

//nolint:gochecknoglobals // package-level registry is the intended extension point, guarded by a mutex.
var (
	registryMu sync.RWMutex
	registry   = map[string]Builder{}
	detectors  = map[string]resource.Detector{}
)

// Register adds a named exporter builder. It must be called before the
// application container is created — typically from an init() function in the
// exporter's package, triggered by a blank import. Registering an existing name
// overrides it.
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
	exporterGCP: "gofr.dev/pkg/gofr/metrics/exporters/gcp",
}

// RegisterResourceDetector associates a resource.Detector with a named exporter.
// Build runs it while assembling the MeterProvider's resource, but only when that
// exporter is the selected one, so an unused detector never reaches for a
// metadata server it cannot see.
//
// This exists because some backends reject points whose resource is missing
// attributes the framework cannot supply: Google's OTLP ingest requires
// "location" and "instance" on prometheus_target and drops every point without
// them, server-side and silently. Only a vendor-specific detector can fill those
// from the environment, and it must live in that vendor's submodule -- the core
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
