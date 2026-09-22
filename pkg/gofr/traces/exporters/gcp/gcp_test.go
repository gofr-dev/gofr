package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gofr.dev/pkg/gofr/traces/exporters"
	"golang.org/x/oauth2/google"
)

type testLogger struct {
	warnings []string
	infos    []string
}

func (*testLogger) Debug(...any) {}
func (l *testLogger) Infof(format string, args ...any) {
	l.infos = append(l.infos, fmt.Sprintf(format, args...))
}
func (l *testLogger) Warnf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}
func (*testLogger) Errorf(string, ...any) {}

func (l *testLogger) warnedAbout(substr string) bool {
	return containsSubstr(l.warnings, substr)
}

func (l *testLogger) infoedAbout(substr string) bool {
	return containsSubstr(l.infos, substr)
}

func containsSubstr(lines []string, substr string) bool {
	for _, line := range lines {
		if strings.Contains(line, substr) {
			return true
		}
	}

	return false
}

// writeADC points GOOGLE_APPLICATION_CREDENTIALS at a well-formed
// authorized_user credentials file so FindDefaultCredentials resolves without
// any network call. That credential type carries no project, which is the local
// development case the GOOGLE_CLOUD_PROJECT fallback exists for; a credential
// that does carry one is simulated with resolvedADC.
func writeADC(t *testing.T) {
	t.Helper()

	creds := map[string]string{
		"type":          "authorized_user",
		"client_id":     "test-id",
		"client_secret": "test-secret",
		"refresh_token": "test-token",
	}

	b, err := json.Marshal(creds)
	if err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", p)
}

// resolvedADC returns an adc that is already resolved to the given credentials,
// standing in for a service account (whose credential does carry a project)
// without needing a parseable RSA private key in a fixture.
func resolvedADC(project string) *adc {
	a := &adc{creds: &google.Credentials{ProjectID: project}}
	a.once.Do(func() {})

	return a
}

