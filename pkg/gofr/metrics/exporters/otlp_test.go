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
