package exporters

import (
	"go.opentelemetry.io/otel/sdk/resource"
)

// Config holds the resolved configuration used to build the application's
// TracerProvider. It is populated by the framework from TRACE_EXPORTER/TRACER_*
// configs and passed to Build. Custom exporters registered via Register receive
// the same Config, so they can honor the shared endpoint/headers knobs.
type Config struct {
	// AppName is the default for the resource's service.name. The environment
	// overrides it: OTEL_SERVICE_NAME, or a non-empty service.name entry in
	// OTEL_RESOURCE_ATTRIBUTES, wins — see resolveServiceName. There is
	// deliberately no AppVersion beside it: nothing in this package reads one, and
	// the instrumentation version already travels on the resource as
	// framework_version.
	AppName string

	// Exporter selects the span exporter: "" (tracing disabled), one of the
	// built-ins ("otlp", "jaeger", "zipkin", "gofr"), or any name registered via
	// Register (e.g. "gcp").
	Exporter string

	// Endpoint is the collector/backend target (TRACER_URL). For OTLP gRPC it is
	// a host:port, optionally carrying an http:// or https:// scheme; for zipkin
	// it is the full spans URL.
	Endpoint string

	// Host and Port carry the deprecated TRACER_HOST/TRACER_PORT configs. They
	// are used only when Endpoint is empty, and each builder applies its own
	// default shape to them (otlp: host:port; zipkin: http://host:port/api/v2/spans).
	Host string
	Port string

	// Headers are sent with each export request (auth, tenant routing, etc.).
	Headers map[string]string

	// Insecure disables transport security for a schemeless Endpoint. It is
	// ignored when Endpoint carries a scheme, which selects the transport itself,
	// and by builders that manage their own transport security (e.g. gcp).
	//
	// Unlike METRICS_INSECURE, TRACER_INSECURE defaults to true. See
	// resolveOtlpTransport for why.
	Insecure bool

	// InsecureSet reports whether Insecure was explicitly configured, so a value
	// that the endpoint's scheme overrides can be warned about rather than
	// ignored in silence.
	InsecureSet bool

	// Ratio is the head-based sampling ratio (TRACER_RATIO) applied as
	// ParentBased(TraceIDRatioBased(Ratio)).
	Ratio float64

	// Resource is the resource attached to every exported span, resolved by Build
	// before any Builder runs. Builders read it to check whether the attributes
	// their backend requires were actually populated — a builder that cannot see
	// the resource cannot warn about what its backend will reject.
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

// noopLogger satisfies Logger for callers that do not supply one.
type noopLogger struct{}

func (noopLogger) Debug(...any)          {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Errorf(string, ...any) {}