func Test_resolveEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
		wantErr  bool
	}{
		{name: "unset falls back to the default", endpoint: "", want: defaultEndpoint},
		{
			name:     "regional endpoint is used verbatim",
			endpoint: "telemetry.europe-west1.rep.googleapis.com:443",
			want:     "telemetry.europe-west1.rep.googleapis.com:443",
		},
		{name: "host:port is not a scheme", endpoint: "telemetry.googleapis.com:443", want: "telemetry.googleapis.com:443"},
		{name: "https is rejected", endpoint: "https://telemetry.googleapis.com:443", wantErr: true},
		{name: "http is rejected", endpoint: "http://telemetry.googleapis.com:443", wantErr: true},
		{name: "scheme match is case-insensitive", endpoint: "HTTPS://telemetry.googleapis.com:443", wantErr: true},
		{name: "any other scheme is rejected too", endpoint: "grpc://telemetry.googleapis.com:443", wantErr: true},
		// RFC 3986 §3.1 allows digits, '+', '-' and '.' after the first ALPHA.
		{name: "scheme with the full RFC 3986 charset", endpoint: "g2rpc+w-x.y://telemetry.googleapis.com:443", wantErr: true},
		// "://" alone is not a scheme: the prefix has to be a well-formed one, or
		// a malformed endpoint gets an error naming a scheme nobody wrote.
		{
			name:     "leading digit is not a scheme",
			endpoint: "2grpc://telemetry.googleapis.com:443",
			want:     "2grpc://telemetry.googleapis.com:443",
		},
		{
			name:     "a separator inside the prefix is not a scheme",
			endpoint: "host/path://telemetry.googleapis.com:443",
			want:     "host/path://telemetry.googleapis.com:443",
		},
		{name: "a leading :// is not a scheme", endpoint: "://telemetry.googleapis.com:443", want: "://telemetry.googleapis.com:443"},
		// Surrounding whitespace survives every path that reaches cfg.Endpoint, and
		// untrimmed it defeats both halves of this function: a leading space makes
		// schemeOf report no scheme, so a scheme-bearing value is accepted; a
		// trailing one leaves a target whose port cannot be parsed. Neither fails
		// until export, ~30s after the app has logged that it is exporting.
		{
			name:     "a padded scheme-bearing value is still rejected",
			endpoint: " https://telemetry.googleapis.com:443 ",
			wantErr:  true,
		},
		{name: "a padded tab-and-newline scheme is still rejected", endpoint: "\t\nhttps://telemetry.googleapis.com:443", wantErr: true},
		{
			name:     "a padded schemeless value resolves to its trimmed form",
			endpoint: "  telemetry.googleapis.com:443  ",
			want:     "telemetry.googleapis.com:443",
		},
		{name: "whitespace only falls back to the default", endpoint: "   ", want: defaultEndpoint},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveEndpoint(&exporters.Config{Endpoint: tt.endpoint})

			if tt.wantErr {
				if !errors.Is(err, errSchemeInEndpoint) {
					t.Fatalf("expected errSchemeInEndpoint, got %v", err)
				}

				if got != "" {
					t.Errorf("expected no endpoint alongside the error, got %q", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("endpoint = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test_resolveEndpoint_errorRedactsCredentials guards the same rule as the
// startup log line: the rejected value is operator input, the registry writes
// the returned error to a log, and TRACER_URL routinely carries a credential.
func Test_resolveEndpoint_errorRedactsCredentials(t *testing.T) {
	const secret = "s3cr3t"

	_, err := resolveEndpoint(&exporters.Config{Endpoint: "https://svc:" + secret + "@telemetry.googleapis.com:443"})
	if err == nil {
		t.Fatal("expected an error")
	}

	if strings.Contains(err.Error(), secret) {
		t.Errorf("error leaked the credential: %s", err)
	}
}

func Test_buildExporter_endpoint(t *testing.T) {
	writeADC(t)

	tests := []struct {
		name string
		cfg  exporters.Config
		want string
	}{
		{name: "default endpoint", cfg: exporters.Config{}, want: defaultEndpoint},
		{
			name: "TRACER_URL overrides",
			cfg:  exporters.Config{Endpoint: "telemetry.europe-west1.rep.googleapis.com:443"},
			want: "telemetry.europe-west1.rep.googleapis.com:443",
		},
		{
			name: "headers are forwarded",
			cfg:  exporters.Config{Headers: map[string]string{"X-Tenant": "a"}},
			want: defaultEndpoint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.Resource = resource.NewSchemaless(attribute.String(projectIDKey, "p"))
			logger := &testLogger{}

			exp, err := buildExporter(t.Context(), &tt.cfg, logger)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if exp == nil {
				t.Fatal("expected a non-nil exporter")
			}

			// otlptracegrpc keeps the target in an unexported field, so the startup
			// log is the observable that binds it. Without an assertion here the
			// table covers buildExporter's lines without constraining the endpoint
			// at all -- discarding cfg.Endpoint entirely still passes.
			if !logger.infoedAbout(tt.want) {
				t.Errorf("expected the startup log to name %q, got %v", tt.want, logger.infos)
			}

			_ = exp.Shutdown(t.Context())
		})
	}
}

// Test_buildExporter_dialsTheResolvedEndpoint is the assertion that constrains
// the endpoint rather than merely covering the line. otlptracegrpc keeps the
// target in an unexported field and the startup log reads the same variable, so
// neither can tell that WithEndpoint received the resolved value -- discarding
// cfg.Endpoint inside the option call survives both.
//
// A listener on an ephemeral port is the one observable that cannot: a
// connection arriving there proves the dialer was given this address. The TLS
// handshake then fails against a bare TCP socket, which is fine -- the socket is
// the assertion, not the export.
func Test_buildExporter_dialsTheResolvedEndpoint(t *testing.T) {
	// The padded case is the one a log line cannot catch: untrimmed, a trailing
	// space leaves a target whose port parses as a service name, so the app logs
	// that it is exporting and no connection is ever made. Only the socket
	// distinguishes the two.
	for _, pad := range []struct{ name, prefix, suffix string }{
		{name: "bare"},
		{name: "padded", prefix: " ", suffix: " "},
	} {
		t.Run(pad.name, func(t *testing.T) {
			assertDialsEndpoint(t, pad.prefix, pad.suffix)
		})
	}
}

func assertDialsEndpoint(t *testing.T, prefix, suffix string) {
	t.Helper()
	writeADC(t)

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	defer ln.Close()

	dialed := make(chan struct{}, 1)

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}

		dialed <- struct{}{}

		conn.Close()
	}()

	cfg := exporters.Config{
		Endpoint: prefix + ln.Addr().String() + suffix,
		Resource: resource.NewSchemaless(attribute.String(projectIDKey, "p")),
	}

	exp, err := buildExporter(t.Context(), &cfg, &testLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = exp.Shutdown(context.Background()) }()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	// The export is expected to fail against a bare socket ("authentication
	// handshake failed: EOF"); it exists only to force the lazy gRPC dial. Its
	// deadline is spent by the time it returns, so the wait below needs a timer
	// of its own rather than this context.
	_ = exp.ExportSpans(ctx, tracetest.SpanStubs{{Name: "probe"}}.Snapshots())

	select {
	case <-dialed:
	case <-time.After(time.Second):
		t.Fatalf("no connection reached %s; the resolved endpoint was not the dial target", ln.Addr())
	}
}

// Test_buildExporter_rejectsSchemeBearingEndpoint pins the failure to startup.
// otlptracegrpc.WithEndpoint stores a scheme-bearing value verbatim as the gRPC
// target, which is invalid ("too many colons in address"), so the app boots
// healthy and silently exports nothing until the ~30s export timeout elapses.
func Test_buildExporter_rejectsSchemeBearingEndpoint(t *testing.T) {
	writeADC(t)

	cfg := exporters.Config{
		Endpoint: "https://telemetry.googleapis.com:443",
		Resource: resource.NewSchemaless(attribute.String(projectIDKey, "p")),
	}

	exp, err := buildExporter(t.Context(), &cfg, &testLogger{})
	if !errors.Is(err, errSchemeInEndpoint) {
		t.Fatalf("expected errSchemeInEndpoint, got %v", err)
	}

	if exp != nil {
		t.Error("expected no exporter when the endpoint is rejected")
	}
}

// Test_buildExporter_redactsEndpointInLog guards the rule RedactURL's own doc
// comment states for a Builder registered from outside that package: TRACER_URL
// is operator input and routinely carries a credential.
func Test_buildExporter_redactsEndpointInLog(t *testing.T) {
	writeADC(t)

	tests := []struct {
		name     string
		endpoint string
		unwanted string
		want     string
	}{
		{
			name:     "userinfo",
			endpoint: "svc:s3cr3t@telemetry.googleapis.com:443",
			unwanted: "s3cr3t",
			want:     "REDACTED@telemetry.googleapis.com:443",
		},
		{
			name:     "query",
			endpoint: "telemetry.googleapis.com:443?api-key=AIzaLIVEKEY",
			unwanted: "AIzaLIVEKEY",
			want:     "telemetry.googleapis.com:443?REDACTED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := exporters.Config{
				Endpoint: tt.endpoint,
				Resource: resource.NewSchemaless(attribute.String(projectIDKey, "p")),
			}
			logger := &testLogger{}

			exp, err := buildExporter(t.Context(), &cfg, logger)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			defer func() { _ = exp.Shutdown(t.Context()) }()

			if !logger.infoedAbout(tt.want) {
				t.Errorf("expected the log to contain %q, got %v", tt.want, logger.infos)
			}

			for _, line := range logger.infos {
				if strings.Contains(line, tt.unwanted) {
					t.Errorf("log line leaked %q: %s", tt.unwanted, line)
				}
			}
		})
	}
}

