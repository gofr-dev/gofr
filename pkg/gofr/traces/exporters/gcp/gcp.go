// Package gcp registers a keyless OTLP trace exporter that pushes spans
// directly to Google Cloud's Telemetry (OTLP) API — the ingestion front door to
// Cloud Trace — using Application Default Credentials.
//
// Blank-import it and set TRACE_EXPORTER=gcp:
//
//	import _ "gofr.dev/pkg/gofr/traces/exporters/gcp"
//
// On Cloud Run this authenticates via the attached service account, with no key
// file and no Collector sidecar. Grant that service account
// roles/telemetry.tracesWriter on the project receiving the spans: it is the
// least-privilege role carrying telemetry.traces.write, the permission this
// endpoint checks. roles/telemetry.writer and roles/cloudtrace.agent also carry
// it, and grant more besides — verified with gcloud iam roles describe on
// 2026-09-21; the permission an endpoint checks is Google's to change, so treat
// the mapping as documentation rather than as a guarantee.
//
// With a service account attached, the quota project resolves automatically.
// User credentials do not carry one: grant roles/serviceusage.serviceUsageConsumer
// on the quota project and set GOOGLE_CLOUD_QUOTA_PROJECT.
//
// TRACER_URL is optional and defaults to telemetry.googleapis.com:443. Set it to
// a regional endpoint (telemetry.<region>.rep.googleapis.com:443) when data
// residency requires one. It must be a schemeless host:port — this destination
// is always TLS on 443, so there is no transport to select and a scheme is
// rejected at startup rather than interpreted.
package gcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	gcpdetect "go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gofr.dev/pkg/gofr/traces/exporters"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

const (
	exporterName       = "gcp"
	defaultEndpoint    = "telemetry.googleapis.com:443"
	cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

	// projectIDKey is the resource attribute Google's OTLP ingest reads to route
	// spans to a destination project.
	projectIDKey = "gcp.project_id"

	// projectEnv is the Google-standard fallback for the destination project,
	// read directly because it is a Google SDK convention rather than a GoFr
	// config (the metrics exporter reads OTEL_RESOURCE_ATTRIBUTES the same way).
	projectEnv = "GOOGLE_CLOUD_PROJECT"

	// quotaProjectHeader is resolved by Google from the service account itself and
	// documented as not settable this way.
	quotaProjectHeader = "x-goog-user-project"
)

// metadataTimeout bounds each call that may reach the GCE metadata server during
// startup: the platform detector, and the ADC lookup.
//
// 5s is the metadata client's own idea of "too slow" (its default HTTP client is
// built with Timeout: 5s over a 2s dialer, compute/metadata@v0.9.0
// metadata.go:74-83), so this neither pre-empts a healthy probe nor invents a
// budget of its own. At most two of these run before the server binds, so the
// worst case a wedged metadata server can add to boot is 2*metadataTimeout,
// against the unbounded wait it costs today.
//
// It is a var only so tests can shrink it; nothing outside this package can.
//
//nolint:gochecknoglobals // shrunk by tests so the timeout path costs milliseconds, not seconds.
var metadataTimeout = 5 * time.Second

//nolint:gochecknoinits // self-registration on blank import is the intended usage.
func init() {
	exporters.Register(exporterName, buildExporter)
	exporters.RegisterResourceDetector(exporterName, &cachingDetector{})
}

// adc resolves Application Default Credentials at most once per instance. The
// detector and the builder both need them — the detector for the destination
// project, the builder for the token source — and there is no reason to make two
// round trips to the metadata server for an answer that cannot differ.
type adc struct {
	once  sync.Once
	creds *google.Credentials
	err   error
}

