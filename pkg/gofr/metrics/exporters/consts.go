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
)