// Test_buildExporter_redactsEndpointInError covers the endpoint the SDK quotes
// back in its own error. otlptracegrpc.New prepends "dns:///" and parses the
// result, so a control character -- which is what a forged log line needs --
// never reaches the startup log at all; it fails construction, and the error
// carries the raw value into whatever logs it.
func Test_buildExporter_redactsEndpointInError(t *testing.T) {
	writeADC(t)

	cfg := exporters.Config{
		Endpoint: "svc:s3cr3t@telemetry.googleapis.com:443\nlevel=INFO msg=\"forged\"",
		Resource: resource.NewSchemaless(attribute.String(projectIDKey, "p")),
	}

	exp, err := buildExporter(t.Context(), &cfg, &testLogger{})
	if err == nil {
		_ = exp.Shutdown(t.Context())

		t.Fatal("expected an error")
	}

	for _, leak := range []string{"s3cr3t", "\n"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error leaked %q: %s", leak, err)
		}
	}
}

func Test_buildExporter_warnsOnIgnoreListedQuotaHeader(t *testing.T) {
	writeADC(t)

	cfg := exporters.Config{
		Headers:  map[string]string{quotaProjectHeader: "some-project"},
		Resource: resource.NewSchemaless(attribute.String(projectIDKey, "p")),
	}

	l := &testLogger{}

	exp, err := buildExporter(t.Context(), &cfg, l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = exp.Shutdown(t.Context()) }()

	if !l.warnedAbout(quotaProjectHeader) {
		t.Errorf("expected a warning naming %s, got: %v", quotaProjectHeader, l.warnings)
	}
}

