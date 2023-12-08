package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/log"
)

type MockHandlerForPanic struct{}

// ServeHTTP is used for testing different panic recovery cases
func (r *MockHandlerForPanic) ServeHTTP(_ http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/errorPanic":
		// generating a panic case to test recovery for error panics
		var a []int
		a[0] = 1
	case "/stringPanic":
		// generating a panic case to test recovery for error panics
		panic("testing string panic")

	case "/panic":
		// generating a panic case to test recovery for unknown panic type
		panic(1)
	}
}

// TestPanicRecoveryContentType tests the content type for a response in case of Panic
func TestPanicRecoveryContentType(t *testing.T) {
	testcases := []struct {
		reqContentType string
		resContentType string
	}{
		{"", "application/json"},
		{"application/json", "application/json"},
		{"application/xml", "application/xml"},
		{"text/json", "application/json"},
		{"text/plain", "text/plain"},
		{"text/xml", "text/xml"},
	}

	muxRouter := mux.NewRouter().StrictSlash(false)
	logger := log.NewMockLogger(io.Discard)
	handler := Recover(logger)(&MockHandlerForPanic{})

	for _, tc := range testcases {
		req := httptest.NewRequest("GET", "/panic", nil)
		req.Header.Add("Content-Type", tc.reqContentType)

		w := httptest.NewRecorder()

		muxRouter.NewRoute().Path(req.URL.Path).Methods("GET").Handler(handler)
		muxRouter.ServeHTTP(w, req)

		contentType := w.Header().Get("Content-TYpe")

		// Check if Content-Type is set as per the request header or not, in the Response.
		if tc.resContentType != contentType {
			t.Errorf("Expected: %v, Got: %v", tc.resContentType, contentType)
		}
	}
}

// TestPanicRecovery tests the different categories of Panic
func TestPanicRecovery(t *testing.T) {
	testcases := []struct {
		endpoint           string
		expectedLogMessage string
	}{
		{"panic", "Unknown panic type"},
		{"errorPanic", "index out of range"},
		{"stringPanic", "testing string panic"},
	}

	var b bytes.Buffer
	logger := log.NewMockLogger(&b)
	handler := Recover(logger)(&MockHandlerForPanic{})
	w := httptest.NewRecorder()
	muxRouter := mux.NewRouter()
	muxRouter.NewRoute().Handler(handler)

	for _, tc := range testcases {
		req := httptest.NewRequest("GET", "/"+tc.endpoint, nil)
		req = req.Clone(context.WithValue(req.Context(), CorrelationIDKey, "gofrTest"))
		muxRouter.ServeHTTP(w, req)

		if len(b.Bytes()) == 0 {
			t.Errorf("Failed to write log panics")
		}

		// testing level of logging
		if !strings.Contains(b.String(), `"level":"ERROR"`) {
			t.Errorf("Error is not logged for panic")
		}

		if !strings.Contains(b.String(), tc.expectedLogMessage) {
			t.Errorf("logging panic types has failed.")
		}

		// test correlationId in log
		if !strings.Contains(b.String(), `"correlationId":"gofrTest"`) {
			t.Errorf("correlationID is not logged")
		}
	}
}

func TestPanicNewRelicErrorReport(t *testing.T) {
	testcases := []struct {
		endpoint           string
		expectedLogMessage string
	}{
		{"panic", "Unknown panic type"},
	}

	var b bytes.Buffer
	logger := log.NewMockLogger(&b)
	handler := NewRelic("gofr", "6378b0a5bf929e7eb36d480d4e3cd914b74eNRAL")(Recover(logger)(&MockHandlerForPanic{}))
	muxRouter := mux.NewRouter()
	muxRouter.NewRoute().Handler(handler)

	for _, tc := range testcases {
		req := httptest.NewRequest("GET", "/"+tc.endpoint, nil)
		muxRouter.NewRoute().Path(req.URL.Path).Methods("GET").Handler(handler)
		muxRouter.ServeHTTP(httptest.NewRecorder(), req)

		if len(b.Bytes()) == 0 {
			t.Errorf("Failed to write log panics")
		}

		// testing level of logging
		if !strings.Contains(b.String(), `"level":"ERROR"`) {
			t.Errorf("Error is not logged for panic")
		}

		if !strings.Contains(b.String(), tc.expectedLogMessage) {
			t.Errorf("logging panic types has failed.")
		}
	}
}

func TestPanicAppDataLogging(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	handler := Logging(logger, "")(&MockHandler{})

	var appData LogDataKey = "appLogData"

	data := &sync.Map{}
	data.Store("key", "value")

	req := httptest.NewRequest("GET", "/panic", nil)
	req = req.Clone(context.WithValue(req.Context(), appData, data))

	handler.ServeHTTP(MockWriteHandler{}, req)

	if len(b.Bytes()) == 0 {
		t.Errorf("Failed to write the logs")
	}

	x := b.String()

	if !strings.Contains(b.String(), `"data":{"key":"value"}}`) {
		t.Errorf("error, expected \"data\":{\"key\":\"value\"}, got: %v", x)
	}
}

