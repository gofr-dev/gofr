package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	tests := []struct {
		method           string
		registeredRoutes *[]string
		respBody         string
		respCode         int
		expHeaders       int
	}{
		{http.MethodGet, &[]string{"GET,POST"}, "Sample Response", http.StatusFound, 3},
		{http.MethodOptions, &[]string{"PUT,DELETE,GET,POST"}, "", http.StatusOK, 3},
	}

	for i, tc := range tests {
		handler := CORS(nil, tc.registeredRoutes)(&MockHandlerForCORS{statusCode: http.StatusFound, response: "Sample Response"})

		req := httptest.NewRequest(tc.method, "/hello", http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"), "TEST[%d], Failed.\n", i)
		assert.Equal(t, strings.Join(*tc.registeredRoutes, ", ")+", OPTIONS",
			w.Header().Get("Access-Control-Allow-Methods"), "TEST[%d], Failed.\n", i)
		assert.Equal(t, tc.expHeaders, len(w.Header()), "TEST[%d], Failed.\n", i)
		assert.Equal(t, tc.respCode, w.Code, "TEST[%d], Failed.\n", i)
		assert.Equal(t, tc.respBody, w.Body.String(), "TEST[%d], Failed.\n", i)
	}
}

func TestSetMiddlewareHeaders(t *testing.T) {
	testCases := []struct {
		environmentConfig map[string]string
		registeredRoutes  []string
		expectedHeaders   map[string]string
	}{
		{
			environmentConfig: map[string]string{},
			registeredRoutes:  []string{"GET"},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": "GET, OPTIONS",
			},
		},
		{
			environmentConfig: map[string]string{"Access-Control-Allow-Headers": "clientid"},
			registeredRoutes:  []string{"POST, PUT"},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Headers": allowedHeaders + ", clientid",
				"Access-Control-Allow-Methods": "POST, PUT, OPTIONS",
			},
		},
		{
			environmentConfig: map[string]string{
				"Access-Control-Max-Age":      strconv.Itoa(600),
				"Access-Control-Allow-Origin": "same-origin",
			},
			registeredRoutes: []string{},
			expectedHeaders: map[string]string{
				"Access-Control-Max-Age":       strconv.Itoa(600),
				"Access-Control-Allow-Origin":  "same-origin",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": "OPTIONS",
			},
		},
	}

	for _, tc := range testCases {
		w := httptest.NewRecorder()

		setMiddlewareHeaders(tc.environmentConfig, tc.registeredRoutes, w)

		// Check if the actual headers match the expected headers
		for header, expectedValue := range tc.expectedHeaders {
			actualValue := w.Header().Get(header)
			if actualValue != expectedValue {
				t.Errorf("Header %s: expected %s, got %s", header, expectedValue, actualValue)
			}
		}
	}
}