func Test_warnMissingProject(t *testing.T) {
	tests := []struct {
		name     string
		resource *resource.Resource
		wantWarn bool
	}{
		{
			name:     "project resolved",
			resource: resource.NewSchemaless(attribute.String(projectIDKey, "my-project")),
			wantWarn: false,
		},
		{
			name:     "attribute present but empty",
			resource: resource.NewSchemaless(attribute.String(projectIDKey, "")),
			wantWarn: true,
		},
		{
			name:     "no project attribute",
			resource: resource.NewSchemaless(attribute.String("service.name", "app")),
			wantWarn: true,
		},
		{name: "no resource at all", resource: nil, wantWarn: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &testLogger{}

			warnMissingProject(&exporters.Config{Resource: tt.resource}, l)

			if got := l.warnedAbout(projectIDKey); got != tt.wantWarn {
				t.Errorf("warning = %v, want %v (warnings: %v)", got, tt.wantWarn, l.warnings)
			}
		})
	}
}

func Test_cachingDetector_projectID(t *testing.T) {
	tests := []struct {
		name        string
		credProject string
		envProject  string
		expected    string
	}{
		{name: "from credentials", credProject: "creds-project", envProject: "env-project", expected: "creds-project"},
		{name: "falls back to the environment", credProject: "", envProject: "env-project", expected: "env-project"},
		{name: "environment is trimmed", credProject: "", envProject: "  env-project  ", expected: "env-project"},
		{name: "neither resolves", credProject: "", envProject: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeADC(t)
			t.Setenv(projectEnv, tt.envProject)

			d := &cachingDetector{adc: resolvedADC(tt.credProject)}

			if got := d.projectID(t.Context()); got != tt.expected {
				t.Errorf("projectID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func Test_cachingDetector_Detect_addsProjectID(t *testing.T) {
	writeADC(t)
	t.Setenv(projectEnv, "env-project")

	d := &cachingDetector{adc: resolvedADC("")}

	// Off Google Cloud the platform detector fails; the resource it produces must
	// still carry the project, and the error must still surface so the SDK can
	// report a partial resource.
	res, _ := d.Detect(t.Context())

	if got := resolvedProject(res); got != "env-project" {
		t.Errorf("gcp.project_id = %q, want %q", got, "env-project")
	}

	// The second call must reuse the cached resource rather than resolving again.
	again, _ := d.Detect(t.Context())
	if again != res {
		t.Error("expected Detect to cache its result")
	}
}

// hangingDetector stands in for the real platform detector under a metadata
// server that accepts the connection and never replies. It ignores its context
// on purpose: so does the real one (contrib/detectors/gcp detector.go:35 takes
// an unnamed context.Context and calls the context-free metadata.OnGCE()), which
// is why a context deadline cannot bound it and the wait has to be bounded
// instead. A stub that honored its context would make this test pass against a
// fix that does nothing in production.
type hangingDetector struct{ released chan struct{} }

func (d hangingDetector) Detect(context.Context) (*resource.Resource, error) {
	<-d.released

	return resource.NewSchemaless(attribute.String("late", "arrival")), nil
}

// shrinkMetadataTimeout keeps the timeout paths in milliseconds. The production
// value is a startup budget, not a unit-test one.
func shrinkMetadataTimeout(t *testing.T) {
	t.Helper()

	previous := metadataTimeout
	metadataTimeout = 50 * time.Millisecond

	t.Cleanup(func() { metadataTimeout = previous })
}

// Test_cachingDetector_Detect_isBoundedByTheStartupDeadline pins the failure the
// deadline exists for. exporters.Build is handed context.Background()
// (pkg/gofr/otel.go:74) and runs before the HTTP server binds, so without a
// bound here a wedged metadata server holds boot open for as long as it hangs --
// and a container that never binds its port never passes its startup probe.
func Test_cachingDetector_Detect_isBoundedByTheStartupDeadline(t *testing.T) {
	writeADC(t)
	shrinkMetadataTimeout(t)

	released := make(chan struct{})
	defer close(released)

	d := &cachingDetector{adc: resolvedADC("creds-project"), platform: hangingDetector{released: released}}

	start := time.Now()
	res, err := d.Detect(t.Context())
	elapsed := time.Since(start)

	if !errors.Is(err, errMetadataTimeout) {
		t.Fatalf("expected errMetadataTimeout, got %v", err)
	}

	// Generous against CI scheduling, and still two orders of magnitude below the
	// unbounded wait: what is under test is that it returns at all.
	if elapsed > time.Second {
		t.Errorf("Detect blocked for %s; the startup deadline did not bound it", elapsed)
	}

	// Startup continues on a partial resource rather than failing: the project is
	// still resolvable from the credentials, and a dropped span beats an app that
	// will not boot.
	if got := resolvedProject(res); got != "creds-project" {
		t.Errorf("gcp.project_id = %q, want %q", got, "creds-project")
	}
}

// Test_adc_get_isBoundedByTheStartupDeadline covers the second call on the same
// startup path. Its deadline is on the wait rather than on the context because
// oauth2/google keeps the context inside the Credentials it returns and reuses
// it for every later token refresh -- a context.WithTimeout would expire export
// about an hour in, long after anything pointed at startup.
func Test_adc_get_isBoundedByTheStartupDeadline(t *testing.T) {
	shrinkMetadataTimeout(t)

	// A credentials file that cannot be read makes FindDefaultCredentials fall
	// through to the metadata server, which here is a listener that accepts and
	// never answers.
	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	defer ln.Close()

	// Accept and hold: the connection is deliberately never answered, which is the
	// shape of a wedged metadata server. Held ones are closed when the test ends
	// -- t.Cleanup cannot be called from this goroutine, which outlives it.
	var (
		mu   sync.Mutex
		held []net.Conn
	)

	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()

		for _, conn := range held {
			conn.Close()
		}
	})

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}

			mu.Lock()

			held = append(held, conn)

			mu.Unlock()
		}
	}()

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("GCE_METADATA_HOST", ln.Addr().String())
	t.Setenv("HOME", t.TempDir())

	var a adc

	start := time.Now()
	_, err = a.get(t.Context())

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("adc.get blocked for %s; the startup deadline did not bound it", elapsed)
	}

	if err == nil {
		t.Fatal("expected an error from a metadata server that never replies")
	}
}

