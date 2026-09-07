package exporters

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Prometheus builds a MeterProvider with a Prometheus pull exporter and returns
// its Meter.
//
// Deprecated: use Build, which also returns the *MeterProvider handle required
// to flush and shut down push exporters gracefully. Retained for backward
// compatibility with external callers.
func Prometheus(appName, appVersion string) metric.Meter {
	cfg := Config{AppName: appName, AppVersion: appVersion}

	_, meter := Build(context.Background(), &cfg, noopLogger{})

	return meter
}

// noopLogger satisfies Logger for callers that do not supply one.
type noopLogger struct{}

func (noopLogger) Debug(...any)          {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Errorf(string, ...any) {}
