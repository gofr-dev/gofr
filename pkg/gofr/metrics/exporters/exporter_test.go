package exporters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheus(t *testing.T) {
	tests := []struct {
		desc       string
		appName    string
		appVersion string
		resAttrs   string
	}{
		{desc: "named app", appName: "testing-app", appVersion: "v1.0.0"},
		{desc: "empty app name and version", appName: "", appVersion: ""},
		{desc: "malformed resource attributes are tolerated", appName: "testing-app", appVersion: "v1.0.0",
			resAttrs: "missing-value"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Setenv("OTEL_RESOURCE_ATTRIBUTES", tc.resAttrs)

			meter := Prometheus(tc.appName, tc.appVersion)
			require.NotNil(t, meter)

			counter, err := meter.Int64Counter("prometheus_test_counter")
			require.NoError(t, err)
			assert.NotNil(t, counter)
		})
	}
}
