package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockHandlerForCORS struct {
	statusCode int
	response   string
}

// ServeHTTP is used for testing different panic recovery cases.
func (r *MockHandlerForCORS) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(r.statusCode)
	_, _ = w.Write([]byte(r.response))
}

func Test_CORS(t *testing.T) {
	handler := CORS(nil)(&MockHandlerForCORS{statusCode: http.StatusFound, response: "Sample Response"})

	tests := []struct {
		method     string
		respBody   string
		respCode   int
		expHeaders int
	}{
		{http.MethodGet, "Sample Response", http.StatusFound, 3},
		{http.MethodOptions, "", http.StatusOK, 3},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(tc.method, "/hello", http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"), "TEST[%d], Failed.\n", i)
		assert.Equal(t, "PUT, POST, GET, DELETE, OPTIONS, PATCH", w.Header().Get("Access-Control-Allow-Methods"), "TEST[%d], Failed.\n", i)
		assert.Equal(t, tc.expHeaders, len(w.Header()), "TEST[%d], Failed.\n", i)
		assert.Equal(t, tc.respCode, w.Code, "TEST[%d], Failed.\n", i)
		assert.Equal(t, tc.respBody, w.Body.String(), "TEST[%d], Failed.\n", i)
	}
}

func TestSetMiddlewareHeaders(t *testing.T) {
	testCases := []struct {
		environmentConfig map[string]string
		expectedHeaders   map[string]string
	}{
		{
			environmentConfig: map[string]string{},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": allowedMethods,
			},
		},
		{map[string]string{
			"Access-Control-Allow-Headers": "clientid",
		},
			map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Headers": allowedHeaders + ", clientid",
				"Access-Control-Allow-Methods": allowedMethods,
			},
		},
		{
			environmentConfig: map[string]string{
				"Access-Control-Max-Age":       strconv.Itoa(600),
				"Access-Control-Allow-Origin":  "same-origin",
				"Access-Control-Allow-Methods": http.MethodPost,
			},
			expectedHeaders: map[string]string{
				"Access-Control-Max-Age":       strconv.Itoa(600),
				"Access-Control-Allow-Origin":  "same-origin",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": http.MethodPost,
			},
		},
	}

	for _, tc := range testCases {
		w := httptest.NewRecorder()

		setMiddlewareHeaders(tc.environmentConfig, w)

		// Check if the actual headers match the expected headers
		for header, expectedValue := range tc.expectedHeaders {
			actualValue := w.Header().Get(header)
			if actualValue != expectedValue {
				t.Errorf("Header %s: expected %s, got %s", header, expectedValue, actualValue)
			}
		}
	}
}
