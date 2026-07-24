package exporters

import (
	"context"
	"testing"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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