// Static test sentinels: err113 forbids defining them inline.
var (
	errAwaitTimedOut = errors.New("timed out")
	errFromFn        = errors.New("from fn")
)

func Test_awaitWithin(t *testing.T) {
	sentinel := errAwaitTimedOut

	t.Run("returns the result when fn finishes in time", func(t *testing.T) {
		got, err := awaitWithin(time.Second, sentinel, func() (string, error) { return "value", nil })
		if err != nil || got != "value" {
			t.Errorf("awaitWithin() = %q, %v; want \"value\", nil", got, err)
		}
	})

	t.Run("propagates fn's own error", func(t *testing.T) {
		own := errFromFn

		if _, err := awaitWithin(time.Second, sentinel, func() (string, error) { return "", own }); !errors.Is(err, own) {
			t.Errorf("err = %v, want %v", err, own)
		}
	})

	t.Run("returns the timeout error and the zero value when fn overruns", func(t *testing.T) {
		released := make(chan struct{})
		defer close(released)

		got, err := awaitWithin(50*time.Millisecond, sentinel, func() (string, error) {
			<-released

			return "too late", nil
		})

		if !errors.Is(err, sentinel) {
			t.Errorf("err = %v, want %v", err, sentinel)
		}

		if got != "" {
			t.Errorf("value = %q, want the zero value", got)
		}
	})
}

func Test_withProjectID(t *testing.T) {
	tests := []struct {
		name  string
		input *resource.Resource
	}{
		{name: "nil resource", input: nil},
		{name: "existing resource", input: resource.NewSchemaless(attribute.String("service.name", "app"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withProjectID(tt.input, "p")

			if resolvedProject(got) != "p" {
				t.Errorf("expected %s=p on the merged resource, got %v", projectIDKey, got.Attributes())
			}
		})
	}
}

func Test_registeredUnderTheExporterName(t *testing.T) {
	// The blank import is this package's entire public contract: TRACE_EXPORTER=gcp
	// must resolve without the core module knowing anything about Google.
	cfg := exporters.Config{AppName: "app", Exporter: exporterName, Ratio: 1}

	writeADC(t)

	shutdown, tp := exporters.Build(t.Context(), &cfg, &testLogger{})
	defer func() { _ = shutdown(t.Context()) }()

	// Deliberately never ended: an ended span would reach the BatchSpanProcessor
	// and the shutdown flush would try to authenticate against Google.
	_, span := tp.Tracer("test").Start(t.Context(), "span")

	if !span.IsRecording() {
		t.Error("expected TRACE_EXPORTER=gcp to install a recording provider")
	}

	if resolvedProject(cfg.Resource) == "" && !strings.Contains(cfg.Resource.String(), "service.name") {
		t.Error("expected Build to publish a resource")
	}
}
