package metrics

import (
	"context"
	"sync"
)

// noopManager discards every call. It is the default global Manager so that
// code importing this package before a Container has run NewContainer (e.g.
// package init(), or a call from tests that never build a Container) does not
// need a nil check.
type noopManager struct{}

func (noopManager) NewCounter(string, string)                                      {}
func (noopManager) NewUpDownCounter(string, string)                                {}
func (noopManager) NewHistogram(string, string, ...float64)                        {}
func (noopManager) NewGauge(string, string)                                        {}
func (noopManager) IncrementCounter(context.Context, string, ...string)            {}
func (noopManager) DeltaUpDownCounter(context.Context, string, float64, ...string) {}
func (noopManager) RecordHistogram(context.Context, string, float64, ...string)    {}
func (noopManager) SetGauge(string, float64, ...string)                            {}

// Package-level default manager, guarded by a mutex; mirrors the exporters.Register/lookup
// pattern in pkg/gofr/metrics/exporters/registry.go.
//
//nolint:gochecknoglobals // intended extension point, guarded by a mutex.
var (
	globalMu      sync.RWMutex
	globalManager Manager = noopManager{}
)

// SetGlobal installs m as the package-level Manager returned by Global and used by the
// package-level NewCounter/IncrementCounter/etc. helpers below.
//
// Container calls this once, right after building its own metricsManager, so that code
// which has no Container/metricsManager instance in scope (new call sites, small helper
// packages) can still record metrics without threading a Manager through their
// constructors. Existing callers that already receive a Metrics/Manager via dependency
// injection are unaffected and should keep using that instance directly — SetGlobal only
// adds an option, it does not replace the DI pattern.
func SetGlobal(m Manager) {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalManager = m
}

// Global returns the package-level Manager installed by SetGlobal, or a no-op Manager if
// none has been installed yet.
func Global() Manager {
	globalMu.RLock()
	defer globalMu.RUnlock()

	return globalManager
}

// NewCounter registers a new counter metric on the global Manager. See SetGlobal.
func NewCounter(name, desc string) { Global().NewCounter(name, desc) }

// NewUpDownCounter registers a new up-down counter metric on the global Manager. See SetGlobal.
func NewUpDownCounter(name, desc string) { Global().NewUpDownCounter(name, desc) }

// NewHistogram registers a new histogram metric on the global Manager. See SetGlobal.
func NewHistogram(name, desc string, buckets ...float64) {
	Global().NewHistogram(name, desc, buckets...)
}

// NewGauge registers a new gauge metric on the global Manager. See SetGlobal.
func NewGauge(name, desc string) { Global().NewGauge(name, desc) }

// IncrementCounter increments a counter metric on the global Manager. See SetGlobal.
func IncrementCounter(ctx context.Context, name string, labels ...string) {
	Global().IncrementCounter(ctx, name, labels...)
}

// DeltaUpDownCounter adjusts an up-down counter metric on the global Manager. See SetGlobal.
func DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
	Global().DeltaUpDownCounter(ctx, name, value, labels...)
}

// RecordHistogram records a histogram observation on the global Manager. See SetGlobal.
func RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	Global().RecordHistogram(ctx, name, value, labels...)
}

// SetGauge sets a gauge metric on the global Manager. See SetGlobal.
func SetGauge(name string, value float64, labels ...string) {
	Global().SetGauge(name, value, labels...)
}
