package exporters

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var errBuilderFailed = errors.New("builder failed")

func TestBuild(t *testing.T) {
	Register("build-custom", func(_ context.Context, _ *Config, _ Logger) (metricSdk.Reader, error) {
		return metricSdk.NewManualReader(), nil
	})
	Register("build-fail", func(_ context.Context, _ *Config, _ Logger) (metricSdk.Reader, error) {
		return nil, errBuilderFailed
	})

	tests := []struct {
		name string
		cfg  Config
	}{
		{"prometheus only when empty", Config{AppName: "app", AppVersion: "v1"}},
		{"prometheus explicit", Config{AppName: "app", Exporter: "prometheus"}},
		{"custom push exporter", Config{AppName: "app", Exporter: "build-custom"}},
		{"unknown exporter falls back to prometheus", Config{AppName: "app", Exporter: "does-not-exist"}},
		{"failing builder falls back to prometheus", Config{AppName: "app", Exporter: "build-fail"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shutdown, meter := Build(context.Background(), &tc.cfg, logging.NewMockLogger(logging.INFO))

			if shutdown == nil {
				t.Fatal("expected non-nil ShutdownFunc")
			}

			if meter == nil {
				t.Fatal("expected non-nil Meter")
			}

			if _, err := meter.Int64Counter("test_counter"); err != nil {
				t.Errorf("meter should be usable, got error: %v", err)
			}

			_ = shutdown(context.Background())
		})
	}
}

func TestBuild_missingSubmoduleImportHint(t *testing.T) {
	cfg := Config{AppName: "app", Exporter: "gcp"} // "gcp" is not blank-imported in this test binary

	out := testutil.StderrOutputForFunc(func() {
		shutdown, meter := Build(context.Background(), &cfg, logging.NewMockLogger(logging.ERROR))
		if meter == nil {
			t.Error("expected a usable meter on fallback")
		}

		_ = shutdown(context.Background())
	})

	if !strings.Contains(out, "gofr.dev/pkg/gofr/metrics/exporters/gcp") {
		t.Errorf("expected the import path in the error, got: %q", out)
	}

	if !strings.Contains(out, "blank import") {
		t.Errorf("expected actionable 'blank import' guidance, got: %q", out)
	}
}

// Google's OTLP ingest maps points onto prometheus_target and rejects any whose
// "location" or "instance" label is empty, so the resource has to carry a source
// for both. instance falls back to host.id, which is detected; location can only
// come from the operator, via the standard OTel environment variable.
func TestBuildResource_carriesRequiredLabelSources(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "location=us-central1")

	// host.id is only detected for push exporters, so select one.
	res := buildResource(context.Background(), &Config{AppName: "app", Exporter: "otlp"}, logging.NewMockLogger(logging.INFO))
	if res == nil {
		t.Fatal("expected a resource")
	}

	got := map[string]string{}
	for _, kv := range res.Attributes() {
		got[string(kv.Key)] = kv.Value.String()
	}

	if got["location"] != "us-central1" {
		t.Errorf("location = %q, want %q (OTEL_RESOURCE_ATTRIBUTES must reach the resource)", got["location"], "us-central1")
	}

	if got["host.id"] == "" {
		t.Error("host.id is empty; it is the last fallback Google accepts for the required instance label")
	}

	// The attributes that were already there must survive the merge.
	if got["service.name"] != "app" {
		t.Errorf("service.name = %q, want %q", got["service.name"], "app")
	}

	if got["framework_version"] == "" {
		t.Error("framework_version was dropped")
	}
}

// A registered detector must only run for the exporter it belongs to, so that a
// Prometheus-only app never reaches for a cloud metadata server.
func TestBuildResource_detectorRunsOnlyForItsExporter(t *testing.T) {
	var called bool

	RegisterResourceDetector("detector-probe", detectorFunc(func(context.Context) (*resource.Resource, error) {
		called = true
		return resource.NewWithAttributes("", attribute.String("probe", "yes")), nil
	}))

	buildResource(context.Background(), &Config{AppName: "app"}, logging.NewMockLogger(logging.INFO))

	if called {
		t.Error("detector ran for an unrelated exporter")
	}

	res := buildResource(context.Background(), &Config{AppName: "app", Exporter: "detector-probe"}, logging.NewMockLogger(logging.INFO))

	if !called {
		t.Fatal("detector did not run for its own exporter")
	}

	for _, kv := range res.Attributes() {
		if string(kv.Key) == "probe" {
			return
		}
	}

	t.Error("detector attributes did not reach the resource")
}

type detectorFunc func(context.Context) (*resource.Resource, error)

func (f detectorFunc) Detect(ctx context.Context) (*resource.Resource, error) { return f(ctx) }

// host.id detection is not free -- on darwin the SDK shells out to ioreg, which
// measured ~10ms. The default configuration is prometheus-only and has no use
// for the label, so it must not pay that cost at every application start.
func TestBuildResource_prometheusOnlySkipsHostIDDetection(t *testing.T) {
	for _, exporter := range []string{"", "prometheus"} {
		t.Run("exporter="+exporter, func(t *testing.T) {
			res := buildResource(context.Background(), &Config{AppName: "app", Exporter: exporter},
				logging.NewMockLogger(logging.INFO))

			for _, kv := range res.Attributes() {
				if string(kv.Key) == "host.id" {
					t.Errorf("host.id was detected for a pull-only exporter; that costs a subprocess on darwin")
				}
			}

			// The attributes that matter to every app must still be present.
			var gotServiceName bool

			for _, kv := range res.Attributes() {
				if string(kv.Key) == "service.name" {
					gotServiceName = true
				}
			}

			if !gotServiceName {
				t.Error("service.name missing")
			}
		})
	}
}
