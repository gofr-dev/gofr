package exporters

import (
	"context"
	"sync"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"
)

// Builder constructs a metric Reader for a single push-export destination.
// Built-in exporters (otlp) and optional submodules (gcp) self-register via
// Register in an init() function; Build then selects one by name.
type Builder func(ctx context.Context, cfg *Config, logger Logger) (metricSdk.Reader, error)

//nolint:gochecknoglobals // package-level registry is the intended extension point, guarded by a mutex.
var (
	registryMu sync.RWMutex
	registry   = map[string]Builder{}
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

func lookup(name string) (Builder, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	b, ok := registry[name]

	return b, ok
}
