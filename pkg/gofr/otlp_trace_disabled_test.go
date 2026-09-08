//go:build gofr_nootlp

package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

// The counterpart to otlp_trace_test.go for a build made with -tags
// gofr_nootlp: TRACE_EXPORTER=otlp must say why it cannot export, and must not
// take the process down doing so.

func TestOTLPTracesOmitted_ExporterErrors(t *testing.T) {
	_, err := buildOtlpExporter(nil, "otlp", "localhost:4317", "", "", nil)

	require.ErrorIs(t, err, errOTLPTracesOmitted)
	assert.Contains(t, err.Error(), "gofr_nootlp")
}

// initTracer logs the error and returns without a span processor. The nil-check
// it does before NewBatchSpanProcessor is what keeps this from panicking on the
// processor's first flush.
func TestOTLPTracesOmitted_InitTracerReportsAndContinues(t *testing.T) {
	cfg := config.NewMockConfig(map[string]string{
		"TRACE_EXPORTER": "otlp",
		"TRACER_URL":     "localhost:4317",
	})

	logMessage := testutil.StderrOutputForFunc(func() {
		mockContainer, _ := container.NewMockContainer(t)

		a := App{Config: cfg, container: mockContainer}

		require.NotPanics(t, a.initTracer)
	})

	assert.Contains(t, logMessage, "gofr_nootlp")
}

// zipkin does not go through OTLP, so it still exports in this build.
func TestOTLPTracesOmitted_ZipkinStillWorks(t *testing.T) {
	cfg := config.NewMockConfig(map[string]string{
		"TRACE_EXPORTER": "zipkin",
		"TRACER_URL":     "http://localhost:2005/api/v2/spans",
	})

	logMessage := testutil.StdoutOutputForFunc(func() {
		mockContainer, _ := container.NewMockContainer(t)

		a := App{Config: cfg, container: mockContainer}
		a.initTracer()
	})

	assert.Contains(t, logMessage, "Exporting traces to zipkin")
}