func (a *adc) get(ctx context.Context) (*google.Credentials, error) {
	a.once.Do(func() {
		// The deadline is on the WAIT, and the lookup keeps an uncancellable
		// context, because oauth2/google stores the context it is handed inside
		// the Credentials it returns and reuses it for every later token refresh
		// (oauth2@v0.37.0 google/google.go:171,223). A context.WithTimeout here
		// would resolve credentials fine and then expire the refresh roughly an
		// hour into the process's life -- spans would stop exporting long after
		// anything pointed at startup.
		a.creds, a.err = awaitWithin(metadataTimeout, errMetadataTimeout,
			func() (*google.Credentials, error) {
				return google.FindDefaultCredentials(context.WithoutCancel(ctx), cloudPlatformScope)
			})
	})

	return a.creds, a.err
}

// errMetadataTimeout reports that a startup call to the GCE metadata server did
// not answer within metadataTimeout. It is a sentinel so the timeout is
// distinguishable from a credentials or detector failure.
var errMetadataTimeout = errors.New("gcp traces: the GCE metadata server did not respond before the startup deadline")

// awaitWithin runs fn and returns its result, or timeoutErr when fn has not
// finished within d.
//
// A context deadline is the obvious shape and does not work for either caller.
// The platform detector DISCARDS its context argument outright
// (contrib/detectors/gcp@v1.46.0 detector.go:35 -- the parameter is unnamed, and
// it calls the context-free metadata.OnGCE()), so a deadline on the context it
// is given bounds nothing at all. The credentials lookup does use its context,
// but keeps it (see adc.get). Bounding the wait is what is correct for both.
//
// The abandoned goroutine outlives the wait: that is the cost of calling a
// library that cannot be canceled. It is one goroutine, it ends when the
// metadata server answers or its socket dies, and the buffered channel means it
// never blocks on a receiver that has gone away.
func awaitWithin[T any](d time.Duration, timeoutErr error, fn func() (T, error)) (T, error) {
	type result struct {
		val T
		err error
	}

	ch := make(chan result, 1)

	go func() {
		val, err := fn()
		ch <- result{val: val, err: err}
	}()

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case r := <-ch:
		return r.val, r.err
	case <-timer.C:
		var zero T

		return zero, timeoutErr
	}
}

//nolint:gochecknoglobals // one ADC resolution per process, shared by the detector and the builder.
var defaultADC adc

// cachingDetector runs the GCP metadata detector at most once per process and
// adds gcp.project_id, which the platform detector does not supply.
//
// Detect is where the project attribute has to be set: Google's migration
// guidance indicates the destination project for spans is taken from the
// resource unless something else supplies it, and that behavior is not
// documented precisely enough for a plain Go gRPC exporter to rely on either
// way. Setting it from the ambient credentials is correct whether or not it is
// strictly required, which is the point — it cannot be wrong, and its absence
// could be.
type cachingDetector struct {
	once sync.Once
	res  *resource.Resource
	err  error

	// adc and platform are overridable so tests can drive project resolution and
	// the metadata probe without touching the process-wide credentials or a real
	// metadata server.
	adc      *adc
	platform resource.Detector
}

func (d *cachingDetector) platformDetector() resource.Detector {
	if d.platform != nil {
		return d.platform
	}

	return gcpdetect.NewDetector()
}

// Detect satisfies resource.Detector. Off Google Cloud the underlying detector
// fails; the error is returned so the SDK can report a partial resource, and the
// exporter still starts — a span dropped by the backend is strictly better than
// an application that will not boot.
func (d *cachingDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	d.once.Do(func() {
		// This runs before the HTTP server binds -- exporters.Build is called from
		// App.initTracer with context.Background() (pkg/gofr/otel.go:74), so there
		// is no deadline anywhere above this line. A metadata server that REFUSES
		// the connection answers instantly and costs nothing, which is why the
		// off-GCP path is fast today; one that ACCEPTS and never replies would
		// block boot for as long as it hangs, and a container that never binds its
		// port never passes its startup probe. Registering the first trace
		// detector is what puts a socket on this path at all, so the bound belongs
		// here.
		d.res, d.err = awaitWithin(metadataTimeout, errMetadataTimeout, func() (*resource.Resource, error) {
			return d.platformDetector().Detect(ctx)
		})

		if project := d.projectID(ctx); project != "" {
			d.res = withProjectID(d.res, project)
		}
	})

	return d.res, d.err
}

