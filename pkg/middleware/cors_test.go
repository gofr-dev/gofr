package middleware

import (
	"fmt"
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
	corsMapping := map[string]string{
		"ACCESS_CONTROL_ALLOW_HEADERS":     "Access-Control-Allow-Headers",
		"ACCESS_CONTROL_ALLOW_METHODS":     "Access-Control-Allow-Methods",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS": "Access-Control-Allow-Credentials",
		"ACCESS_CONTROL_EXPOSE_HEADERS":    "Access-Control-Expose-Headers",
		"ACCESS_CONTROL_MAX_AGE":           "Access-Control-Max-Age",
		"ACCESS_CONTROL_ALLOW_ORIGIN":      "Access-Control-Allow-Origin",
	}

	handler := CORS(nil)(&MockHandlerForCORS{statusCode: http.StatusFound, response: "Sample Response"})
	testCases := []struct {
		method       string
		responseBody string
		responseCode int
	}{
		{http.MethodGet, "Sample Response", http.StatusFound},
		{http.MethodOptions, "", http.StatusOK},
	}
	expectedHeaders := getValidCORSHeaders(nil, corsMapping)

	for i, testCase := range testCases {
		req := httptest.NewRequest(testCase.method, "/hello", http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		for _, header := range corsMapping {
			w.Header().Get("Access-Control-Allow-Credentials")
			fmt.Println(expectedHeaders[header])
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
	corsMapping := map[string]string{
		"ACCESS_CONTROL_ALLOW_HEADERS":     "Access-Control-Allow-Headers",
		"ACCESS_CONTROL_ALLOW_METHODS":     "Access-Control-Allow-Methods",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS": "Access-Control-Allow-Credentials",
		"ACCESS_CONTROL_EXPOSE_HEADERS":    "Access-Control-Expose-Headers",
		"ACCESS_CONTROL_MAX_AGE":           "Access-Control-Max-Age",
		"ACCESS_CONTROL_ALLOW_ORIGIN":      "Access-Control-Allow-Origin",
	}

	testCases := []struct {
		environmentConfig map[string]string
		headers           map[string]string
	}{
		{map[string]string{},
			map[string]string{
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": allowedMethods,
			},
		},
		{map[string]string{
			"ACCESS_CONTROL_MAX_AGE":       strconv.Itoa(600),
			"ACCESS_CONTROL_ALLOW_HEADERS": "",
			"ACCESS_CONTROL_ALLOW_METHODS": http.MethodPost,
			"ACCESS_CONTROL_ALLOW_ORIGIN":  "abc.com",
		},
			map[string]string{
				"Access-Control-Max-Age":       strconv.Itoa(600),
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": http.MethodPost,
				"Access-Control-Allow-Origin":  "abc.com",
			},
		},
		{map[string]string{
			"ACCESS_CONTROL_ALLOW_HEADERS": "clientid",
			"ACCESS_CONTROL_ALLOW_ORIGIN":  "abc.com",
		},
			map[string]string{
				"Access-Control-Allow-Headers": allowedHeaders + ", clientid",
				"Access-Control-Allow-Methods": allowedMethods,
				"Access-Control-Allow-Origin":  "abc.com",
			},
		},
		{map[string]string{
			"ACCESS_CONTROL_MAX_AGE":           strconv.Itoa(600),
			"ACCESS_CONTROL_ALLOW_METHODS":     allowedMethods,
			"Some-Random-Header-String":        allowedMethods,
			"ACCESS_CONTROL_ALLOW_CREDENTIALS": "true",
			"ACCESS_CONTROL_ALLOW_ORIGIN":      "abc.com",
		},
			map[string]string{
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Max-Age":           strconv.Itoa(600),
				"Access-Control-Allow-Headers":     allowedHeaders,
				"Access-Control-Allow-Methods":     allowedMethods,
				"Access-Control-Allow-Origin":      "abc.com",
			},
		},
	}

	for i, testCase := range testCases {
		response := getValidCORSHeaders(testCase.environmentConfig, corsMapping)
		assert.Equal(t, testCase.headers, response, i)
	}
}
