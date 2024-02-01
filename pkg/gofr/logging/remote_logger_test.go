package logging

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func TestRemoteLevelService_LogLevelUpdate(t *testing.T) {
	// Create a mock HTTP server for testing
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond with a sample log level update
		response := `{"data":[{"serviceName":"my-service","logLevel":{"LOG_LEVEL":"INFO"}}]}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer mockServer.Close()

	result := testutil.StdoutOutputForFunc(func() {
		logger := NewRemoteLogger(testutil.NewMockConfig(map[string]string{
			"LOG_LEVEL":         "DEBUG",
			"REMOTE_LOG_URL":    mockServer.URL,
			"REMOTE_ACCESS_KEY": "test-key",
			"APP_NAME":          "test-app",
		}))

		time.Sleep(7 * time.Second)

		// Log statements to test the updated log level
		logger.Logger.Debug("testing the debug log")
		logger.Logger.Info("testing the info log")
	})

	assert.NotContains(t, result, "debug log")
	assert.Contains(t, result, "info log")
}
