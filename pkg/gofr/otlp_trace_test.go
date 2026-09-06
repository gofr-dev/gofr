//go:build !gofr_nootlp

package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

// The OTLP-transport cases of Test_initTracer, which exist only in a build
// without -tags gofr_nootlp. The zipkin and gofr cases stay in gofr_test.go:
// neither goes through OTLP, so both work in either build.
func Test_initTracer_otlpTransport(t *testing.T) {
	mockConfig := func(traceExporter, authKey string) config.Config {
		return config.NewMockConfig(map[string]string{
			"TRACE_EXPORTER":  traceExporter,
			"TRACER_URL":      "localhost:4317",
			"TRACER_AUTH_KEY": authKey,
		})
	}

	tests := []struct {
		desc               string
		config             config.Config
		expectedLogMessage string
	}{
		{"jaeger exporter", mockConfig("jaeger", ""), "Exporting traces to jaeger at localhost:4317"},
		{"jaeger exporter with auth", mockConfig("jaeger", "valid-token"), "Exporting traces to jaeger at localhost:4317"},
		{"otlp exporter", mockConfig("otlp", ""), "Exporting traces to otlp at localhost:4317"},
		{"otlp exporter with authKey", mockConfig("otlp", "valid-token"), "Exporting traces to otlp at localhost:4317"},
	}

	for i, tc := range tests {
		logMessage := testutil.StdoutOutputForFunc(func() {
			mockContainer, _ := container.NewMockContainer(t)

			a := App{Config: tc.config, container: mockContainer}
			a.initTracer()
		})
		assert.Contains(t, logMessage, tc.expectedLogMessage, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
