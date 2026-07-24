package main

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	// The example points METRICS_URL at a collector that is not present in CI.
	// The OTLP push reader connects lazily and only ticks on its interval, so
	// booting is safe; we assert the pull /metrics endpoint still works in the
	// dual (pull + push) configuration.
	m.Run()
}

func TestIntegration_MetricsEndpointStillServed(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	t.Setenv("METRICS_EXPORTER", "otlp")
	t.Setenv("METRICS_URL", "localhost:4317")

	go main()
	time.Sleep(200 * time.Millisecond) // give the server time to start

	c := http.Client{}

	req, _ := http.NewRequest(http.MethodPost, configs.HTTPHost+"/order", http.NoBody)
	req.Header.Set("content-type", "application/json")

	resp, err := c.Do(req)
	require.NoError(t, err, "request to /order failed")
	resp.Body.Close()

	metricsReq, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("http://localhost:%d/metrics", configs.MetricsPort), http.NoBody)

	metricsResp, err := c.Do(metricsReq)
	require.NoError(t, err, "request to /metrics failed")
	defer metricsResp.Body.Close()

	assert.Equal(t, http.StatusOK, metricsResp.StatusCode, "pull /metrics endpoint should stay available in dual mode")
}
