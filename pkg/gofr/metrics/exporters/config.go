package exporters

import (
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
)

// Config holds the resolved configuration used to build the application's
// metric readers. It is populated by the container from METRICS_* configs and
// passed to Build. Custom exporters registered via Register receive the same
// Config, so they can honor the shared endpoint/interval/temporality knobs.
type Config struct {
	AppName    string
	AppVersion string

	// Exporter selects the push exporter: "" or "prometheus" (pull only),
	// "otlp", or any name registered via Register (e.g. "gcp").
	Exporter string

	// Endpoint is the collector/backend target (METRICS_URL). For gRPC it is a
	// host:port; for HTTP it may be a host:port or a full URL with scheme/path.
	Endpoint string

	// Protocol is "grpc" (default) or "http" for OTLP transport.
	Protocol string

	// Interval is the PeriodicReader push interval.
	Interval time.Duration

	// Temporality is the OTLP temporality preference: "cumulative" (default),
	// "delta", or "lowmemory".
	Temporality string

	// Headers are sent with each export request (auth, tenant routing, etc.).
	Headers map[string]string

	// Insecure disables transport security for the OTLP connection. Ignored by
	// builders that manage their own transport security (e.g. gcp).
	Insecure bool

	// CardinalityLimit overrides the per-instrument attribute-set limit applied
	// to every meter. A non-nil value calls metricSdk.WithCardinalityLimit: a
	// positive value caps distinct series per instrument per collection cycle
	// (further series collapse into a single otel.metric.overflow series), while
	// zero or negative disables the limit (unlimited). nil leaves the SDK default
	// in place (2000, or OTEL_GO_X_CARDINALITY_LIMIT). Populated from
	// METRICS_CARDINALITY_LIMIT.
	CardinalityLimit *int

	// Resource is the resource attached to every exported metric, resolved by
	// Build before any Builder runs. Builders read it to check whether the
	// attributes their backend requires were actually populated -- Google drops
	// points whose prometheus_target has no location or instance, and does so
	// server-side, so a builder that cannot see the resource cannot warn about it.
	//
	// Set by Build; ignored on input.
	Resource *resource.Resource
}

// Logger is the subset of the framework logger used by the exporters package.
// The container's logger satisfies it.
type Logger interface {
	Debug(args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}
