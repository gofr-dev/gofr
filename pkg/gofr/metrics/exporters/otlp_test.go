package exporters

import (
	"context"
	"errors"
	"strings"
	"testing"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_temporalitySelector(t *testing.T) {
	tests := []struct {
		pref     string
		kind     metricSdk.InstrumentKind
		expected metricdata.Temporality
	}{
		{"cumulative", metricSdk.InstrumentKindCounter, metricdata.CumulativeTemporality},
		{"", metricSdk.InstrumentKindCounter, metricdata.CumulativeTemporality},
		{"delta", metricSdk.InstrumentKindCounter, metricdata.DeltaTemporality},
		{"delta", metricSdk.InstrumentKindHistogram, metricdata.DeltaTemporality},
		{"delta", metricSdk.InstrumentKindObservableCounter, metricdata.DeltaTemporality},
		{"delta", metricSdk.InstrumentKindUpDownCounter, metricdata.CumulativeTemporality},
		{"delta", metricSdk.InstrumentKindObservableGauge, metricdata.CumulativeTemporality},
		{"lowmemory", metricSdk.InstrumentKindCounter, metricdata.DeltaTemporality},
		{"lowmemory", metricSdk.InstrumentKindHistogram, metricdata.DeltaTemporality},
		{"lowmemory", metricSdk.InstrumentKindObservableCounter, metricdata.CumulativeTemporality},
	}

	for _, tc := range tests {
		if got := temporalitySelector(tc.pref)(tc.kind); got != tc.expected {
			t.Errorf("temporalitySelector(%q)(%v) = %v, want %v", tc.pref, tc.kind, got, tc.expected)
		}
	}
}

func Test_buildOTLPExporter(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"grpc default", Config{Endpoint: "localhost:4317", Protocol: "grpc", Insecure: true}},
		{"grpc with headers", Config{Endpoint: "localhost:4317", Protocol: "grpc", Insecure: true, Headers: map[string]string{"x": "y"}}},
		{"grpc secure with delta", Config{Endpoint: "otlp.nr-data.net:4317", Protocol: "grpc", Temporality: "delta"}},
		{"http host:port", Config{Endpoint: "localhost:4318", Protocol: "http", Insecure: true}},
		{"http full url", Config{Endpoint: "https://collector.example.com/v1/metrics", Protocol: "http"}},
		{"grpc https scheme ignores insecure override", Config{
			Endpoint: "https://collector.example.com:4317", Protocol: "grpc", Insecure: true,
		}},
		{"http https scheme ignores insecure override", Config{
			Endpoint: "https://collector.example.com/v1/metrics", Protocol: "http", Insecure: true,
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exp, err := buildOTLPExporter(context.Background(), &tc.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if exp == nil {
				t.Fatal("expected non-nil exporter")
			}

			_ = exp.Shutdown(context.Background())
		})
	}
}

func Test_buildOTLPReader_emptyEndpoint(t *testing.T) {
	cfg := Config{Endpoint: "", Protocol: "grpc"}

	r, err := buildOTLPReader(context.Background(), &cfg, noopLogger{})
	if r != nil {
		t.Errorf("expected nil reader for empty endpoint, got %v", r)
	}

	if !errors.Is(err, errEmptyOTLPEndpoint) {
		t.Errorf("expected errEmptyOTLPEndpoint, got %v", err)
	}
}

func Test_buildOTLPReader_pushReaderDegradesOnEmptyEndpoint(t *testing.T) {
	// End-to-end: pushReader (the general Build path) must surface the
	// empty-endpoint failure visibly and degrade to prometheus-only,
	// rather than building an otlp reader that silently fails every
	// export interval.
	cfg := Config{AppName: "app", Exporter: "otlp", Protocol: "grpc"}

	out := testutil.StderrOutputForFunc(func() {
		r := pushReader(context.Background(), &cfg, logging.NewMockLogger(logging.WARN))
		if r != nil {
			t.Errorf("expected nil reader when otlp endpoint is empty, got %v", r)
		}
	})

	if !strings.Contains(out, "METRICS_URL") {
		t.Errorf("expected the degrade error to mention METRICS_URL, got: %q", out)
	}
}

func Test_buildOTLPReader_warnsOnUnrecognizedProtocolAndTemporality(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantLog string
	}{
		{"unrecognized protocol", Config{Endpoint: "localhost:4317", Protocol: "carrier-pigeon"}, "METRICS_PROTOCOL"},
		{"unrecognized temporality", Config{Endpoint: "localhost:4317", Temporality: "quantum"}, "METRICS_TEMPORALITY"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				_, err := buildOTLPReader(context.Background(), &tc.cfg, logging.NewMockLogger(logging.WARN))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			if !strings.Contains(out, tc.wantLog) {
				t.Errorf("expected warning to mention %q, got: %q", tc.wantLog, out)
			}
		})
	}
}
