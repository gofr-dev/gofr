package remotelogger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func TestRemoteLogger_UpdateLevel(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		body := `{ "data": { "serviceName": "test-service","logLevel":"DEBUG" } }`
		_, _ = w.Write([]byte(body))
	}))

	rl := remoteLogger{
		remoteURL:          mockServer.URL,
		levelFetchInterval: 1,
		currentLevel:       2,
		Logger:             logging.NewMockLogger(logging.INFO),
	}

	go rl.UpdateLogLevel()

	time.Sleep(2 * time.Second)

	assert.Equal(t, logging.DEBUG, rl.currentLevel)
}

func TestRemoteLogger_UpdateLevelError(t *testing.T) {
	rl := remoteLogger{
		remoteURL:          "invalid url",
		levelFetchInterval: 1,
		currentLevel:       2,
		Logger:             logging.NewMockLogger(logging.INFO),
	}

	go rl.UpdateLogLevel()

	time.Sleep(2 * time.Second)

	assert.Equal(t, logging.INFO, rl.currentLevel)
}

func Test_fetchAndUpdateLogLevel_InvalidResponse(t *testing.T) {
	logger := logging.NewMockLogger(logging.INFO)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		body := `{ "data": { "serviceName": "test-service","logLevel":"TEST" } }`

		_, _ = w.Write([]byte(body))
	}))
	defer mockServer.Close()

	remoteService := service.NewHTTPService(mockServer.URL, logger, nil)

	level, err := fetchAndUpdateLogLevel(remoteService, logging.DEBUG)

	assert.Equal(t, logging.DEBUG, level, "Test_fetchAndUpdateLogLevel_InvalidResponse, Failed.\n")

	require.NoError(t, err)
}

func Test_fetchAndUpdateLogLevel_InvalidLogLevel(t *testing.T) {
	logger := logging.NewMockLogger(logging.INFO)

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

	level, err := fetchAndUpdateLogLevel(remoteService2, logging.DEBUG)

	assert.Equal(t, logging.DEBUG, level, "Test_fetchAndUpdateLogLevel_InvalidResponse, Failed.\n")

	require.Error(t, err)
}

func TestDynamicLoggerSuccess(t *testing.T) {
	// Create a mock server that returns a predefined log level
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		body := `{ "data": { "serviceName": "test-service","logLevel":"DEBUG" } }`

		_, _ = w.Write([]byte(body))
	}))

	defer mockServer.Close()

	log := testutil.StdoutOutputForFunc(func() {
		// Create a new remote logger with the mock server URL
		rl := New(logging.INFO, mockServer.URL, "1")

		// Wait for the remote logger to update the log level
		time.Sleep(2 * time.Second)

		// Check if the log level has been updated
		rl.Debug("Debug log after log level change")
	})

	if !strings.Contains(log, "LOG_LEVEL updated from INFO to DEBUG") {
		t.Errorf("TestDynamicLoggerSuccess failed! Missing log message about level update")
	}

	if !strings.Contains(log, "Debug log after log level change") {
		t.Errorf("TestDynamicLoggerSuccess failed! missing debug log")
	}
}
