//go:build !gofr_nootlp

package gofr

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
)

// The OTLP trace transport, split out of otel.go so that -tags gofr_nootlp can
// leave it out. TRACE_EXPORTER=zipkin and TRACE_EXPORTER=gofr are unaffected --
// neither goes through OTLP -- so a build with the tag can still trace.

// buildOpenTelemetryProtocol using OpenTelemetryProtocol as the trace exporter
// jaeger accept OpenTelemetry Protocol (OTLP) over gRPC to upload trace data.
func buildOtlpExporter(logger logging.Logger, name, url, host, port string, headers map[string]string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("%s:%s", host, port)
	}

	logger.Infof("Exporting traces to %s at %s", strings.ToLower(name), url)

	opts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(url)}

	if len(headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(headers))
	}

	return otlptracegrpc.New(context.Background(), opts...)
}
