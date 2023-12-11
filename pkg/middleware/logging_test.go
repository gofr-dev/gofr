package middleware

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

type MockHandler struct {
	statusCode int
}
type MockWriteHandler struct {
}

func (m MockWriteHandler) Header() http.Header {
	return http.Header{}
}
func (m MockWriteHandler) Write([]byte) (int, error) {
	return 0, nil
}
func (m MockWriteHandler) WriteHeader(int) {}

func (r *MockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	switch r.statusCode {
	case 0:
		r.statusCode = http.StatusOK
	case 1:
		r.statusCode = http.StatusInternalServerError
	}

	w.WriteHeader(r.statusCode)
	_, _ = w.Write([]byte("testing log"))
}

func TestLogging(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	handler := Logging(logger, "")(&MockHandler{})

	req := httptest.NewRequest("GET", "/dummy", http.NoBody)
	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	x := b.String()
	if !strings.Contains(x, "time") || !strings.Contains(x, "level") {
		t.Errorf("error, expected fields are not present in log, got: %v", x)
	}
}

func Test5xxLogs(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	handler := Logging(logger, "")(&MockHandler{1})

	req := httptest.NewRequest("GET", "/dummy", http.NoBody)
	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	x := b.String()
	assert.NotContains(t, x, "INFO", "Test Failed")
	assert.Containsf(t, x, "\"type\":\"ERROR\"", "5xx responses could not be logged, got: %v", x)
}

func TestExemptPath(t *testing.T) {
	b := new(bytes.Buffer)
	handler := Logging(log.NewMockLogger(b), "")(&MockHandler{})

	request := httptest.NewRequest("GET", "/metrics", http.NoBody)
	handler.ServeHTTP(MockWriteHandler{}, request)

	x := b.String()
	if !strings.Contains(x, "time") || !strings.Contains(x, "level") {
		t.Errorf("error, expected fields are not present in log, got: %v", x)
	}

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}
}

func TestExemptPathWith5xxLogs(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	handler := Logging(logger, "")(&MockHandler{http.StatusInternalServerError})

	req := httptest.NewRequest("GET", "/.well-known/health-check", http.NoBody)
	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	x := b.String()
	if !strings.Contains(x, "time") || !strings.Contains(x, "level") ||
		!strings.Contains(x, "ERROR") || !strings.Contains(x, "DEBUG") {
		t.Errorf("error, expected fields or log levels are not present in log, got: %v", x)
	}
}

func TestGetIPAddress(t *testing.T) {
	{
		// 1. When RemoteAddr is set
		addr := "0.0.0.0:8080"
		req, err := http.NewRequest("GET", "http://dummy", http.NoBody)
		if err != nil {
			t.Errorf("FAILED, got error creating req object: %v", err)
		}

		req.RemoteAddr = addr

		if ip := GetIPAddress(req); ip != addr {
			t.Errorf("FAILED, expected: %v, got: %v", addr, ip)
		}
	}

	{
		// 2. When `X-Forwarded-For` header is set
		addr := "192.168.0.1:8080"

		req, err := http.NewRequest("GET", "http://dummy", http.NoBody)
		if err != nil {
			t.Errorf("FAILED, got error creating req object: %v", err)
		}

		req.Header.Set("X-Forwarded-For", addr)

		if ip := GetIPAddress(req); ip != addr {
			t.Errorf("FAILED, expected: %v, got: %v", addr, ip)
		}
	}
}

func TestLoggingCorrelationID(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	handler := Logging(logger, "")(&MockHandler{})

	req := httptest.NewRequest("GET", "/dummy", http.NoBody)
	req.Header.Add("X-B3-TraceID", "12bhu987")
	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	x := b.String()
	if !strings.Contains(x, "correlationId") || !strings.Contains(x, "12bhu987") {
		t.Errorf("error, expected correlation id in log, got: %v", x)
	}
}

func TestLoggingCorrelationContext(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	handler := Logging(logger, "")(&MockHandler{})

	const correlationID = "b00ff8de800911ec8f6502bfe7568078"

	tests := []struct {
		desc   string
		header string
	}{
		{"set X-Correlation-ID header", "X-Correlation-ID"},
		{"set X-B3-TraceID header", "X-B3-TraceID"},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/dummy", http.NoBody)

		req.Header.Add(tc.header, correlationID)
		handler.ServeHTTP(MockWriteHandler{}, req)

		cID, _ := req.Context().Value(CorrelationIDKey).(string)

		if cID != correlationID {
			t.Errorf("TEST[%d], failed.\n%s\nCorrelationID is not present in the request context.", i, tc.desc)
		}
	}
}

