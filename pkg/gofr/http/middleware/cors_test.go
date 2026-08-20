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

// corsBenchWriter is a ResponseWriter that keeps a header map and discards everything else, so a
// benchmark measures the middleware rather than the recorder.
type corsBenchWriter struct{ h http.Header }

func (w *corsBenchWriter) Header() http.Header       { return w.h }
func (*corsBenchWriter) Write(b []byte) (int, error) { return len(b), nil }
func (*corsBenchWriter) WriteHeader(int)             {}

// BenchmarkCORS measures the middleware on the path every request takes: the CORS headers are
// written before the handler runs, on every response, matched or not.
//
// Allocations are the metric that matters here. The header set is constant for the lifetime of the
// server, so building it per request was pure waste.
func BenchmarkCORS(b *testing.B) {
	routes := []string{"GET /users", "POST /users", "GET /users/{id}"}
	h := CORS(map[string]string{}, &routes)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/users", http.NoBody)
	req.Header.Set("Origin", "https://example.com")

	w := &corsBenchWriter{h: make(http.Header, 8)}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		clear(w.h)
		h.ServeHTTP(w, req)
	}
}

// TestCORS_OriginAllowListMatchedByCanonicalKey is the regression test for a
// config that restricted the origin being turned into one that echoed a
// wildcard, and for that same config then overwriting the negotiated header.
//
// buildFixedHeaders classifies each config entry by key and writes everything it
// does not recognize into the fixed set, which is applied AFTER the per-request
// origin negotiation. With a raw-key match, a caller spelling the key
// "access-control-allow-origin" -- CORS is exported, so callers do build this map
// -- was missed by parseOrigins, which fell back to its wildcard default, and was
// routed to the default branch, which canonicalized it straight over the
// negotiated Access-Control-Allow-Origin.
func TestCORS_OriginAllowListMatchedByCanonicalKey(t *testing.T) {
	for _, spelling := range []string{
		"Access-Control-Allow-Origin",
		"access-control-allow-origin",
		"ACCESS-CONTROL-ALLOW-ORIGIN",
		"Access-control-allow-origin",
	} {
		t.Run(spelling, func(t *testing.T) {
			routes := []string{http.MethodGet}
			handler := CORS(map[string]string{spelling: "https://trusted.com"}, &routes)(
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

			unlisted := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			req.Header.Set("Origin", "https://evil.com")
			handler.ServeHTTP(unlisted, req)

			assert.Empty(t, unlisted.Header().Get(headerAccessControlAllowOrigin),
				"an unlisted origin must not be granted access under any spelling of the config key")

			listed := httptest.NewRecorder()
			req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			req.Header.Set("Origin", "https://trusted.com")
			handler.ServeHTTP(listed, req)

			assert.Equal(t, "https://trusted.com", listed.Header().Get(headerAccessControlAllowOrigin),
				"the negotiated origin must survive the fixed-header pass that runs after it")
			assert.Equal(t, "Origin", listed.Header().Get("Vary"))
		})
	}
}

// TestCORS_CaseVariantMethodsAndHeadersAreClassified pins the other two headers
// the classifier owns. A case-variant spelling reaching the default branch would
// REPLACE the derived Allow-Methods, or replace rather than extend the
// framework's required Allow-Headers.
func TestCORS_CaseVariantMethodsAndHeadersAreClassified(t *testing.T) {
	routes := []string{http.MethodGet}
	handler := CORS(map[string]string{
		"access-control-allow-methods": "GET, PATCH",
		"access-control-allow-headers": "clientid",
	}, &routes)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))

	assert.Equal(t, "GET, PATCH", w.Header().Get(headerAccessControlAllowMethods),
		"a configured method list replaces the derived one")
	assert.Equal(t, allowedHeaders+", clientid", w.Header().Get(headerAccessControlAllowHeaders),
		"configured headers extend the framework's required set rather than replacing it")
}

// TestCORS_DuplicateSpellingsResolveDeterministically pins the precedence rule.
// Two spellings of one header are one header, and the map they arrive in has no
// order, so leaving the collision to the classifier let map iteration decide the
// winner -- a different value could be sent on different requests in one process.
func TestCORS_DuplicateSpellingsResolveDeterministically(t *testing.T) {
	routes := []string{http.MethodGet}
	handler := CORS(map[string]string{
		"Access-Control-Allow-Headers": "X-Canonical",
		"access-control-allow-headers": "x-lower",
		"ACCESS-CONTROL-ALLOW-HEADERS": "X-UPPER",
	}, &routes)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for range 50 {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))

		assert.Equal(t, allowedHeaders+", X-Canonical", w.Header().Get(headerAccessControlAllowHeaders),
			"the exactly-canonical spelling must win, on every request")
	}
}

// TestSharedHeaderValuesHaveNoSpareCapacity states the invariant the shared
// value slices rest on, for every slice rather than only the ones a particular
// test happens to touch. A shared slice with room to grow would let one
// response's Header.Add write into every other response's header, process-wide.
func TestSharedHeaderValuesHaveNoSpareCapacity(t *testing.T) {
	routes := []string{http.MethodGet, http.MethodPost}
	fixed, methods := buildFixedHeaders(canonicalizeConfig(map[string]string{
		"Access-Control-Max-Age":       "600",
		"access-control-allow-headers": "clientid",
	}), routes)

	require.Len(t, methods, 1)
	assert.Equal(t, 1, cap(methods), "the shared methods slice must have no spare capacity")

	assert.Equal(t, 1, cap(wildcardOrigin), "the shared wildcard origin must have no spare capacity")

	for _, f := range fixed {
		assert.Len(t, f.value, 1, "a shared header value must stay single-element")
		assert.Equal(t, 1, cap(f.value), "a shared header value must have no spare capacity: "+f.key)
	}
}

// BenchmarkCORS_NamedOrigin measures the branch BenchmarkCORS does not reach.
// An empty config takes the wildcard fast path, where the value slice is shared
// and nothing allocates; a named-origin config still goes through Header.Set and
// Vary's Header.Add, so the "→ 0 allocs" claim does not hold for it.
func BenchmarkCORS_NamedOrigin(b *testing.B) {
	routes := []string{"GET /users", "POST /users", "GET /users/{id}"}
	h := CORS(map[string]string{
		headerAccessControlAllowOrigin: "https://a.example.com, https://b.example.com",
	}, &routes)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/users", http.NoBody)
	req.Header.Set("Origin", "https://b.example.com")

	w := &corsBenchWriter{h: make(http.Header, 8)}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		clear(w.h)
		h.ServeHTTP(w, req)
	}
}

// TestCORS_WildcardPathIsAllocationFree guards the headline claim with a number
// rather than a benchmark reading, so a regression fails the suite instead of
// quietly showing up in a table nobody diffs.
func TestCORS_WildcardPathIsAllocationFree(t *testing.T) {
	routes := []string{"GET /users", "POST /users"}
	h := CORS(map[string]string{}, &routes)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users", http.NoBody)
	req.Header.Set("Origin", "https://example.com")

	w := &corsBenchWriter{h: make(http.Header, 8)}

	// Warm the sync.Once so first-request construction is not counted.
	h.ServeHTTP(w, req)

	allocs := testing.AllocsPerRun(200, func() {
		clear(w.h)
		h.ServeHTTP(w, req)
	})

	assert.Zero(t, allocs, "the wildcard path must write its headers without allocating")
}
