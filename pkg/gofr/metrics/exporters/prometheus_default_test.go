package exporters

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

// The Prometheus reader Build always installs registers on prometheus.DefaultRegisterer, so what
// the provider records is observable through the default gatherer whether or not GoFr's own
// metrics server runs (METRICS_PORT=0 only skips that server) and with no push exporter set.
// An application serving promhttp.Handler() itself reads it there. This is why the request
// histogram cannot skip Record when "no reader is registered": in a GoFr app there always is one,
// and its output is user-visible.
func TestBuild_prometheusReaderIsAlwaysObservable(t *testing.T) {
	cfg := Config{AppName: "app", AppVersion: "v1"}

	shutdown, meter := Build(context.Background(), &cfg, logging.NewMockLogger(logging.ERROR))

	defer func() { _ = shutdown(context.Background()) }()

	h, err := meter.Float64Histogram("v8_reader_proof_seconds")
	require.NoError(t, err)

	h.Record(context.Background(), 0.01)

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	found := false

	for _, f := range families {
		if f.GetName() == "v8_reader_proof_seconds" {
			found = true
		}
	}

	assert.True(t, found, "a recording with no exporter configured reaches the default Prometheus gatherer")
}