func TestGetCorrelationID(t *testing.T) {
	const (
		correlationIDLength  = 32
		invalidCorrelationID = "000"
	)

	nullCorrelationID := fmt.Sprintf("%0*s", correlationIDLength, "")

	tests := []struct {
		desc   string
		header string
		value  string
	}{
		{"empty X-Correlation-ID header", "X-Correlation-ID", ""},
		{"empty X-B3-TraceID header", "X-B3-TraceID", ""},
		{"invalid CorrelationID for X-Correlation-ID header", "X-Correlation-ID", invalidCorrelationID},
		{"invalid CorrelationID for X-B3-TraceID header", "X-B3-TraceID", invalidCorrelationID},
		{"null CorrelationID for X-Correlation-ID header", "X-Correlation-ID", nullCorrelationID},
		{"null CorrelationID for X-B3-TraceID header", "X-B3-TraceID", nullCorrelationID},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/dummy", http.NoBody)
		req.Header.Add(tc.header, tc.value)

		correlationID := getCorrelationID(req)

		if correlationID == tc.value {
			t.Errorf("TEST[%d], failed.\n%s\nExpected valid CorrelationID, Got: %v .", i, tc.desc, tc.value)
		}

		if req.Header.Get("X-Correlation-ID") == tc.value {
			t.Errorf("TEST[%d], failed.\n%s\nExpected valid CorrelationID to be set in request header, Got: %v .",
				i, tc.desc, tc.value)
		}
	}
}

func TestLoggingOmitHeader(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	omitHeaders := "X-Some-Random-Header-1,X-Some-Random-Header-2,X-Some-Random-Header-3"
	handler := Logging(logger, omitHeaders)(&MockHandler{})

	req := httptest.NewRequest("GET", "/dummy", http.NoBody)
	req.Header.Add("X-Some-Random-Header-1", "Some-Random-Value")
	req.Header.Add("X-Some-Random-Header-2", "Some-Random-Value")
	req.Header.Add("X-Some-random-header-3", "Some-Random-Value-Case-Insensitive")
	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	x := b.String()
	if !strings.Contains(x, "X-Some-Random-Header-1") || !strings.Contains(x, "X-Some-Random-Header-2") ||
		strings.Contains(x, "Some-Random-Value") || !strings.Contains(x, "xxx-masked-value-xxx") {
		t.Errorf("error, expected X-Some-Random-Header-1 and X-Some-Random-Header-1 with value :"+
			" xxx-masked-value-xxx, got: %v", x)
	}
}

func TestLoggingAuthorizationHeader(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	handler := Logging(logger, "")(&MockHandler{})

	req := httptest.NewRequest("GET", "/dummy", http.NoBody)
	req.Header.Add("Authorization", "Basic dXNlcjpwYXNz")
	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	// Authorization header should be present
	x := b.String()

	if !strings.Contains(b.String(), "Authorization") || !strings.Contains(b.String(), "user") {
		t.Errorf("error, expected Authorization:user in header, got: %v", x)
	}

	// Authorization header should not be present as the auth token is invalid
	b.Reset()

	req = httptest.NewRequest("GET", "/dummy", http.NoBody)
	req.Header.Add("Authorization", "dummy")
	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	x = b.String()

	if strings.Contains(x, "Authorization") {
		t.Errorf("error, Authorization Header should not be present in logs, got: %v", x)
	}

	// Authorization header should be masked
	b.Reset()

	handler = Logging(logger, "Authorization")(&MockHandler{})

	req = httptest.NewRequest("GET", "/dummy", http.NoBody)
	req.Header.Add("Authorization", "dummy")
	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	x = b.String()

	if !strings.Contains(x, `"Authorization":"xxx-masked-value-xxx"`) {
		t.Errorf("error, Authorization Header should be masked in logs, got: %v", x)
	}
}

