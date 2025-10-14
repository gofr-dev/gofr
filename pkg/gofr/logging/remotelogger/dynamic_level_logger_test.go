package remotelogger

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestRemoteLogger_UpdateLevel(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		body := `{ "data": { "serviceName": "test-service","logLevel":"DEBUG" } }`
		_, _ = w.Write([]byte(body))
	}))

	rl := remoteLogger{
		remoteURL:          mockServer.URL,
		levelFetchInterval: 100 * time.Millisecond,
		currentLevel:       2,
		Logger:             logging.NewMockLogger(logging.INFO),
	}

	go rl.UpdateLogLevel()

	time.Sleep(200 * time.Millisecond)

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

	time.Sleep(100 * time.Millisecond)

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
		rl := New(logging.INFO, mockServer.URL, 100*time.Millisecond)

		// Wait for the remote logger to update the log level
		time.Sleep(200 * time.Millisecond)

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

// TestHTTPLogFilter_NonHTTPLogs tests regular non-HTTP logs are passed through.
func TestHTTPLogFilter_NonHTTPLogs(t *testing.T) {
	var buf strings.Builder

	testLogger := &testBufferLogger{buf: &buf}

	filter := &httpLogFilter{
		Logger: testLogger,
	}

	filter.Log("This is a regular message")

	assert.Contains(t, buf.String(), "This is a regular message")
}

// TestHTTPLogFilter_EmptyArgs tests handling of empty arguments.
func TestHTTPLogFilter_EmptyArgs(t *testing.T) {
	var buf strings.Builder

	testLogger := &testBufferLogger{buf: &buf}

	filter := &httpLogFilter{
		Logger: testLogger,
	}

	filter.Log()

	// Should not write anything meaningful
	assert.Equal(t, "\n", buf.String())
}

// TestHTTPLogFilter_InitAndFirstSuccess tests initialization and first successful hit.
func TestHTTPLogFilter_InitAndFirstSuccess(t *testing.T) {
	var buf strings.Builder

	testLogger := &testBufferLogger{buf: &buf}

	filter := &httpLogFilter{
		Logger: testLogger,
	}

	successLog := &service.Log{
		URI:           "http://example.com/test",
		ResponseCode:  200,
		ResponseTime:  150,
		HTTPMethod:    "GET",
		CorrelationID: "test-id-1",
	}

	filter.Log(successLog)

	output := buf.String()
	assert.Contains(t, output, "Initializing remote logger connection to http://example.com/test")
	assert.Contains(t, output, "test-id-1")
}

// TestHTTPLogFilter_SubsequentSuccess tests subsequent successful HTTP hits.
func TestHTTPLogFilter_SubsequentSuccess(t *testing.T) {
	var buf strings.Builder

	testLogger := &testBufferLogger{buf: &buf}

	filter := &httpLogFilter{
		Logger:             testLogger,
		firstSuccessfulHit: true,
		initLogged:         true,
	}

	successLog := &service.Log{
		URI:           "http://example.com/test2",
		ResponseCode:  200,
		ResponseTime:  200,
		HTTPMethod:    "POST",
		CorrelationID: "test-id-2",
	}

	filter.Log(successLog)

	output := buf.String()
	assert.Contains(t, output, "test-id-2")
	assert.Contains(t, output, "POST")
	assert.Contains(t, output, "http://example.com/test2")
}

// TestHTTPLogFilter_ErrorLogs tests handling of error HTTP logs.
func TestHTTPLogFilter_ErrorLogs(t *testing.T) {
	var buf strings.Builder

	testLogger := &testBufferLogger{buf: &buf}

	filter := &httpLogFilter{
		Logger:             testLogger,
		firstSuccessfulHit: true,
		initLogged:         true,
	}

	errorLog := &service.Log{
		URI:           "http://example.com/error",
		ResponseCode:  500,
		ResponseTime:  300,
		HTTPMethod:    "GET",
		CorrelationID: "test-id-3",
	}

	filter.Log(errorLog)

	output := buf.String()
	assert.Contains(t, output, "http://example.com/error")
	assert.Contains(t, output, "500")
}

func TestHTTPLogFilter_ConcurrentAccess(t *testing.T) {
	const (
		goroutines       = 50
		logsPerGoroutine = 20
	)

	var buf strings.Builder

	testLogger := &testBufferLogger{buf: &buf}
	filter := &httpLogFilter{Logger: testLogger}

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < logsPerGoroutine; j++ {
				filter.Log(&service.Log{
					CorrelationID: fmt.Sprintf("req-%d-%d", id, j),
					URI:           "/test",
					HTTPMethod:    "GET",
					ResponseCode:  200,
					ResponseTime:  int64(j),
				})
			}
		}(i)
	}

	wg.Wait()

	filter.mu.Lock()
	assert.True(t, filter.initLogged, "expected initLogged to be true")
	filter.mu.Unlock()

	assert.NotEmpty(t, buf.String(), "expected logs to be written")
}

func TestRemoteLogger_ConcurrentLevelAccess(t *testing.T) {
	var count int32

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		lvl := "DEBUG"

		if atomic.AddInt32(&count, 1)%2 == 0 {
			lvl = "ERROR"
		}

		fmt.Fprintf(w, `{"data":{"serviceName":"test-service","logLevel":"%s"}}`, lvl)
	}))

	defer mockServer.Close()

	rl := &remoteLogger{
		remoteURL:          mockServer.URL,
		levelFetchInterval: 5 * time.Millisecond,
		currentLevel:       logging.INFO,
		Logger:             logging.NewMockLogger(logging.INFO),
	}

	// Run UpdateLogLevel in multiple goroutines
	for i := 0; i < 5; i++ {
		go rl.UpdateLogLevel()
	}

	time.Sleep(10 * time.Millisecond)

	rl.mu.RLock()
	defer rl.mu.RUnlock()

	assert.NotEqual(t, logging.INFO, rl.currentLevel, "expected level to change")
}

func TestLogLevelChangeToFatal_NoExit(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := `{ "data": { "serviceName": "test-service", "logLevel": "FATAL" } }`
		_, _ = w.Write([]byte(body))
	}))
	defer mockServer.Close()

	// This test succeeds if it completes without the process exiting
	log := testutil.StdoutOutputForFunc(func() {
		rl := New(logging.INFO, mockServer.URL, 10*time.Millisecond)

		time.Sleep(20 * time.Millisecond)

		// If we reach this point, the application didn't exit
		// Now check that the logger did change to FATAL level
		if remoteLogger, ok := rl.(*remoteLogger); ok {
			remoteLogger.mu.RLock()

			assert.Equal(t, logging.FATAL, remoteLogger.currentLevel, "Log level should be updated to FATAL")

			remoteLogger.mu.RUnlock()
		}
	})

	// Verify the log contains a warning about the level change
	assert.Contains(t, log, "LOG_LEVEL updated from INFO to FATAL")
}
