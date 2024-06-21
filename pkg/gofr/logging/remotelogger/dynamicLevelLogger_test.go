package remotelogger

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_fetchAndUpdateLogLevel_ErrorCases(t *testing.T) {
	logger := logging.NewMockLogger(logging.INFO)

	ctrl := gomock.NewController(t)
	mockMetrics := service.NewMockMetrics(ctrl)

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", gomock.Any(),
		"method", http.MethodGet, "status", fmt.Sprintf("%v", http.StatusInternalServerError))

	remoteService := service.NewHTTPService("http://", logger, mockMetrics)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		body := `{
			"data": [
				{
					"invalid"
					}
				}
			]
		}`
		_, _ = w.Write([]byte(body))
	}))
	defer mockServer.Close()

	remoteService2 := service.NewHTTPService(mockServer.URL, logger, nil)

	tests := []struct {
		desc            string
		remoteService   service.HTTP
		currentLogLevel logging.Level
	}{
		{"invalid URL for remote service", remoteService, logging.INFO},
		{"invalid response from remote service", remoteService2, logging.DEBUG},
	}

	for i, tc := range tests {
		level, err := fetchAndUpdateLogLevel(tc.remoteService, tc.currentLogLevel)

		assert.Equal(t, tc.currentLogLevel, level, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.NotNil(t, err)
	}
}

func TestDynamicLoggerSuccess(t *testing.T) {
	// Create a mock server that returns a predefined log level
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		body := `{"data":[{"serviceName":"test-service","logLevel":{"LOG_LEVEL":"DEBUG"}}]}`

		_, _ = w.Write([]byte(body))
	}))

	defer mockServer.Close()

	log := testutil.StdoutOutputForFunc(func() {
		// Create a new remote logger with the mock server URL
		remoteLogger := New(logging.INFO, mockServer.URL, "1")

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
