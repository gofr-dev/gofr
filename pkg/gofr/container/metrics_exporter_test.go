package container

import (
	"context"
	"reflect"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics/exporters"
)

func Test_metricsExporterConfig(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want exporters.Config
	}{
		{
			name: "defaults when unset",
			env:  map[string]string{},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "cumulative", Insecure: true,
			},
		},
		{
			name: "full otlp config",
			env: map[string]string{
				"METRICS_EXPORTER": "otlp", "METRICS_URL": "collector:4317", "METRICS_PROTOCOL": "http",
				"METRICS_EXPORT_INTERVAL": "15", "METRICS_TEMPORALITY": "delta", "METRICS_INSECURE": "false",
			},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1", Exporter: "otlp", Endpoint: "collector:4317",
				Protocol: "http", Interval: 15 * time.Second, Temporality: "delta", Insecure: false,
			},
		},
		{
			name: "invalid interval falls back to default",
			env:  map[string]string{"METRICS_EXPORT_INTERVAL": "not-a-number"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "cumulative", Insecure: true,
			},
		},
		{
			name: "headers parsed from METRICS_HEADERS",
			env:  map[string]string{"METRICS_HEADERS": "dd-api-key=abc, x-tenant=t1"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1", Protocol: "grpc", Interval: 30 * time.Second,
				Temporality: "cumulative", Insecure: true,
				Headers: map[string]string{"dd-api-key": "abc", "x-tenant": "t1"},
			},
		},
		{
			name: "auth key used when headers unset",
			env:  map[string]string{"METRICS_AUTH_KEY": "Basic zzz"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1", Protocol: "grpc", Interval: 30 * time.Second,
				Temporality: "cumulative", Insecure: true,
				Headers: map[string]string{"Authorization": "Basic zzz"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := metricsExporterConfig(config.NewMockConfig(tc.env), "app", "v1")
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("metricsExporterConfig() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

func TestContainer_ShutdownMetrics(t *testing.T) {
	// Nil provider is a no-op.
	c := &Container{}
	if err := c.ShutdownMetrics(context.Background()); err != nil {
		t.Errorf("nil provider: expected nil error, got %v", err)
	}

	// A configured provider flushes and shuts down without error.
	cfg := exporters.Config{AppName: "t"}

	mp, _ := exporters.Build(context.Background(), &cfg, logging.NewMockLogger(logging.INFO))
	c = &Container{metricsProvider: mp}

	if err := c.ShutdownMetrics(context.Background()); err != nil {
		t.Errorf("configured provider: expected nil error, got %v", err)
	}
}
