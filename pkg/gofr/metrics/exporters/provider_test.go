package exporters

import (
	"context"
	"errors"
	"strings"
	"testing"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"

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