// projectID resolves the destination project from the ambient credentials,
// falling back to GOOGLE_CLOUD_PROJECT. An authorized_user credential carries no
// project, so the fallback is the normal path for local development rather than
// an edge case.
func (d *cachingDetector) projectID(ctx context.Context) string {
	source := d.adc
	if source == nil {
		source = &defaultADC
	}

	if creds, err := source.get(ctx); err == nil && creds.ProjectID != "" {
		return creds.ProjectID
	}

	return strings.TrimSpace(os.Getenv(projectEnv))
}

// withProjectID merges gcp.project_id into res, tolerating a nil res (which is
// what the platform detector returns off Google Cloud).
func withProjectID(res *resource.Resource, project string) *resource.Resource {
	attrs := resource.NewSchemaless(attribute.String(projectIDKey, project))

	if res == nil {
		return attrs
	}

	// Merge returns the combined set alongside a schema-conflict error, so the
	// error branch keeps that set rather than falling back to attrs alone --
	// which would drop every platform attribute the detector resolved. It is
	// unreachable at the pinned otel/sdk (Merge cannot fail when b is
	// schemaless), and this is what it should do if that ever changes.
	merged, err := resource.Merge(res, attrs)
	if err != nil && merged == nil {
		return attrs
	}

	return merged
}

// buildExporter builds an OTLP gRPC span exporter authenticated with Google
// Application Default Credentials (ADC). On Cloud Run this uses the attached
// service account via the metadata server — no key file. The token source
// refreshes automatically, which Google's direct OTLP ingest requires (~1h token
// lifetime) and which no static TRACER_HEADERS value can do.
func buildExporter(ctx context.Context, cfg *exporters.Config, logger exporters.Logger) (sdktrace.SpanExporter, error) {
	endpoint, err := resolveEndpoint(cfg)
	if err != nil {
		return nil, err
	}

	// The destination project comes off the resource, and the resource is already
	// built by the time a builder runs, so this is the last point at which an
	// operator can be told before spans quietly go to the wrong place or nowhere.
	// It follows the endpoint check so that a rejected TRACER_URL is not preceded
	// by a warning about a project that is no longer going to be used.
	warnMissingProject(cfg, logger)

	creds, err := defaultADC.get(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcp traces: resolving application default credentials: %w", err)
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTLSCredentials(credentials.NewTLS(nil)),
		otlptracegrpc.WithDialOption(
			grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: creds.TokenSource}),
		),
	}

	if len(cfg.Headers) > 0 {
		if _, ok := cfg.Headers[quotaProjectHeader]; ok {
			logger.Warnf("gcp traces: header %s is forwarded but Google does not honor it on this endpoint; "+
				"set GOOGLE_CLOUD_QUOTA_PROJECT instead, or attach a service account", quotaProjectHeader)
		}

		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		// The SDK quotes the endpoint back in its own error ("parse
		// \"dns:///<endpoint>\": …"), so redacting at the log line above is not
		// enough -- this error is logged too. %w would re-expose it through
		// Error(), which is why the message is rewritten rather than wrapped.
		//nolint:err113 // a third-party constructor's message is being redacted, not wrapped.
		return nil, errors.New(exporters.RedactMessage(
			fmt.Sprintf("gcp traces: creating OTLP exporter: %s", err), endpoint))
	}

	logger.Infof("exporting traces to Google Cloud at %s via keyless ADC", exporters.RedactURL(endpoint))

	return exporter, nil
}

// errSchemeInEndpoint reports a TRACER_URL that carries a scheme. It is a
// sentinel so the rejection is testable without matching on message text.
var errSchemeInEndpoint = errors.New("gcp traces: TRACER_URL must be a schemeless host:port")

