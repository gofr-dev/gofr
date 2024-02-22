package logging

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/testutil"
)

func TestDynamicLoggerSuccess(t *testing.T) {
	// Create a mock server that returns a predefined log level
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := `{
			"data": [
				{
					"serviceName": "test-service",
					"logLevel": {
						"LOG_LEVEL": "DEBUG"
					}
				}
			]
		}`
		_, _ = w.Write([]byte(body))
	}))
	defer mockServer.Close()

	log := testutil.StdoutOutputForFunc(func() {
		// Create a new remote logger with the mock server URL
		remoteLogger := NewRemoteLogger(INFO, mockServer.URL, "1")

		// Wait for the remote logger to update the log level
		time.Sleep(2 * time.Second)

		// Check if the log level has been updated
		remoteLogger.Debug("Debug log after log level change")
	})

	if !strings.Contains(log, "LOG_LEVEL updated from INFO to DEBUG") {
		t.Errorf("TestDynamicLoggerSuccess failed! Missing log message about level update")
	}

	if !strings.Contains(log, "Debug log after log level change") {
		t.Errorf("TestDynamicLoggerSuccess failed! missing debug log")
	}
}
