package container

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics/exporters"
	"gofr.dev/pkg/gofr/testutil"
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
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "cumulative", Insecure: false,
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
			name: "insecure explicitly enabled",
			env:  map[string]string{"METRICS_INSECURE": "true"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "cumulative", Insecure: true,
			},
		},
		{
			name: "invalid interval falls back to default",
			env:  map[string]string{"METRICS_EXPORT_INTERVAL": "not-a-number"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "cumulative", Insecure: false,
			},
		},
		{
			name: "zero interval falls back to default",
			env:  map[string]string{"METRICS_EXPORT_INTERVAL": "0"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "cumulative", Insecure: false,
			},
		},
		{
			name: "negative interval falls back to default",
			env:  map[string]string{"METRICS_EXPORT_INTERVAL": "-5"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "cumulative", Insecure: false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := metricsExporterConfig(config.NewMockConfig(tc.env), "app", "v1", logging.NewMockLogger(logging.ERROR))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("metricsExporterConfig() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

// Test_metricsExporterConfig_otelFallback covers interval/temporality falling
// back to the OpenTelemetry standard env vars, with METRICS_* taking precedence.
func Test_metricsExporterConfig_otelFallback(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want exporters.Config
	}{
		{
			name: "OTEL interval (ms) used when METRICS_EXPORT_INTERVAL unset",
			env:  map[string]string{"OTEL_METRIC_EXPORT_INTERVAL": "5000"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 5 * time.Second, Temporality: "cumulative", Insecure: false,
			},
		},
		{
			name: "METRICS_EXPORT_INTERVAL wins over OTEL interval",
			env: map[string]string{
				"METRICS_EXPORT_INTERVAL": "15", "OTEL_METRIC_EXPORT_INTERVAL": "5000",
			},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 15 * time.Second, Temporality: "cumulative", Insecure: false,
			},
		},
		{
			name: "invalid METRICS interval falls back to OTEL interval",
			env: map[string]string{
				"METRICS_EXPORT_INTERVAL": "not-a-number", "OTEL_METRIC_EXPORT_INTERVAL": "5000",
			},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 5 * time.Second, Temporality: "cumulative", Insecure: false,
			},
		},
		{
			name: "OTEL temporality preference used when METRICS_TEMPORALITY unset",
			env:  map[string]string{"OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE": "delta"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "delta", Insecure: false,
			},
		},
		{
			name: "METRICS_TEMPORALITY wins over OTEL preference",
			env: map[string]string{
				"METRICS_TEMPORALITY": "cumulative", "OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE": "delta",
			},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1",
				Protocol: "grpc", Interval: 30 * time.Second, Temporality: "cumulative", Insecure: false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := metricsExporterConfig(config.NewMockConfig(tc.env), "app", "v1", logging.NewMockLogger(logging.ERROR))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("metricsExporterConfig() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

func Test_metricsExporterConfig_headers(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want exporters.Config
	}{
		{
			name: "headers parsed from METRICS_HEADERS",
			env:  map[string]string{"METRICS_HEADERS": "dd-api-key=abc, x-tenant=t1"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1", Protocol: "grpc", Interval: 30 * time.Second,
				Temporality: "cumulative", Insecure: false,
				Headers: map[string]string{"dd-api-key": "abc", "x-tenant": "t1"},
			},
		},
		{
			name: "auth key used when headers unset",
			env:  map[string]string{"METRICS_AUTH_KEY": "Basic zzz"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1", Protocol: "grpc", Interval: 30 * time.Second,
				Temporality: "cumulative", Insecure: false,
				Headers: map[string]string{"Authorization": "Basic zzz"},
			},
		},
		{
			name: "headers win over auth key when both set",
			env: map[string]string{
				"METRICS_HEADERS":  "Authorization=Bearer headers-token",
				"METRICS_AUTH_KEY": "Bearer auth-key-token",
			},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1", Protocol: "grpc", Interval: 30 * time.Second,
				Temporality: "cumulative", Insecure: false,
				Headers: map[string]string{"Authorization": "Bearer headers-token"},
			},
		},
		{
			name: "malformed header pairs are dropped",
			env:  map[string]string{"METRICS_HEADERS": "no-equals-sign, =empty-key, empty-value=, ok=1"},
			want: exporters.Config{
				AppName: "app", AppVersion: "v1", Protocol: "grpc", Interval: 30 * time.Second,
				Temporality: "cumulative", Insecure: false,
				Headers: map[string]string{"ok": "1"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := metricsExporterConfig(config.NewMockConfig(tc.env), "app", "v1", logging.NewMockLogger(logging.ERROR))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("metricsExporterConfig() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

func Test_metricsExporterConfig_insecureHeadersWarning(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		wantWarn bool
	}{
		{
			name:     "warns when insecure with headers",
			env:      map[string]string{"METRICS_INSECURE": "true", "METRICS_AUTH_KEY": "Basic zzz"},
			wantWarn: true,
		},
		{
			name:     "no warning when insecure without headers",
			env:      map[string]string{"METRICS_INSECURE": "true"},
			wantWarn: false,
		},
		{
			name:     "no warning when secure with headers",
			env:      map[string]string{"METRICS_AUTH_KEY": "Basic zzz"},
			wantWarn: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				metricsExporterConfig(config.NewMockConfig(tc.env), "app", "v1", logging.NewMockLogger(logging.WARN))
			})

			gotWarn := strings.Contains(out, "plaintext")
			if gotWarn != tc.wantWarn {
				t.Errorf("plaintext warning present = %v, want %v (output: %q)", gotWarn, tc.wantWarn, out)
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

	shutdown, _ := exporters.Build(context.Background(), &cfg, logging.NewMockLogger(logging.INFO))
	c = &Container{shutdownMetrics: shutdown}

	if err := c.ShutdownMetrics(context.Background()); err != nil {
		t.Errorf("configured provider: expected nil error, got %v", err)
	}

	// Idempotent: a second call is a no-op returning the first result, not the
	// MeterProvider's ErrReaderShutdown. App.Shutdown is public, so a manual call
	// plus the signal handler can legitimately invoke this twice.
	if err := c.ShutdownMetrics(context.Background()); err != nil {
		t.Errorf("repeat call: expected nil error from idempotent shutdown, got %v", err)
	}
}

func Test_metricsExporterConfig_resourceAttributes(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "unset when neither var is present",
			env:  map[string]string{},
			want: "",
		},
		{
			name: "OTEL_RESOURCE_ATTRIBUTES is honored",
			env:  map[string]string{"OTEL_RESOURCE_ATTRIBUTES": "cloud.region=asia-south1"},
			want: "cloud.region=asia-south1",
		},
		{
			name: "METRICS_RESOURCE_ATTRIBUTES is honored",
			env:  map[string]string{"METRICS_RESOURCE_ATTRIBUTES": "faas.instance=inst-1"},
			want: "faas.instance=inst-1",
		},
		{
			name: "both are concatenated with the GoFr-native value last",
			env: map[string]string{
				"OTEL_RESOURCE_ATTRIBUTES":    "cloud.region=asia-south1,k8s.pod.name=pod-1",
				"METRICS_RESOURCE_ATTRIBUTES": "cloud.region=europe-west1",
			},
			want: "cloud.region=asia-south1,k8s.pod.name=pod-1,cloud.region=europe-west1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := metricsExporterConfig(config.NewMockConfig(tc.env), "app", "v1", logging.NewMockLogger(logging.ERROR))
			if got.ResourceAttributes != tc.want {
				t.Errorf("ResourceAttributes = %q, want %q", got.ResourceAttributes, tc.want)
			}
		})
	}
}