// resolveEndpoint returns the gRPC target for the OTLP dialer: TRACER_URL when
// set, otherwise defaultEndpoint.
//
// A scheme is rejected rather than interpreted. otlptracegrpc.WithEndpoint
// stores its argument verbatim as the gRPC target, and a scheme-bearing value is
// not a valid one -- "invalid target address https://…:443, too many colons in
// address". That failure lands at export, not at startup, so the app boots
// healthy, logs that it is exporting, and drops every span; under a context
// shorter than the ~30s export timeout the operator sees only "context deadline
// exceeded", which points at the network rather than at the config.
//
// The sibling otlp builder resolves a scheme instead (resolveOtlpTransport in
// the parent package, gofr-dev/gofr#4205), because there the scheme genuinely
// selects a transport. Here it cannot: Google's OTLP ingest is always TLS on
// 443, so the only honest readings of a scheme are "redundant" and "wrong", and
// failing at startup with the fix in the message beats guessing between them.
func resolveEndpoint(cfg *exporters.Config) (string, error) {
	// Nothing upstream trims. config.Get is a bare os.Getenv
	// (pkg/gofr/config/godotenv.go:93-95), and godotenv trims an unquoted .env
	// value but not a quoted one -- while the plain environment path this
	// exporter exists for (Cloud Run, GKE, a K8s manifest with a stray space
	// after the colon) has no trimming anywhere. Untrimmed, a single space is
	// enough to make the value unusable as a gRPC target with no startup
	// diagnostic: " host:port" resolves the port as a service name, and
	// " https://host:port" walks past the scheme check below, because a space
	// fails isSchemeRune at position 0 and schemeOf then reports no scheme.
	// Either way the app boots healthy, logs that it is exporting, and drops
	// every span. Trimming is also what this file already does for the other
	// operator-supplied value, in cachingDetector.projectID.
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return defaultEndpoint, nil
	}

	if scheme := schemeOf(endpoint); scheme != "" {
		return "", fmt.Errorf("%w, not %q: %s carries a %q scheme and Google's OTLP ingest is always TLS on 443",
			errSchemeInEndpoint, scheme+"://…", exporters.RedactURL(endpoint), scheme)
	}

	return endpoint, nil
}

// schemeOf returns the lowercased URI scheme of raw, or "" when it has none.
//
// A schemeless host:port also contains a colon, so the "://" separator is what
// distinguishes the two. The prefix is then validated against RFC 3986 §3.1
// (ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )) so that a stray "://" inside some
// other malformed value is not reported as a scheme the operator never wrote.
func schemeOf(raw string) string {
	i := strings.Index(raw, "://")
	if i <= 0 {
		return ""
	}

	for j, r := range raw[:i] {
		if !isSchemeRune(r, j == 0) {
			return ""
		}
	}

	return strings.ToLower(raw[:i])
}

// isSchemeRune reports whether r is allowed at this position of a URI scheme.
// Only the first rune is restricted to ALPHA.
func isSchemeRune(r rune, first bool) bool {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
		return true
	}

	return !first && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.')
}

// warnMissingProject reports that no destination project could be resolved.
// cfg.Resource is the resource the TracerProvider will actually export, so this
// sees what the detector produced rather than guessing again.
func warnMissingProject(cfg *exporters.Config, logger exporters.Logger) {
	if resolvedProject(cfg.Resource) != "" {
		return
	}

	logger.Warnf("gcp traces: no %s could be resolved from the ambient credentials and this host is not "+
		"on Google Cloud; spans may not reach a project. Set %s=<project-id>", projectIDKey, projectEnv)
}

func resolvedProject(res *resource.Resource) string {
	if res == nil {
		return ""
	}

	for _, kv := range res.Attributes() {
		if string(kv.Key) == projectIDKey {
			return kv.Value.AsString()
		}
	}

	return ""
}
