package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyLogger struct {
	logged []any
}

func (s *spyLogger) Log(args ...any) {
	s.logged = append(s.logged, args...)
}

func TestLog_PrettyPrint(t *testing.T) {
	w := new(bytes.Buffer)

	l := &Log{
		ResponseTime:  100,
		CorrelationID: "abc-test-correlation-id",
		ResponseCode:  200,
		HTTPMethod:    "GET",
		URI:           "/api/test",
	}

	l.PrettyPrint(w)

	assert.Equal(t, "\u001B[38;5;8mabc-test-correlation-id \u001B[38;5;34m200   \u001B[0m      100\u001B[38;5;8mµs\u001B[0m GET /api/test \n",
		w.String())
}

func TestErrorLog_PrettyPrint(t *testing.T) {
	w := new(bytes.Buffer)

	l := &ErrorLog{
		Log: &Log{
			ResponseTime:  100,
			CorrelationID: "abc-test-correlation-id",
			ResponseCode:  200,
			HTTPMethod:    "GET",
			URI:           "/api/test",
		},
		ErrorMessage: "some error occurred",
	}

	l.PrettyPrint(w)

	assert.Equal(t, "\u001B[38;5;8mabc-test-correlation-id \u001B[38;5;34m200   \u001B[0m      100\u001B[38;5;8mµs\u001B[0m GET /api/test \n",
		w.String())
}

func Test_ColorForStatusCode(t *testing.T) {
	testCases := []struct {
		desc   string
		code   int
		expOut int
	}{
		{desc: "200 OK", code: 200, expOut: 34},
		{desc: "201 Created", code: 201, expOut: 34},
		{desc: "400 Bad Request", code: 400, expOut: 220},
		{desc: "409 Conflict", code: 409, expOut: 220},
		{desc: "500 Internal Srv Error", code: 500, expOut: 202},
		{desc: "302 Found has no color", code: 302, expOut: 0},
	}

	for _, tc := range testCases {
		out := colorForStatusCode(tc.code)

		assert.Equal(t, tc.expOut, out)
	}
}

func TestLog_JSON(t *testing.T) {
	l := &Log{
		Timestamp:     time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC),
		ResponseTime:  150,
		LatencyUnit:   "µs",
		CorrelationID: "test-correlation-id",
		ResponseCode:  200,
		HTTPMethod:    "GET",
		URI:           "/api/test",
	}

	data, err := json.Marshal(l)
	require.NoError(t, err)

	var m map[string]any
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)

	assert.Equal(t, "µs", m["latency_unit"], "latency_unit should be µs to match the pretty-printed output")
	assert.Equal(t, float64(150), m["latency"])
	assert.Equal(t, "GET", m["httpMethod"])
	assert.Equal(t, "/api/test", m["uri"])
	assert.Equal(t, float64(200), m["responseCode"])
	assert.Equal(t, "test-correlation-id", m["correlationId"])
}

func TestErrorLog_JSON(t *testing.T) {
	el := &ErrorLog{
		Log: &Log{
			Timestamp:     time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC),
			ResponseTime:  250,
			LatencyUnit:   "µs",
			CorrelationID: "err-correlation-id",
			ResponseCode:  503,
			HTTPMethod:    "POST",
			URI:           "/api/fail",
		},
		ErrorMessage: "service unavailable",
	}

	data, err := json.Marshal(el)
	require.NoError(t, err)

	var m map[string]any
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)

	assert.Equal(t, "µs", m["latency_unit"])
	assert.Equal(t, float64(250), m["latency"])
	assert.Equal(t, "service unavailable", m["errorMessage"])
}

func TestCreateAndSendRequest_LatencyUnit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := &spyLogger{}
	svc := NewHTTPService(server.URL, logger, nil)

	resp, err := svc.Get(t.Context(), "/test", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Len(t, logger.logged, 1)
	entry, ok := logger.logged[0].(*Log)
	require.True(t, ok)
	assert.Equal(t, "µs", entry.LatencyUnit)
}

func TestCreateAndSendRequest_ErrorLog_LatencyUnit(t *testing.T) {
	logger := &spyLogger{}
	svc := NewHTTPService("http://127.0.0.1:1", logger, nil)

	resp, err := svc.Get(t.Context(), "/test", nil)
	require.Error(t, err)
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.Len(t, logger.logged, 1)
	errEntry, ok := logger.logged[0].(*ErrorLog)
	require.True(t, ok)
	require.NotNil(t, errEntry.Log)
	assert.Equal(t, "µs", errEntry.Log.LatencyUnit)
}
