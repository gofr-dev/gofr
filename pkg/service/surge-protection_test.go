package service

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/log"
)

func initializeTest(statusCode int) (*surgeProtector, *httptest.Server, *bytes.Buffer) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))

	sp := new(surgeProtector)

	sp.customHeartbeatURL = "/.well-known/heartbeat"
	sp.logger = logger

	return sp, ts, b
}

func Test_checkHealth(t *testing.T) {
	sp, ts, _ := initializeTest(http.StatusOK)
	ch := make(chan bool)

	go sp.checkHealth(ts.URL, ch)

	if got := <-ch; got != true {
		t.Errorf("FAILED, Expected: %v, Got: %v", true, got)
	}

	ts.Close()
}

func Test_checkHealth_InternalServerError(t *testing.T) {
	sp, ts, b := initializeTest(http.StatusInternalServerError)
	expectedLog := "Health check failed for " + ts.URL + " Reason: Status Code 500"
	ch := make(chan bool)

	go sp.checkHealth(ts.URL, ch)

	if got := <-ch; got != false {
		t.Errorf("Test case Failed, Expected: %v, Got: %v", false, got)
	}

	if !strings.Contains(b.String(), expectedLog) {
		t.Errorf("FAILED expected %v,got: %v", expectedLog, b.String())
	}

	ts.Close()
}

func Test_checkHealthError(t *testing.T) {
	sp, _, b := initializeTest(http.StatusOK)
	expLog := "Health check failed for http://localhost:9090 Error:"
	ch := make(chan bool)

	go sp.checkHealth("http://localhost:9090", ch)

	assert.False(t, <-ch)

	if !strings.Contains(b.String(), expLog) {
		t.Errorf("FAILED expected %v,got: %v", expLog, b.String())
	}
}

func Test_Goroutine_Count(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
	}))

	s := NewHTTPServiceWithOptions(ts.URL, log.NewLogger(), nil)

	initialGoroutine := runtime.NumGoroutine()

	s.SetSurgeProtectorOptions(false, "", 5)

	finalGoroutine := runtime.NumGoroutine()

	assert.Equal(t, finalGoroutine, initialGoroutine, "Test Failed")
}
