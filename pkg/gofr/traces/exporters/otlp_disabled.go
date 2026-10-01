//go:build gofr_nootlp

package exporters

import (
	"context"
	"fmt"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// This file is the gofr_nootlp half of otlp.go. Its whole purpose is that nothing here imports
// otlptracegrpc, which is what pins google.golang.org/grpc and its ~80 packages into a binary that
// exports no traces over OTLP.
//
// The names are still REGISTERED rather than left absent, and that is the point of the design.
// Build's unknown-exporter path would report TRACE_EXPORTER=otlp as a name it does not recognize,
// which reads like a typo and sends an operator looking at their spelling. Registering a builder
// that fails with the tag in the message tells them the truth: the name is right, this binary was
// built without it.

//nolint:gochecknoinits // self-registration of the built-in exporters is the intended pattern.
func init() {
	Register(exporterOTLP, omittedOtlpBuilder(exporterOTLP))
	Register(exporterJaeger, omittedOtlpBuilder(exporterJaeger))
}

// errOTLPTracesOmitted names the tag rather than the symptom, so the log line is actionable without
// knowing how GoFr is built. Jaeger shares it because jaeger and otlp are the same transport here --
// jaeger accepts OTLP over gRPC natively, and both went out with the same import.
var errOTLPTracesOmitted = fmt.Errorf(
	"this binary was built with -tags gofr_nootlp, which omits the OTLP trace exporter; " +
		"rebuild without the tag to use it, or set TRACE_EXPORTER to zipkin or gofr")

func omittedOtlpBuilder(name string) Builder {
	return func(_ context.Context, _ *Config, logger Logger) (sdktrace.SpanExporter, error) {
		// Logged as well as returned: Build's caller reports the error, but naming the configured
		// exporter here is what makes the two-line story complete for whoever reads the logs.
		logger.Errorf("TRACE_EXPORTER=%s was configured, but %v", name, errOTLPTracesOmitted)

		return nil, errOTLPTracesOmitted
	}
}

// otlpTraceLinked reports whether the OTLP trace exporter is compiled into this binary. See the
// !gofr_nootlp half in otlp.go.
const otlpTraceLinked = false
