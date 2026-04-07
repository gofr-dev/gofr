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
	tests := []struct {
		name             string
		method           string
		origin           string
		config           map[string]string
		registeredRoutes *[]string
		respBody         string
		respCode         int
		expOriginHeader  string
		expVary          string
	}{
		{
			name:             "wildcard GET",
			method:           http.MethodGet,
			registeredRoutes: &[]string{"GET,POST"},
			respBody:         "Sample Response",
			respCode:         http.StatusFound,
			expOriginHeader:  "*",
		},
		{
			name:             "wildcard OPTIONS",
			method:           http.MethodOptions,
			registeredRoutes: &[]string{"PUT,DELETE,GET,POST"},
			respCode:         http.StatusOK,
			expOriginHeader:  "*",
		},
		{
			name:   "multiple origins matched",
			method: http.MethodGet,
			origin: "https://admin.example.com",
			config: map[string]string{
				"Access-Control-Allow-Origin": "https://app.example.com,https://admin.example.com",
			},
			registeredRoutes: &[]string{"GET"},
			respBody:         "Sample Response",
			respCode:         http.StatusFound,
			expOriginHeader:  "https://admin.example.com",
			expVary:          "Origin",
		},
		{
			name:   "multiple origins not matched",
			method: http.MethodGet,
			origin: "https://evil.com",
			config: map[string]string{
				"Access-Control-Allow-Origin": "https://app.example.com,https://admin.example.com",
			},
			registeredRoutes: &[]string{"GET"},
			respBody:         "Sample Response",
			respCode:         http.StatusFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := CORS(tc.config, tc.registeredRoutes)(
				&MockHandlerForCORS{statusCode: http.StatusFound, response: "Sample Response"})

			req := httptest.NewRequest(tc.method, "/hello", http.NoBody)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tc.expOriginHeader, w.Header().Get("Access-Control-Allow-Origin"))
			assert.Equal(t, tc.expVary, w.Header().Get("Vary"))
			assert.Equal(t, tc.respCode, w.Code)
			assert.Equal(t, tc.respBody, w.Body.String())
		})
	}
}

func TestSetMiddlewareHeaders(t *testing.T) {
	testCases := setMiddlewareHeadersTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			setMiddlewareHeaders(tc.environmentConfig, tc.registeredRoutes, w, tc.origin, tc.allowedOrigins)

			for header, expectedValue := range tc.expectedHeaders {
				actualValue := w.Header().Get(header)
				assert.Equal(t, expectedValue, actualValue, "Header %s mismatch", header)
			}
		})
	}
}

func setMiddlewareHeadersTestCases() []struct {
	name              string
	environmentConfig map[string]string
	registeredRoutes  []string
	origin            string
	allowedOrigins    map[string]bool
	expectedHeaders   map[string]string
} {
	return []struct {
		name              string
		environmentConfig map[string]string
		registeredRoutes  []string
		origin            string
		allowedOrigins    map[string]bool
		expectedHeaders   map[string]string
	}{
		{
			name:              "default wildcard",
			environmentConfig: map[string]string{},
			registeredRoutes:  []string{"GET"},
			allowedOrigins:    map[string]bool{"*": true},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": "GET, OPTIONS",
			},
		},
		{
			name:              "custom headers appended",
			environmentConfig: map[string]string{"Access-Control-Allow-Headers": "clientid"},
			registeredRoutes:  []string{"POST, PUT"},
			allowedOrigins:    map[string]bool{"*": true},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Headers": allowedHeaders + ", clientid",
				"Access-Control-Allow-Methods": "POST, PUT, OPTIONS",
			},
		},
		{
			name: "single origin matched with max age",
			environmentConfig: map[string]string{
				"Access-Control-Max-Age":      strconv.Itoa(600),
				"Access-Control-Allow-Origin": "https://example.com",
			},
			registeredRoutes: []string{},
			origin:           "https://example.com",
			allowedOrigins:   map[string]bool{"https://example.com": true},
			expectedHeaders: map[string]string{
				"Access-Control-Max-Age":       strconv.Itoa(600),
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": "OPTIONS",
				"Vary":                         "Origin",
			},
		},
		{
			name: "custom methods override",
			environmentConfig: map[string]string{
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
			},
			registeredRoutes: []string{"GET"},
			allowedOrigins:   map[string]bool{"*": true},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
			},
		},
		{
			name: "multiple origins matched",
			environmentConfig: map[string]string{
				"Access-Control-Allow-Origin": "https://a.com,https://b.com",
			},
			registeredRoutes: []string{"GET"},
			origin:           "https://b.com",
			allowedOrigins:   map[string]bool{"https://a.com": true, "https://b.com": true},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://b.com",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": "GET, OPTIONS",
				"Vary":                         "Origin",
			},
		},
		{
			name: "origin not in allowed set",
			environmentConfig: map[string]string{
				"Access-Control-Allow-Origin": "https://a.com",
			},
			registeredRoutes: []string{"GET"},
			origin:           "https://evil.com",
			allowedOrigins:   map[string]bool{"https://a.com": true},
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "",
				"Access-Control-Allow-Headers": allowedHeaders,
				"Access-Control-Allow-Methods": "GET, OPTIONS",
			},
		},
	}
}

func TestParseOrigins(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:     "empty string defaults to wildcard",
			input:    "",
			expected: map[string]bool{"*": true},
		},
		{
			name:     "wildcard",
			input:    "*",
			expected: map[string]bool{"*": true},
		},
		{
			name:     "single origin",
			input:    "https://example.com",
			expected: map[string]bool{"https://example.com": true},
		},
		{
			name:  "multiple origins",
			input: "https://a.com,https://b.com",
			expected: map[string]bool{
				"https://a.com": true,
				"https://b.com": true,
			},
		},
		{
			name:  "multiple origins with spaces",
			input: "https://a.com, https://b.com , https://c.com",
			expected: map[string]bool{
				"https://a.com": true,
				"https://b.com": true,
				"https://c.com": true,
			},
		},
		{
			name:     "only commas and spaces defaults to wildcard",
			input:    ", , ,",
			expected: map[string]bool{"*": true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseOrigins(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
