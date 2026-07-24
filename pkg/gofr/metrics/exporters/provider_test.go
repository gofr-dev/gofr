package exporters

import (
	"context"
	"errors"
	"testing"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"

	"gofr.dev/pkg/gofr/logging"
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
			mp, meter := Build(context.Background(), &tc.cfg, logging.NewMockLogger(logging.INFO))

			if mp == nil {
				t.Fatal("expected non-nil MeterProvider")
			}

			if meter == nil {
				t.Fatal("expected non-nil Meter")
			}

			if _, err := meter.Int64Counter("test_counter"); err != nil {
				t.Errorf("meter should be usable, got error: %v", err)
			}

			_ = mp.Shutdown(context.Background())
		})
	}
}