func TestAppData(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	handler := Logging(logger, "")(&MockHandler{})

	var appData LogDataKey = "appLogData"

	{
		data := &sync.Map{}
		data.Store("key1", "val1")
		req := httptest.NewRequest("GET", "/dummy", http.NoBody)
		req = req.Clone(context.WithValue(req.Context(), appData, data))

		handler.ServeHTTP(MockWriteHandler{}, req)

		if len(b.Bytes()) == 0 {
			t.Errorf("Failed to write the logs")
		}

		x := b.String()

		if !strings.Contains(b.String(), `"data":{"key1":"val1"}}`) {
			t.Errorf("error, expected \"data\":{\"key1\":\"val1\"},\n got: %v", x)
		}
	}

	{
		b.Reset()
		data := &sync.Map{}
		data.Store("key2", "val2")
		req := httptest.NewRequest("GET", "/dummy", http.NoBody)
		req = req.Clone(context.WithValue(req.Context(), appData, data))

		handler.ServeHTTP(MockWriteHandler{}, req)

		if len(b.Bytes()) == 0 {
			t.Errorf("Failed to write the logs")
		}

		x := b.String()

		if !strings.Contains(b.String(), `"data":{"key2":"val2"}}`) {
			t.Errorf("error, expected \"data\":{\"key2\":\"val2\"}, got: %v", x)
		}
	}
}

func Test_getUsernameForBasicAuth(t *testing.T) {
	type args struct {
		authHeader string
	}

	tests := []struct {
		name     string
		args     args
		wantUser string
		wantPass string
		wantErr  bool
	}{
		{"invalid authHeader", args{authHeader: ""}, "", "", false},
		{"success", args{authHeader: "Basic dXNlcjpwYXNz"}, "user", "pass", false},
		{"invalid token", args{authHeader: "Basic a"}, "", "", true},
		{"failure", args{authHeader: "fail"}, "", "", true},
	}

	for i, tt := range tests {
		gotUser := getUsernameForBasicAuth(tt.args.authHeader)
		assert.Equal(t, tt.wantUser, gotUser, "Test [%d] failed.\n%s", i, tt.name)
	}
}

// Test_ValidAppDataInConcurrentRequest tries to mimic the behavior of ApacheBench(ab)
// test with parameter n=15, c=5
func Test_ValidAppDataInConcurrentRequest(t *testing.T) {
	conReq := 5
	totalReq := 15
	b := new(Buffer)
	logger := log.NewMockLogger(b)
	handler := Logging(logger, "")(&MockHandlerLogging{})
	muxRouter := mux.NewRouter()

	muxRouter.NewRoute().Path("/hello-planet").Methods("GET").Handler(handler)
	muxRouter.NewRoute().Path("/hello-galaxy").Methods("GET").Handler(handler)

	var wg sync.WaitGroup

	batch := totalReq / conReq
	for i := 0; i < batch; i++ {
		wg.Add(1)

		go makeRequestPlanet(t, handler, &wg, "/hello-planet", conReq)
		wg.Add(1)

		go makeRequestGalaxy(t, handler, &wg, "/hello-galaxy", conReq)
		wg.Wait()
	}

	checkLogs(t, b)
}

func TestErrorMessages(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	errorMessage := "test-error"

	err := errors.Response{Reason: errorMessage}

	req := httptest.NewRequest("GET", "/dummy", http.NoBody)
	req = req.Clone(context.WithValue(req.Context(), ErrorMessage, err.Error()))

	handler := Logging(logger, "")(&MockHandler{statusCode: http.StatusInternalServerError})

	handler.ServeHTTP(MockWriteHandler{}, req)

	actual := b.String()

	if !strings.Contains(actual, errorMessage) {
		t.Errorf("FAILED, expected: %v, got: %v", errorMessage, b.String())
	}
}

// TestCookieLogging checks Cookie is getting logged or not.
func TestCookieLogging(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	handler := Logging(logger, "")(&MockHandler{})

	req := httptest.NewRequest("GET", "http://dummy", http.NoBody)
	req.Header.Add("Cookie", "Some-Random-Value")

	handler.ServeHTTP(MockWriteHandler{}, req)

	x := b.String()
	if strings.Contains(x, "Cookie") {
		t.Errorf("Error: Expected no cookie, Got: %v", x)
	}
}

func Test_String(t *testing.T) {
	expOut := `{"correlationId":"","type":"","startTimestamp":"0001-01-01T00:00:00Z",` +
		`"duration":0,"method":"","ip":"","uri":"","responseCode":0,"headers":null,"appData":null}`
	l := LogLine{}
	output := l.String()
	assert.Equal(t, expOut, output, "Test Failed")
}
