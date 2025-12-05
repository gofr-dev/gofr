package gofr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

func Test_zipkinExporterWrapper_Status202(t *testing.T) {
	// Create a mock server that returns status 202 (Accepted) - the expected status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted) // 202
	}))
	defer server.Close()

	logger := logging.NewLogger(logging.INFO)

	exporter, err := buildZipkinExporter(logger, server.URL, "", "", "")
	require.NoError(t, err)

	spans := provideSampleSpan(t)

	// Export spans - should succeed with 202
	err = exporter.ExportSpans(t.Context(), spans)
	require.NoError(t, err, "Status 202 should succeed")
}

func Test_zipkinExporterWrapper_Status400(t *testing.T) {
	// Create a mock server that returns status 400 (Bad Request)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest) // 400
	}))
	defer server.Close()

	logger := logging.NewLogger(logging.INFO)

	exporter, err := buildZipkinExporter(logger, server.URL, "", "", "")
	require.NoError(t, err)

	spans := provideSampleSpan(t)

	// Export spans - should return error for 400
	err = exporter.ExportSpans(t.Context(), spans)
	require.Error(t, err, "Status 400 should return error")
	assert.Contains(t, err.Error(), "failed to send spans to zipkin server",
		"Error should mention failed to send spans")
	assert.Contains(t, err.Error(), "400", "Error should include status code")
}

func Test_zipkinExporterWrapper_Shutdown(t *testing.T) {
	logger := logging.NewLogger(logging.INFO)

	// Create exporter with invalid URL to test shutdown doesn't fail
	exporter, err := buildZipkinExporter(logger, "http://invalid-url:9411/api/v2/spans", "", "", "")
	require.NoError(t, err)

	// Shutdown should not error
	err = exporter.Shutdown(t.Context())
	require.NoError(t, err, "Shutdown should not error")
}