// Buffer is a concurrent safe
type Buffer struct {
	buf   bytes.Buffer
	mutex sync.Mutex
}

// Write appends the contents of p to the buffer, growing the buffer as needed. It returns
// the number of bytes written.
func (s *Buffer) Write(p []byte) (n int, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buf.Write(p)
}

// String returns the contents of the unread portion of the buffer
// as a string.  If the Buffer is a nil pointer, it returns "<nil>".
func (s *Buffer) String() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buf.String()
}

type MockHandlerLogging struct{}

// ServeHTTP is used for testing panic recovery cases for concurrent request
func (r *MockHandlerLogging) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/hello-planet-recover":
		// generating a panic for hello-planet endpoint
		panic("planet")

	case "/hello-galaxy-recover":
		// generating a panic for hello-galaxy endpoint
		panic("galaxy")

	case "/hello-planet":
		// response for hello-planet endpoint
		_, _ = w.Write([]byte("checking log for hello-planet endpoint"))

	case "/hello-galaxy":
		// response for hello-galaxy endpoint
		_, _ = w.Write([]byte("checking log for hello-galaxy endpoint"))
	}
}

func makeRequestPlanet(t *testing.T, handler http.Handler, wg *sync.WaitGroup, target string, n int) {
	defer wg.Done()

	for i := 0; i < n; i++ {
		i := i
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			req := httptest.NewRequest("GET", target, nil)
			data := &sync.Map{}
			data.Store("Planet", "Earth")
			req.Header.Add("X-Correlation-ID", "gofrTest-planet")
			ctx := context.WithValue(context.WithValue(context.Background(), CorrelationIDKey, "gofrTest-planet"),
				LogDataKey("appLogData"), data)
			req = req.Clone(ctx)
			handler.ServeHTTP(&MockWriteHandler{}, req)
		})
	}
}

func makeRequestGalaxy(t *testing.T, handler http.Handler, wg *sync.WaitGroup, target string, n int) {
	defer wg.Done()

	for i := 0; i < n; i++ {
		i := i
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			req := httptest.NewRequest("GET", target, nil)
			data := &sync.Map{}
			data.Store("Galaxy", "MilkyWay")
			req.Header.Add("X-Correlation-ID", "gofrTest-galaxy")
			req = req.Clone(context.WithValue(context.WithValue(context.Background(), CorrelationIDKey, "gofrTest-galaxy"),
				LogDataKey("appLogData"), data))
			handler.ServeHTTP(&MockWriteHandler{}, req)
		})
	}
}

// Test_ValidAppLogDataInConcurrentRequest tries to mimic the behavior of ApacheBench(ab)
// test with parameter n=15, c=5
func Test_ValidAppLogDataInConcurrentRequest(t *testing.T) {
	conReq := 5
	totalReq := 15
	b := new(Buffer)
	logger := log.NewMockLogger(b)
	handler := Recover(logger)(&MockHandlerLogging{})
	muxRouter := mux.NewRouter()

	muxRouter.NewRoute().Path("/hello-planet-recover").Methods("GET").Handler(handler)
	muxRouter.NewRoute().Path("/hello-galaxy-recover").Methods("GET").Handler(handler)

	var wg sync.WaitGroup

	batch := totalReq / conReq
	for i := 0; i < batch; i++ {
		wg.Add(1)

		go makeRequestPlanet(t, muxRouter, &wg, "/hello-planet-recover", conReq)
		wg.Add(1)

		go makeRequestGalaxy(t, muxRouter, &wg, "/hello-galaxy-recover", conReq)
		wg.Wait()
	}

	checkLogs(t, b)
}

func checkLogs(t *testing.T, b *Buffer) {
	if bufLen := len(b.String()); bufLen <= 0 {
		t.Error("Nothing is logged")
	}

	logs := strings.Split(b.String(), "\n")
	for i := 0; i < len(logs)-1; i++ {
		countPlanet := strings.Count(logs[i], `"Planet":"Earth"`)
		countGalaxy := strings.Count(logs[i], `"Galaxy":"MilkyWay"`)

		if (strings.Contains(logs[i], `hello-planet`) && (countGalaxy != 0 || countPlanet != 1)) ||
			(strings.Contains(logs[i], `hello-galaxy`) && (countGalaxy != 1 || countPlanet != 0)) {
			t.Errorf("Error logs is not consistent for concurrent requests:\n %v ", logs[i])
			break
		}
	}
}

func TestGetAppData(t *testing.T) {
	appData := &sync.Map{}
	appData.Store("key1", "value1")
	appData.Store("key2", "value2")

	tests := []struct {
		ctx      context.Context
		expected map[string]interface{}
	}{
		{context.Background(), map[string]interface{}{}}, // no appData
		{context.WithValue(context.Background(), LogDataKey("appLogData"), &sync.Map{}), map[string]interface{}{}},
		{context.WithValue(context.Background(), LogDataKey("appLogData"), appData), map[string]interface{}{"key1": "value1", "key2": "value2"}},
	}

	for i, tc := range tests {
		data := getAppData(tc.ctx)
		assert.Equal(t, tc.expected, data, i)
	}
}
