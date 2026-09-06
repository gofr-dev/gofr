//go:build gofr_nootlp

package gofr

import (
	"errors"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
)

// The OTLP trace transport is omitted from this build. TRACE_EXPORTER=zipkin
// and TRACE_EXPORTER=gofr do not go through OTLP and still work, so a service
// built with the tag can still trace -- it just cannot speak OTLP.
//
// getExporter returns this error, which initTracer logs; tracing is then off
// and the rest of the app starts normally. Failing loudly here is the point:
// the alternative, quietly exporting over a different transport than
// TRACE_EXPORTER asked for, loses spans at a collector that is not listening.

var errOTLPTracesOmitted = errors.New("TRACE_EXPORTER=otlp is unavailable: this binary was built with " +
	"-tags gofr_nootlp, which omits the OTLP exporters. Use TRACE_EXPORTER=zipkin or gofr, " +
	"or rebuild without the tag")

func buildOtlpExporter(_ logging.Logger, _, _, _, _ string, _ map[string]string) (sdktrace.SpanExporter, error) {
	return nil, errOTLPTracesOmitted
}
