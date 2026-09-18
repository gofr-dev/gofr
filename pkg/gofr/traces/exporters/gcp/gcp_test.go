package gcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"gofr.dev/pkg/gofr/traces/exporters"
	"golang.org/x/oauth2/google"
)

type testLogger struct{ warnings []string }

func (*testLogger) Debug(...any)         {}
func (*testLogger) Infof(string, ...any) {}
func (l *testLogger) Warnf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}
func (*testLogger) Errorf(string, ...any) {}

func (l *testLogger) warnedAbout(substr string) bool {
	for _, w := range l.warnings {
		if strings.Contains(w, substr) {
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

func Test_buildExporter_endpoint(t *testing.T) {
	writeADC(t)

	tests := []struct {
		name string
		cfg  exporters.Config
	}{
		{name: "default endpoint", cfg: exporters.Config{}},
		{name: "TRACER_URL overrides", cfg: exporters.Config{Endpoint: "telemetry.europe-west1.rep.googleapis.com:443"}},
		{name: "headers are forwarded", cfg: exporters.Config{Headers: map[string]string{"X-Tenant": "a"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.Resource = resource.NewSchemaless(attribute.String(projectIDKey, "p"))

			exp, err := buildExporter(t.Context(), &tt.cfg, &testLogger{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if exp == nil {
				t.Fatal("expected a non-nil exporter")
			}

			_ = exp.Shutdown(t.Context())
		})
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
