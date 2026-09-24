module gofr.dev/pkg/gofr/traces/exporters/gcp

go 1.26.0

// TODO(#4210): drop this replace once a gofr.dev release contains
// pkg/gofr/traces/exporters, and pin that version instead. The metrics exporter
// module went through exactly this: it shipped the same replace in 5e00b8c6d
// and dropped it in ceb503900 (#3926) once a release carried the core it needs.
// Until then this module is in-repo only: a replace in a dependency's go.mod is
// ignored by the consuming main module.
replace gofr.dev => ../../../../..

require (
	go.opentelemetry.io/contrib/detectors/gcp v1.46.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0
	go.opentelemetry.io/otel/sdk v1.46.0
	gofr.dev v1.60.1
	golang.org/x/oauth2 v0.37.0
	google.golang.org/grpc v1.83.2
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.36.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/zipkin v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260921155816-b14227669459 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260921155816-b14227669459 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
