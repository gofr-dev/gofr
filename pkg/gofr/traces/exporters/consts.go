package exporters

// Shared string constants for exporter selection, used across the exporters
// package.
const (
	exporterOTLP   = "otlp"
	exporterJaeger = "jaeger"
	exporterZipkin = "zipkin"
	exporterGCP    = "gcp"
)

// gcpExporterImportPath is the blank import that registers the gcp exporter. It
// is named here rather than inlined so the hint table and the doc comment cannot
// drift apart; the tests assert the literal on purpose.
const gcpExporterImportPath = "gofr.dev/pkg/gofr/traces/exporters/gcp"
