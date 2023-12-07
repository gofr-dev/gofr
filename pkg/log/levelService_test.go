package log

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRemoteLevelLogger(t *testing.T) {
	tests := []struct {
		desc        string
		level       level
		serviceName string
		body        []byte
	}{
		{"success case", Info, "gofr-sample-api", []byte(`{"data": [{"serviceName": "gofr-sample-api","config": {"LOG_LEVEL": "INFO"}}]}`)},
		{"failure case", Debug, "", nil},
	}

	for i, tc := range tests {
		// test server that returns log level for the app
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(tc.body)
		}))

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/configs?serviceName="+tc.serviceName, http.NoBody)

		tr := &http.Transport{
			//nolint:gosec // need this to skip TLS verification
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}

		logger := NewMockLogger(io.Discard)

		s := &levelService{url: ts.URL}

		s.level = Debug

		s.updateRemoteLevel(client, req, logger)

		ts.Close()

		assert.Equal(t, tc.level, s.level, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestRemoteLevelLoggerRequestError(t *testing.T) {
	// test server that returns log level for the app
	b := new(bytes.Buffer)
	l := NewMockLogger(b)

	req, _ := http.NewRequest(http.MethodGet, "", http.NoBody)
	client := &http.Client{}

	s := &levelService{url: ""}

	s.updateRemoteLevel(client, req, l)

	assert.Contains(t, b.String(), "Could not create log service client")
}

func TestRemoteLevelLoggerNoResponse(t *testing.T) {
	// test server that returns log level for the app
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/configs?serviceName=", http.NoBody)

	tr := &http.Transport{
		//nolint:gosec // need this to skip TLS verification
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	defer ts.Close()

	b := new(bytes.Buffer)
	l := NewMockLogger(b)

	s := &levelService{url: ts.URL}

	s.updateRemoteLevel(client, req, l)

	expectedLog := "Logging Service returned 404 status. Req: " + ts.URL

	if !strings.Contains(b.String(), expectedLog) {
		t.Errorf("expected error")
	}
}

func TestRemoteLevelLogging(t *testing.T) {
	body := []byte(`{"data": [{"serviceName": "gofr-sample-api","config": {"LOG_LEVEL": "WARN"}}]}`)
	// test server that returns log level for the app
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))

	defer ts.Close()

	t.Setenv("LOG_SERVICE_URL", ts.URL)

	l := &logger{
		out: new(bytes.Buffer),
		app: appInfo{
			Data:      make(map[string]interface{}),
			Framework: "gofr-" + GofrVersion,
			syncData:  &sync.Map{},
		},
	}

	l.rls = newLevelService(l, "gofr-app")

	time.Sleep(1 * time.Second)

	lvl := l.rls.level

	if lvl != Warn {
		t.Errorf("expected WARN\tGot %v", lvl)
	}

	if l.rls.app != "gofr-app" {
		t.Errorf("expected APP_NAME : test, Got : %v", l.rls.app)
	}
}

func Test_Goroutine_Count(t *testing.T) {
	l := newLogger()

	t.Setenv("LOG_SERVICE_URL", "mockURL")

	initialGoroutine := runtime.NumGoroutine()

	newLevelService(l, "sample-api")

	finalGoroutine := runtime.NumGoroutine()
	// one goroutine will be there to refresh the log level
	assert.Equal(t, finalGoroutine-1, initialGoroutine, "Test Failed")
}

// TestRemoteLevelLogger_Race tests using the race flag whether the race condition in updateRemoteLevel method is being handled
func TestRemoteLevelLogger_Race(t *testing.T) {
	var s *levelService

	var b bytes.Buffer
	l := NewMockLogger(&b)
	s = &levelService{}

	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data": [{"serviceName": "gofr-sample-api","config": {"LOG_LEVEL": "DEBUG"}}]}`))
	}))
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data": [{"serviceName": "gofr-sample-api","config": {"LOG_LEVEL": "ERROR"}}]}`))
	}))
	ts3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data": [{"serviceName": "gofr-sample-api","config": {"LOG_LEVEL": "WARN"}}]}`))
	}))
	tr := &http.Transport{
		//nolint:gosec // need this to skip TLS verification
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	tests := []struct {
		desc  string
		level level
		url   string
	}{
		{"case when log level is Debug", Debug, ts1.URL + "/configs?serviceName=gofr-sample-api"},
		{"case when log level is Error", Error, ts2.URL + "/configs?serviceName=gofr-sample-api"},
		{"case when log level is Warn", Warn, ts3.URL + "/configs?serviceName=gofr-sample-api"},
	}

	wg := sync.WaitGroup{}
	for i, tc := range tests {
		wg.Add(1)

		req := httptest.NewRequest(http.MethodGet, tc.url, http.NoBody)

		go func() {
			defer wg.Done()
			s.updateRemoteLevel(client, req, l)
		}()
		assert.NotEqualf(t, Info, s.level.String(), "TEST[%d] Failed:%v,Final log level should never be Info", i, tc.desc)
	}

	wg.Wait()
}
