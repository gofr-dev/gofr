//go:build gofr_nootlp

package exporters

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogger struct{}

func (testLogger) Debug(...any)          {}
func (testLogger) Infof(string, ...any)  {}
func (testLogger) Warnf(string, ...any)  {}
func (testLogger) Errorf(string, ...any) {}

// A build made with -tags gofr_nootlp must fail METRICS_EXPORTER=otlp with an
// error that names the tag, rather than dropping "otlp" from the registry and
// letting the caller silently get a different exporter than it configured.

func TestOTLPMetricsOmitted_ExporterErrors(t *testing.T) {
	_, err := buildOTLPExporter(context.Background(), &Config{Endpoint: "localhost:4317"})

	require.ErrorIs(t, err, errOTLPMetricsOmitted)
	assert.Contains(t, err.Error(), "gofr_nootlp")
}

// The reader still validates its config first, so an empty endpoint is still
// reported as an empty endpoint and not as a missing transport.
func TestOTLPMetricsOmitted_StillRegisteredAndValidates(t *testing.T) {
	build, ok := lookup("otlp")
	require.True(t, ok, "otlp stays in the registry so the failure names the tag")

	_, err := build(context.Background(), &Config{Endpoint: ""}, testLogger{})
	require.ErrorIs(t, err, errEmptyOTLPEndpoint)

	_, err = build(context.Background(), &Config{Endpoint: "localhost:4317"}, testLogger{})
	require.ErrorIs(t, err, errOTLPMetricsOmitted)
}
