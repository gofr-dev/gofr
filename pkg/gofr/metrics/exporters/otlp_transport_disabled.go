//go:build gofr_nootlp

package exporters

import (
	"context"
	"errors"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"
)

// The OTLP metric transports are omitted from this build. Prometheus, which is
// the default and needs no transport of its own, is unaffected; so is gcp,
// which brings its own.
//
// METRICS_EXPORTER=otlp still registers and still validates its config -- it
// fails at the point where it would open the connection, with an error naming
// the tag, rather than being quietly absent from the registry and falling
// through to a different exporter than the operator configured.
var errOTLPMetricsOmitted = errors.New("METRICS_EXPORTER=otlp is unavailable: this binary was built with " +
	"-tags gofr_nootlp, which omits the OTLP exporters. Use METRICS_EXPORTER=prometheus, or rebuild without the tag")

func buildOTLPExporter(context.Context, *Config) (metricSdk.Exporter, error) {
	return nil, errOTLPMetricsOmitted
}
