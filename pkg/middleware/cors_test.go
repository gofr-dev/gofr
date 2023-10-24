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

// ServeHTTP is used for testing different panic recovery cases
func (r *MockHandlerForCORS) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(r.statusCode)
	_, _ = w.Write([]byte(r.response))
}

func Test_CORS(t *testing.T) {
	handler := CORS(nil)(&MockHandlerForCORS{statusCode: http.StatusFound, response: "Sample Response"})
	testCases := []struct {
		method       string
		responseBody string
		responseCode int
	}{
		{http.MethodGet, "Sample Response", http.StatusFound},
		{http.MethodOptions, "", http.StatusOK},
	}
	expectedHeaders := getValidCORSHeaders(nil)

	for i, testCase := range testCases {
		req := httptest.NewRequest(testCase.method, "/hello", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		for _, header := range AllowedCORSHeader() {
			w.Header().Get("Access-Control-Allow-Credentials")
			assert.Equal(t, expectedHeaders[header], w.Header().Get(header), i)
			w.Header().Del(header)
		}

		// Check if no other headers apart from the allowed headers are being set
		assert.Equal(t, 0, len(w.Header()), i)
		assert.Equal(t, testCase.responseCode, w.Code, i)
		assert.Equal(t, testCase.responseBody, w.Body.String(), i)
	}
}

func Test_getValidCORSHeaders(t *testing.T) {
	testCases := []struct {
		environmentConfig map[string]string
		headers           map[string]string
	}{
		{map[string]string{},
			map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": allowedMethods,
			},
		},
		{map[string]string{
			"Access-Control-Max-Age":       strconv.Itoa(600),
			"Access-Control-Allow-Headers": "",
			"Access-Control-Allow-Origin":  "same-origin",
			"Access-Control-Allow-Methods": http.MethodPost,
		},
			map[string]string{
				"Access-Control-Max-Age":       strconv.Itoa(600),
				"Access-Control-Allow-Origin":  "same-origin",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": http.MethodPost,
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
		{map[string]string{
			"Access-Control-Max-Age":           strconv.Itoa(600),
			"Access-Control-Allow-Methods":     allowedMethods,
			"Some-Random-Header-String":        allowedMethods,
			"Access-Control-Allow-Credentials": "true",
		},
			map[string]string{
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Max-Age":           strconv.Itoa(600),
				"Access-Control-Allow-Origin":      "*",
				"Access-Control-Allow-Headers":     allowedHeaders,
				"Access-Control-Allow-Methods":     allowedMethods,
			},
		},
	}

	for i, testCase := range testCases {
		response := getValidCORSHeaders(testCase.environmentConfig)
		assert.Equal(t, testCase.headers, response, i)
	}
}
