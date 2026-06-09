package exporters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPrometheus_CardinalityLimitOption verifies that Prometheus constructs a
// usable meter both with the default cardinality limit and when the limit is
// overridden via WithCardinalityLimit (including 0 = unlimited). The limit's
// runtime semantics are owned by the OpenTelemetry SDK; this guards GoFr's
// wiring of the option.
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
