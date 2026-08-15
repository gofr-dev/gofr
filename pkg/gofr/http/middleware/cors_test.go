package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

			// The fixed headers are now built once per middleware instance
			// rather than per request; build them here the same way CORS does.
			fixed, methods := buildFixedHeaders(tc.environmentConfig, tc.registeredRoutes)
			setMiddlewareHeaders(w, tc.origin, tc.allowedOrigins, fixed, methods)

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

// TestCORSDoesNotMutateRegisteredRoutes pins that building the method list does
// not append into the caller's slice. That slice is the router's
// RegisteredRoutes, and appending in place would write past the caller's length
// whenever capacity exceeded it.
func TestCORSDoesNotMutateRegisteredRoutes(t *testing.T) {
	// A sentinel sits just past the caller's length, in spare capacity: an
	// in-place append would overwrite exactly that element.
	backing := make([]string, 3, 8)
	backing[0], backing[1], backing[2] = http.MethodGet, http.MethodPost, "sentinel"
	routes := backing[:2]

	_, methods := buildFixedHeaders(map[string]string{}, routes)

	require.Equal(t, []string{"GET, POST, OPTIONS"}, methods)
	require.Equal(t, []string{http.MethodGet, http.MethodPost}, routes,
		"the caller's slice must be unchanged")
	require.Equal(t, "sentinel", backing[2], "nothing may be written past the caller's length")
}

// TestCORSHeadersStableAcrossRequests pins that precomputing the fixed headers
// yields the same response headers on every request, including a configured
// override and a pass-through custom header.
func TestCORSHeadersStableAcrossRequests(t *testing.T) {
	routes := []string{http.MethodGet}
	cfg := map[string]string{
		"Access-Control-Allow-Origin": "*",
		"X-Custom-Header":             "custom",
	}

	h := CORS(cfg, &routes)(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	var first http.Header

	for i := range 5 {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody))

		if i == 0 {
			first = w.Header().Clone()

			continue
		}

		require.Equal(t, first, w.Header(), "headers must not drift between requests")
	}

	require.Equal(t, "*", first.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET, OPTIONS", first.Get("Access-Control-Allow-Methods"))
	require.Equal(t, "custom", first.Get("X-Custom-Header"))
	require.Contains(t, first.Get("Access-Control-Allow-Headers"), "Authorization")
}

// TestSharedHeaderValueSurvivesAdd is the safety proof for assigning a shared
// one-element value slice straight into the header map: a later Header.Add must
// append into a new array rather than mutating the slice every response shares.
func TestSharedHeaderValueSurvivesAdd(t *testing.T) {
	routes := []string{http.MethodGet}
	fixed, methods := buildFixedHeaders(map[string]string{}, routes)

	before := append([]string(nil), methods...)

	for range 3 {
		h := http.Header{}
		setMiddlewareHeaders(h2w(h), "", map[string]bool{"*": true}, fixed, methods)

		h.Add("Access-Control-Allow-Methods", "PATCH")
		h.Add("Access-Control-Allow-Headers", "X-Extra")
	}

	require.Equal(t, before, methods, "the shared methods slice must never be mutated")

	for _, f := range fixed {
		require.Len(t, f.value, 1, "a shared header value must stay single-element")
	}
}

// h2w adapts a bare Header to the ResponseWriter setMiddlewareHeaders expects.
type headerOnlyWriter struct{ h http.Header }

func (w headerOnlyWriter) Header() http.Header     { return w.h }
func (headerOnlyWriter) Write([]byte) (int, error) { return 0, nil }
func (headerOnlyWriter) WriteHeader(int)           {}

func h2w(h http.Header) http.ResponseWriter { return headerOnlyWriter{h: h} }
