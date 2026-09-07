package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"gofr.dev/pkg/gofr/metrics/exporters"
)

type testLogger struct{ warnings []string }

func (*testLogger) Debug(...any)         {}
func (*testLogger) Infof(string, ...any) {}
func (l *testLogger) Warnf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}
func (*testLogger) Errorf(string, ...any) {}

// warnedAbout reports whether any warning mentions substr, so a test can assert
// on the warning it cares about rather than on a bare count -- buildReader can
// emit both a temporality warning and a location warning in the same call.
func (l *testLogger) warnedAbout(substr string) bool {
	for _, w := range l.warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}

	return false
}

// writeADC points GOOGLE_APPLICATION_CREDENTIALS at a well-formed authorized_user
// credentials file so FindDefaultCredentials resolves without any network call.
func writeADC(t *testing.T) {
	t.Helper()

	adc := map[string]string{
		"type":          "authorized_user",
		"client_id":     "test-id",
		"client_secret": "test-secret",
		"refresh_token": "test-token",
	}

	b, err := json.Marshal(adc)
	if err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", p)
}

func Test_buildReader(t *testing.T) {
	writeADC(t)
	// Off Google Cloud the metadata detector finds nothing, so location has to
	// come from the environment; set it unless the case is exercising its absence.
	t.Setenv(otelResourceAttrsEnv, "location=us-central1")

	tests := []struct {
		name         string
		cfg          exporters.Config
		wantTemporal bool
	}{
		{"cumulative default endpoint", exporters.Config{Interval: time.Second}, false},
		{"explicit endpoint", exporters.Config{Endpoint: defaultEndpoint, Interval: time.Second}, false},
		{"delta preference warns and is ignored", exporters.Config{Interval: time.Second, Temporality: "delta"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := &testLogger{}

			r, err := buildReader(context.Background(), &tc.cfg, l)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if r == nil {
				t.Fatal("expected non-nil reader")
			}

			if got := l.warnedAbout("METRICS_TEMPORALITY"); got != tc.wantTemporal {
				t.Errorf("temporality warning = %v, want %v", got, tc.wantTemporal)
			}

			if l.warnedAbout("location") {
				t.Errorf("did not expect a location warning when location is set, got: %v", l.warnings)
			}

			_ = r.Shutdown(context.Background())
		})
	}
}

// A push with no resolvable location authenticates and connects, and is then
// discarded per-point server-side with nothing on the wire to notice. The
// warning is the only signal an operator gets, so it is worth a test.
func Test_buildReader_warnsWhenLocationUnresolvable(t *testing.T) {
	writeADC(t)
	t.Setenv(otelResourceAttrsEnv, "")

	l := &testLogger{}
	cfg := exporters.Config{Interval: time.Second}

	r, err := buildReader(context.Background(), &cfg, l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = r.Shutdown(context.Background()) }()

	if !l.warnedAbout("location") {
		t.Errorf("expected a location warning, got: %v", l.warnings)
	}

	if !l.warnedAbout(otelResourceAttrsEnv) {
		t.Errorf("warning should name the env var that fixes it, got: %v", l.warnings)
	}
}

func Test_buildReader_warnsOnQuotaProjectHeader(t *testing.T) {
	writeADC(t)
	t.Setenv(otelResourceAttrsEnv, "location=us-central1")

	l := &testLogger{}
	cfg := exporters.Config{
		Interval: time.Second,
		Headers:  map[string]string{"x-goog-user-project": "some-project"},
	}

	r, err := buildReader(context.Background(), &cfg, l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = r.Shutdown(context.Background()) }()

	if !l.warnedAbout("GOOGLE_CLOUD_QUOTA_PROJECT") {
		t.Errorf("expected a quota-project warning, got: %v", l.warnings)
	}
}

func Test_resolves_location(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"empty", "", false},
		{"location", "location=us-central1", true},
		{"cloud.region", "cloud.region=europe-west1", true},
		{"cloud.availability_zone", "cloud.availability_zone=us-central1-a", true},
		{"among others", "service.version=1,location=us-central1,team=core", true},
		{"unrelated attributes only", "service.version=1,team=core", false},
		// "location" must match as a key, not merely appear somewhere in the value.
		{"substring in a value is not a location", "note=relocation-pending", false},
		// A key merely ending in "location" is a different attribute entirely.
		{"key ending in location", "custom.location=us-central1", false},
		{"location present but empty", "location=", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolves(nil, tc.env, locationAttributes); got != tc.want {
				t.Errorf("resolves(nil, %q, location) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

// An attribute present but empty is what Google sees as empty, and it is a real
// case: the SDK host detector reads /etc/machine-id and succeeds on the empty
// file that ubuntu base images ship, so host.id arrives present and blank.
func Test_resolves_treatsEmptyAttributeAsAbsent(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"populated host.id", "abc123", true},
		{"empty host.id, as on ubuntu images", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := resource.NewWithAttributes("", attribute.String("host.id", tc.value))

			if got := resolves(res, "", instanceAttributes); got != tc.want {
				t.Errorf("resolves(host.id=%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func Test_resolves_instanceSources(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"empty", "", false},
		{"service.instance.id", "service.instance.id=pod-abc", true},
		{"k8s.pod.name", "k8s.pod.name=my-pod-7d9f", true},
		{"faas.instance", "faas.instance=0056bf3c9a", true},
		{"location only does not fill instance", "location=us-central1", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolves(nil, tc.env, instanceAttributes); got != tc.want {
				t.Errorf("resolves(nil, %q, instance) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

// The instance warning must fire on its own: a container with a location set but
// no host id is the common Kubernetes case, and it loses every point.
func Test_buildReader_warnsWhenInstanceUnresolvable(t *testing.T) {
	writeADC(t)
	t.Setenv(otelResourceAttrsEnv, "location=us-central1")

	l := &testLogger{}
	cfg := exporters.Config{
		Interval: time.Second,
		// No host.id, as in a debian or alpine based image.
		Resource: resource.NewWithAttributes("", attribute.String("location", "us-central1")),
	}

	r, err := buildReader(context.Background(), &cfg, l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = r.Shutdown(context.Background()) }()

	if !l.warnedAbout("no instance could be resolved") {
		t.Errorf("expected an instance warning, got: %v", l.warnings)
	}

	if l.warnedAbout("no location could be resolved") {
		t.Errorf("location was set; it must not warn, got: %v", l.warnings)
	}
}
