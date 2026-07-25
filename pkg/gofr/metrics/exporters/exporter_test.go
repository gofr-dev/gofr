package exporters

import (
	"strings"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// TestPrometheus_CardinalityLimitOption verifies the public Prometheus
// constructor returns a usable meter with the default limit and when overridden
// via WithCardinalityLimit (including 0 = unlimited).
func TestPrometheus_CardinalityLimitOption(t *testing.T) {
	tests := []struct {
		desc string
		opts []Option
	}{
		{"default limit", nil},
		{"explicit limit", []Option{WithCardinalityLimit(5000)}},
		{"unlimited", []Option{WithCardinalityLimit(0)}},
	}

	for i, tc := range tests {
		meter := Prometheus("test-app", "v1.0.0", tc.opts...)

		require.NotNil(t, meter, "TEST[%d] %s: meter should not be nil", i, tc.desc)
	}
}

// TestWithCardinalityLimit verifies the option sets the configured limit.
func TestWithCardinalityLimit(t *testing.T) {
	cfg := promConfig{cardinalityLimit: DefaultCardinalityLimit}

	WithCardinalityLimit(0)(&cfg)

	require.Equal(t, 0, cfg.cardinalityLimit)
}

// TestPrometheus_CardinalityLimitApplied proves the configured limit is actually
// wired into the MeterProvider: with a limit of 1, recording two distinct
// attribute sets must produce an OTel cardinality-overflow series, while an
// unlimited (0) meter records both distinctly with no overflow. This fails if
// metricSdk.WithCardinalityLimit is removed from newPrometheusMeter.
func TestPrometheus_CardinalityLimitApplied(t *testing.T) {
	tests := []struct {
		desc         string
		limit        int
		wantOverflow bool
	}{
		{"limit reached produces overflow series", 1, true},
		{"unlimited records both distinctly", 0, false},
	}

	for i, tc := range tests {
		reg := promclient.NewRegistry()

		meter := newPrometheusMeter("test-app", "v1.0.0", promConfig{cardinalityLimit: tc.limit}, reg)
		require.NotNil(t, meter, "TEST[%d] %s", i, tc.desc)

		counter, err := meter.Int64Counter("test_counter")
		require.NoError(t, err, "TEST[%d] %s", i, tc.desc)

		// Two distinct attribute sets — exceeds a limit of 1.
		counter.Add(t.Context(), 1, metric.WithAttributes(attribute.String("k", "a")))
		counter.Add(t.Context(), 1, metric.WithAttributes(attribute.String("k", "b")))

		mfs, err := reg.Gather()
		require.NoError(t, err, "TEST[%d] %s", i, tc.desc)

		overflow := false

		for _, mf := range mfs {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if strings.Contains(lp.GetName(), "overflow") {
						overflow = true
					}
				}
			}
		}

		require.Equal(t, tc.wantOverflow, overflow, "TEST[%d] %s: overflow series presence", i, tc.desc)
	}
}
