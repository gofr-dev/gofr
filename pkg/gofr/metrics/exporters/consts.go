package exporters

// Shared string constants for exporter selection, transport and temporality,
// used across the exporters package.
const (
	exporterPrometheus = "prometheus"
	exporterGCP        = "gcp"

	protocolGRPC = "grpc"
	protocolHTTP = "http"

	temporalityCumulative = "cumulative"
	temporalityDelta      = "delta"
	temporalityLowMemory  = "lowmemory"

	// otlpMetricsSignalPath is the OTLP/HTTP metrics signal path otlpmetrichttp
	// appended by default before otel v1.45; buildOTLPExporter re-appends it for a
	// path-less scheme-bearing endpoint so the bump does not silently reroute exports.
	otlpMetricsSignalPath = "/v1/metrics"
)
