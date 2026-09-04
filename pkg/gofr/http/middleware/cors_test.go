package middleware

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// corsLifecycleServe issues one request through h and returns the response headers.
func corsLifecycleServe(t *testing.T, h http.Handler, origin string) http.Header {
	t.Helper()

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)

	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	h.ServeHTTP(w, req)

	return w.Header()
}

func corsLifecycleNoop() http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
}

// TestCORSLifecycleContract pins WHEN the configuration and the route set are
// read, which is the one behavioral difference this PR introduces against the
// per-request build it replaces.
//
// The contract is a single rule: BOTH inputs are read exactly once, on the first
// request, and never again. The rule matters because the two halves used to
// freeze at different instants -- the config at construction, the routes at the
// first request -- a split nothing documented and nobody would expect. Anything
// completed before the first request is therefore still honored, which is what
// this test covers; TestCORSLifecycleFrozenAfterFirstRequest covers the other
// half.
func TestCORSLifecycleContract(t *testing.T) {
	t.Run("config completed before the first request is observed", func(t *testing.T) {
		cfg := map[string]string{}
		routes := []string{http.MethodGet}
		h := CORS(cfg, &routes)(corsLifecycleNoop())

		cfg["Access-Control-Max-Age"] = "99"
		cfg[headerAccessControlAllowOrigin] = "https://late.com"

		got := corsLifecycleServe(t, h, "https://late.com")

		assert.Equal(t, "99", got.Get("Access-Control-Max-Age"),
			"a config completed after CORS() but before serving must still be honored")
		assert.Equal(t, "https://late.com", got.Get(headerAccessControlAllowOrigin),
			"the allow-list must be built from the same snapshot as the fixed headers")
	})

	t.Run("routes appended before the first request are observed", func(t *testing.T) {
		routes := make([]string, 0, 3)
		routes = append(routes, http.MethodGet)

		h := CORS(map[string]string{}, &routes)(corsLifecycleNoop())

		// Exactly what handler registration does: finish the route list, then serve.
		routes = append(routes, http.MethodPatch, http.MethodDelete)

		assert.Equal(t, "GET, PATCH, DELETE, OPTIONS",
			corsLifecycleServe(t, h, "").Get(headerAccessControlAllowMethods))
	})

	t.Run("routes replaced through the pointer before serving are observed", func(t *testing.T) {
		// gofr.go assigns rather than appends: *RegisteredRoutes = registeredMethods.
		routes := []string{http.MethodGet}
		h := CORS(map[string]string{}, &routes)(corsLifecycleNoop())

		routes = []string{http.MethodPost, http.MethodPut}

		assert.Equal(t, "POST, PUT, OPTIONS",
			corsLifecycleServe(t, h, "").Get(headerAccessControlAllowMethods))
	})
}

// TestCORSLifecycleFrozenAfterFirstRequest pins the other half of the contract.
//
// Mutating either input after the first request is served is unsupported, and
// deliberately so: both are read from every serving goroutine, so a caller
// mutating them concurrently races regardless of when the read happens. Freezing
// makes that race impossible rather than merely unlikely. GoFr never does it --
// GetConfigs builds the map fully before handing it to CORS, and httpServerSetup
// assigns the final route list synchronously before the serve goroutine starts.
func TestCORSLifecycleFrozenAfterFirstRequest(t *testing.T) {
	t.Run("both inputs are frozen once a request has been served", func(t *testing.T) {
		cfg := map[string]string{"Access-Control-Max-Age": "600"}
		routes := make([]string, 0, 2)
		routes = append(routes, http.MethodGet)

		h := CORS(cfg, &routes)(corsLifecycleNoop())

		before := corsLifecycleServe(t, h, "")
		require.Equal(t, "600", before.Get("Access-Control-Max-Age"))
		require.Equal(t, "GET, OPTIONS", before.Get(headerAccessControlAllowMethods))

		cfg["Access-Control-Max-Age"] = "1"
		cfg[headerAccessControlAllowHeaders] = "clientid"

		routes = append(routes, http.MethodDelete)

		after := corsLifecycleServe(t, h, "")

		assert.Equal(t, "600", after.Get("Access-Control-Max-Age"),
			"config is frozen after the first request")
		assert.Equal(t, allowedHeaders, after.Get(headerAccessControlAllowHeaders),
			"config is frozen after the first request")
		assert.Equal(t, "GET, OPTIONS", after.Get(headerAccessControlAllowMethods),
			"routes are frozen after the first request")
	})

	t.Run("the freeze happens exactly once under concurrent first requests", func(t *testing.T) {
		routes := []string{http.MethodGet, http.MethodPost}
		h := CORS(map[string]string{"Access-Control-Max-Age": "600"}, &routes)(corsLifecycleNoop())

		const n = 200

		var (
			wg   sync.WaitGroup
			mu   sync.Mutex
			seen = map[string]int{}
		)

		for range n {
			wg.Add(1)

			go func() {
				defer wg.Done()

				got := corsLifecycleServe(t, h, "").Get(headerAccessControlAllowMethods)

				mu.Lock()
				seen[got]++
				mu.Unlock()
			}()
		}

		wg.Wait()

		assert.Equal(t, map[string]int{"GET, POST, OPTIONS": n}, seen,
			"every concurrent first request must observe the same fully-built header set")
	})
}

// Characterization suite: pins the EXACT observable output of the CORS
// middleware. Every helper/type/const below is prefixed with `corsChar` and
// every test with `Test_CORSContract` to stay collision-free.
//
// These tests describe CURRENT behavior, including behavior that looks like a
// latent bug. They must be updated only when a behavior change is intentional.
// ---------------------------------------------------------------------------

// corsCharDefaultAllowHeaders is the literal Access-Control-Allow-Headers
// value GoFr puts on the wire. It is spelled out here on purpose rather than
// referencing the production `allowedHeaders` constant: a characterization
// test that reuses the constant under test would silently follow any edit to
// it. Test_CORSContract_DefaultAllowHeadersLiteral asserts the two agree, so
// changing the production spelling (including its casing or its comma-space
// separators) fails loudly here.
const corsCharDefaultAllowHeaders = "Authorization, Content-Type, x-requested-with, " +
	"origin, true-client-ip, X-Correlation-ID"

// Test_CORSContract_DefaultAllowHeadersLiteral pins the exact bytes of the
// default Access-Control-Allow-Headers value.

// Test_CORSContract_DefaultAllowHeadersLiteral pins the exact bytes of the
// default Access-Control-Allow-Headers value.
func Test_CORSContract_DefaultAllowHeadersLiteral(t *testing.T) {
	assert.Equal(t, corsCharDefaultAllowHeaders, allowedHeaders)
}

const (
	corsCharBody             = "Sample Response"
	corsCharOriginA          = "https://a.com"
	corsCharOriginB          = "https://b.com"
	corsCharOriginEvil       = "https://evil.com"
	corsCharKeyOrigin        = "Access-Control-Allow-Origin"
	corsCharKeyMethods       = "Access-Control-Allow-Methods"
	corsCharKeyHeaders       = "Access-Control-Allow-Headers"
	corsCharAllowHeadersLine = corsCharKeyHeaders + ": " + corsCharDefaultAllowHeaders
	corsCharVaryLine         = "Vary: Origin"
)

// corsCharSpyHandler records whether the inner handler was reached.

// corsCharSpyHandler records whether the inner handler was reached.
type corsCharSpyHandler struct {
	called int
}

func (h *corsCharSpyHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.called++

	w.WriteHeader(http.StatusFound)
	_, _ = w.Write([]byte(corsCharBody))
}

// corsCharHeaderLines renders a whole http.Header into a deterministic, sorted
// slice of "Key: v1, v2" lines so a full snapshot can be compared exactly.

// corsCharHeaderLines renders a whole http.Header into a deterministic, sorted
// slice of "Key: v1, v2" lines so a full snapshot can be compared exactly.
func corsCharHeaderLines(h http.Header) []string {
	lines := make([]string, 0, len(h))
	for k, v := range h {
		lines = append(lines, k+": "+strings.Join(v, ", "))
	}

	sort.Strings(lines)

	return lines
}

func corsCharSorted(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)

	return out
}

func corsCharMethodsLine(v string) string { return corsCharKeyMethods + ": " + v }

func corsCharOriginLine(v string) string { return corsCharKeyOrigin + ": " + v }

// corsCharRun drives the middleware once and returns the recorder plus the spy.

// corsCharRun drives the middleware once and returns the recorder plus the spy.
func corsCharRun(t *testing.T, cfg map[string]string, routes *[]string,
	method, origin string,
) (*httptest.ResponseRecorder, *corsCharSpyHandler) {
	t.Helper()

	spy := &corsCharSpyHandler{}
	handler := CORS(cfg, routes)(spy)

	req := httptest.NewRequestWithContext(t.Context(), method, "/hello", http.NoBody)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	return w, spy
}

type corsCharCase struct {
	name     string
	config   map[string]string
	method   string
	origin   string
	routes   []string
	expLines []string
	expCode  int
	expBody  string
	expInner int
}

func corsCharBaselineCases() []corsCharCase {
	twoRoutes := []string{http.MethodGet, http.MethodPost}
	baseMethods := corsCharMethodsLine("GET, POST, OPTIONS")

	return []corsCharCase{
		{
			name: "nil config GET no origin", config: nil, method: http.MethodGet,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name: "empty config POST no origin", config: map[string]string{}, method: http.MethodPost,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name: "empty config OPTIONS short circuits", config: map[string]string{}, method: http.MethodOptions,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusOK, expBody: "", expInner: 0,
		},
		{
			name:   "empty origin config value falls back to wildcard",
			config: map[string]string{corsCharKeyOrigin: ""}, method: http.MethodGet, origin: corsCharOriginA,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "explicit wildcard never adds Vary",
			config: map[string]string{corsCharKeyOrigin: "*"}, method: http.MethodGet, origin: corsCharOriginA,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "wildcard OPTIONS with origin",
			config: map[string]string{corsCharKeyOrigin: "*"}, method: http.MethodOptions, origin: corsCharOriginA,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusOK, expBody: "", expInner: 0,
		},
	}
}

func corsCharOriginMatchingCases() []corsCharCase {
	twoRoutes := []string{http.MethodGet, http.MethodPost}
	baseMethods := corsCharMethodsLine("GET, POST, OPTIONS")

	return []corsCharCase{
		{
			name:   "single origin matched adds Vary",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA}, method: http.MethodGet, origin: corsCharOriginA,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine(corsCharOriginA), corsCharVaryLine},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "single origin not matched drops origin and Vary",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA}, method: http.MethodGet, origin: corsCharOriginEvil,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "single origin with no Origin request header drops origin",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA}, method: http.MethodGet,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "single origin OPTIONS not matched still short circuits",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA}, method: http.MethodOptions, origin: corsCharOriginEvil,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods},
			expCode:  http.StatusOK, expBody: "", expInner: 0,
		},
		{
			name:   "comma list without spaces",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA + "," + corsCharOriginB},
			method: http.MethodGet, origin: corsCharOriginB, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine(corsCharOriginB), corsCharVaryLine},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "comma list with spaces",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA + " , " + corsCharOriginB},
			method: http.MethodGet, origin: corsCharOriginA, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine(corsCharOriginA), corsCharVaryLine},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "comma list with empty entry is skipped",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA + ", ," + corsCharOriginB},
			method: http.MethodGet, origin: corsCharOriginB, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine(corsCharOriginB), corsCharVaryLine},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "only separators degrades to wildcard",
			config: map[string]string{corsCharKeyOrigin: ", , ,"}, method: http.MethodGet, origin: corsCharOriginEvil,
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "origin match is exact and untrimmed",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA}, method: http.MethodGet, origin: corsCharOriginA + " ",
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "origin match is case sensitive",
			config: map[string]string{corsCharKeyOrigin: corsCharOriginA}, method: http.MethodGet, origin: "HTTPS://A.COM",
			routes:   twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
	}
}

func corsCharCustomHeaderCases() []corsCharCase {
	twoRoutes := []string{http.MethodGet, http.MethodPost}
	baseMethods := corsCharMethodsLine("GET, POST, OPTIONS")

	return []corsCharCase{
		{
			name:   "custom allow-headers is concatenated onto the defaults",
			config: map[string]string{corsCharKeyHeaders: "clientid"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine + ", clientid", baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "empty custom allow-headers keeps the defaults",
			config: map[string]string{corsCharKeyHeaders: ""}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "custom allow-methods fully replaces routes derived value",
			config: map[string]string{corsCharKeyMethods: "GET, PUT"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, corsCharMethodsLine("GET, PUT"), corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "empty custom allow-methods keeps routes derived value",
			config: map[string]string{corsCharKeyMethods: ""}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name: "credentials max-age and expose-headers pass through",
			config: map[string]string{
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Max-Age":           "600",
				"Access-Control-Expose-Headers":    "X-Foo, X-Bar",
			},
			method: http.MethodGet, routes: twoRoutes,
			expLines: []string{
				"Access-Control-Allow-Credentials: true",
				corsCharAllowHeadersLine,
				baseMethods,
				corsCharOriginLine("*"),
				"Access-Control-Expose-Headers: X-Foo, X-Bar",
				"Access-Control-Max-Age: 600",
			},
			expCode: http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "empty valued custom header is still emitted with an empty value",
			config: map[string]string{"Access-Control-Max-Age": ""}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{
				corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*"), "Access-Control-Max-Age: ",
			},
			expCode: http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name: "everything combined",
			config: map[string]string{
				corsCharKeyOrigin:                  corsCharOriginA + ", " + corsCharOriginB,
				corsCharKeyHeaders:                 "clientid",
				corsCharKeyMethods:                 "GET, DELETE",
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Max-Age":           "86400",
			},
			method: http.MethodOptions, origin: corsCharOriginB, routes: twoRoutes,
			expLines: []string{
				"Access-Control-Allow-Credentials: true",
				corsCharAllowHeadersLine + ", clientid",
				corsCharMethodsLine("GET, DELETE"),
				corsCharOriginLine(corsCharOriginB),
				"Access-Control-Max-Age: 86400",
				corsCharVaryLine,
			},
			expCode: http.StatusOK, expBody: "", expInner: 0,
		},
	}
}

// corsCharGarbageKeyCases pins what happens for config keys that are not the
// canonical, exactly-cased header names the implementation compares against.

// corsCharGarbageKeyCases pins what happens for config keys that are not the
// canonical, exactly-cased header names the implementation compares against.
func corsCharGarbageKeyCases() []corsCharCase {
	twoRoutes := []string{http.MethodGet, http.MethodPost}
	baseMethods := corsCharMethodsLine("GET, POST, OPTIONS")

	return []corsCharCase{
		{
			name:   "arbitrary key is blindly set and canonicalized",
			config: map[string]string{"x-garbage": "boom"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*"), "X-Garbage: boom"},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "key with a space is not canonicalizable and is stored verbatim",
			config: map[string]string{"Bad Key": "v"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, "Bad Key: v", baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "underscore is a token char so only the first letter is upper cased",
			config: map[string]string{"x_under_score": "v"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*"), "X_under_score: v"},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			// A differently-cased allow-headers key EXTENDS the defaults, exactly
			// like the canonical spelling. Matching the raw key previously made
			// it miss the concat branch and replace the list instead, silently
			// dropping Authorization, Content-Type and X-Correlation-ID.
			name:   "lower cased allow-headers key extends the defaults, like the canonical key",
			config: map[string]string{"access-control-allow-headers": "only-this"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{
				corsCharKeyHeaders + ": " + corsCharDefaultAllowHeaders + ", only-this",
				baseMethods, corsCharOriginLine("*"),
			},
			expCode: http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "upper cased allow-methods key overwrites the routes derived value",
			config: map[string]string{"ACCESS-CONTROL-ALLOW-METHODS": "TRACE"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharAllowHeadersLine, corsCharMethodsLine("TRACE"), corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name:   "mixed case max-age key is canonicalized",
			config: map[string]string{"ACCESS-CONTROL-MAX-AGE": "60"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{
				corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*"), "Access-Control-Max-Age: 60",
			},
			expCode: http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		// The two cases below guard the origin-override fix. A config key is
		// compared on its canonical header form, so a differently-cased
		// spelling can no longer reach Access-Control-Allow-Origin through the
		// custom-header loop and replace the negotiated value.
		{
			name:   "lower cased origin key configures the allow-list and emits nothing for an unlisted origin",
			config: map[string]string{"access-control-allow-origin": corsCharOriginEvil}, method: http.MethodGet, routes: twoRoutes,
			// The key is folded to its canonical spelling, so this IS the
			// allow-list — it restricts origins to corsCharOriginEvil. The
			// request carries no Origin, which is not on that list, so nothing
			// is negotiated and the custom loop must not emit one of its own.
			//
			// This case previously expected "*". That was the bug: the
			// allow-list was read with a raw literal lookup and missed the key
			// entirely, so parseOrigins fell back to its wildcard default and a
			// config that restricted the origin echoed "*" instead.
			expLines: []string{corsCharAllowHeadersLine, baseMethods},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
		{
			name: "lower cased origin key cannot overwrite a properly negotiated origin",
			config: map[string]string{
				corsCharKeyOrigin:             corsCharOriginA,
				"access-control-allow-origin": corsCharOriginEvil,
			},
			method: http.MethodGet, origin: corsCharOriginA, routes: twoRoutes,
			expLines: []string{
				corsCharAllowHeadersLine, baseMethods, corsCharOriginLine(corsCharOriginA), corsCharVaryLine,
			},
			expCode: http.StatusFound, expBody: corsCharBody, expInner: 1,
		},
	}
}

func corsCharAllCases() []corsCharCase {
	cases := corsCharBaselineCases()
	cases = append(cases, corsCharOriginMatchingCases()...)
	cases = append(cases, corsCharCustomHeaderCases()...)
	cases = append(cases, corsCharGarbageKeyCases()...)

	return cases
}

func Test_CORSContract_ResponseSnapshot(t *testing.T) {
	cases := corsCharAllCases()

	for i := range cases {
		tc := &cases[i]

		t.Run(tc.name, func(t *testing.T) {
			routes := tc.routes
			w, spy := corsCharRun(t, tc.config, &routes, tc.method, tc.origin)

			assert.Equal(t, corsCharSorted(tc.expLines), corsCharHeaderLines(w.Header()))
			assert.Equal(t, tc.expCode, w.Code)
			assert.Equal(t, tc.expBody, w.Body.String())
			assert.Equal(t, tc.expInner, spy.called)
		})
	}
}

func Test_CORSContract_AllowMethodsJoin(t *testing.T) {
	cases := []struct {
		name   string
		routes []string
		exp    string
	}{
		{name: "nil slice", routes: nil, exp: "OPTIONS"},
		{name: "empty slice", routes: []string{}, exp: "OPTIONS"},
		{name: "single element", routes: []string{http.MethodGet}, exp: "GET, OPTIONS"},
		{name: "two elements", routes: []string{http.MethodGet, http.MethodPut}, exp: "GET, PUT, OPTIONS"},
		{name: "element already containing commas", routes: []string{"GET,POST"}, exp: "GET,POST, OPTIONS"},
		{name: "element already containing OPTIONS is duplicated", routes: []string{"OPTIONS"}, exp: "OPTIONS, OPTIONS"},
		{name: "empty string element", routes: []string{""}, exp: ", OPTIONS"},
		{name: "whitespace preserved", routes: []string{" GET "}, exp: " GET , OPTIONS"},
	}

	for i := range cases {
		tc := &cases[i]

		t.Run(tc.name, func(t *testing.T) {
			routes := tc.routes
			w, _ := corsCharRun(t, nil, &routes, http.MethodGet, "")

			assert.Equal(t, corsCharSorted([]string{
				corsCharAllowHeadersLine, corsCharMethodsLine(tc.exp), corsCharOriginLine("*"),
			}), corsCharHeaderLines(w.Header()))
		})
	}
}

// Test_CORSContract_RoutesReadAtRequestTime pins that the routes slice is
// dereferenced per request, so routes registered after the middleware was
// constructed do show up in Access-Control-Allow-Methods.

// Test_CORSContract_RoutesBackingArrayAliasing guards against writing through
// the caller's route slice. The header used to be built with
// `routes = append(routes, "OPTIONS")` on a copy of the dereferenced slice
// header, so whenever cap > len the append stored "OPTIONS" into the CALLER's
// backing array at index len. That was invisible through the caller's own slice
// (its length is unchanged) but visible through any longer alias — here,
// SENTINEL-1 would be clobbered.
func Test_CORSContract_RoutesBackingArrayAliasing(t *testing.T) {
	backing := []string{http.MethodGet, "SENTINEL-1", "SENTINEL-2", "SENTINEL-3"}
	routes := backing[:1]
	require.Equal(t, 4, cap(routes), "precondition: cap must exceed len for aliasing to be observable")

	w, _ := corsCharRun(t, nil, &routes, http.MethodGet, "")
	require.Equal(t, "GET, OPTIONS", w.Header().Get(corsCharKeyMethods))

	// The middleware must not write through the shared backing array. It used to
	// build the header with append(routes, "OPTIONS"), which stores into the
	// caller's array whenever cap > len — silently replacing the element after
	// the caller's length. Here that would clobber SENTINEL-1.
	assert.Equal(t, []string{http.MethodGet}, routes, "caller's slice must be untouched")
	assert.Equal(t, []string{http.MethodGet, "SENTINEL-1", "SENTINEL-2", "SENTINEL-3"}, backing,
		"the caller's backing array must be left intact")

	// A caller-side append still behaves normally afterwards, and the next
	// request reflects the newly registered route.
	routes = append(routes, http.MethodPost)
	assert.Equal(t, []string{http.MethodGet, http.MethodPost}, routes)

	w2, _ := corsCharRun(t, nil, &routes, http.MethodGet, "")
	assert.Equal(t, "GET, POST, OPTIONS", w2.Header().Get(corsCharKeyMethods))
	assert.Equal(t, "SENTINEL-2", backing[2], "second request must not clobber the array either")
}
func Test_CORSContract_CustomHeaderLoopIsOrderIndependent(t *testing.T) {
	cfg := map[string]string{
		corsCharKeyOrigin:                  corsCharOriginA,
		corsCharKeyHeaders:                 "clientid",
		corsCharKeyMethods:                 "GET, PATCH",
		"Access-Control-Allow-Credentials": "true",
		"Access-Control-Expose-Headers":    "X-A, X-B",
		"Access-Control-Max-Age":           "600",
		"X-Custom-One":                     "1",
		"X-Custom-Two":                     "2",
		"x-custom-three":                   "3",
	}

	expected := corsCharSorted([]string{
		"Access-Control-Allow-Credentials: true",
		corsCharAllowHeadersLine + ", clientid",
		corsCharMethodsLine("GET, PATCH"),
		corsCharOriginLine(corsCharOriginA),
		"Access-Control-Expose-Headers: X-A, X-B",
		"Access-Control-Max-Age: 600",
		"X-Custom-One: 1",
		"X-Custom-Three: 3",
		"X-Custom-Two: 2",
		corsCharVaryLine,
	})

	for range 50 {
		routes := []string{http.MethodGet}
		w, _ := corsCharRun(t, cfg, &routes, http.MethodGet, corsCharOriginA)

		require.Equal(t, expected, corsCharHeaderLines(w.Header()))
	}
}

// Test_CORSContract_VaryIsAddedNotSet pins that Vary is added once per request
// and that repeated requests on fresh recorders never accumulate values.

// Test_CORSContract_VaryIsAddedNotSet pins that Vary is added once per request
// and that repeated requests on fresh recorders never accumulate values.
func Test_CORSContract_VaryIsAddedNotSet(t *testing.T) {
	cfg := map[string]string{corsCharKeyOrigin: corsCharOriginA}
	routes := []string{http.MethodGet}
	handler := CORS(cfg, &routes)(&corsCharSpyHandler{})

	for range 3 {
		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody)
		req.Header.Set("Origin", corsCharOriginA)
		handler.ServeHTTP(w, req)

		assert.Equal(t, []string{"Origin"}, w.Header().Values("Vary"))
	}
}

// Test_CORSContract_VaryAccumulatesOnSharedResponseWriter pins that the
// middleware uses Add (not Set) for Vary, so a pre-existing Vary value is
// preserved and appended to.

// Test_CORSContract_VaryAccumulatesOnSharedResponseWriter pins that the
// middleware uses Add (not Set) for Vary, so a pre-existing Vary value is
// preserved and appended to.
func Test_CORSContract_VaryAccumulatesOnSharedResponseWriter(t *testing.T) {
	cfg := map[string]string{corsCharKeyOrigin: corsCharOriginA}
	routes := []string{http.MethodGet}
	handler := CORS(cfg, &routes)(&corsCharSpyHandler{})

	w := httptest.NewRecorder()
	w.Header().Add("Vary", "Accept-Encoding")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody)
	req.Header.Set("Origin", corsCharOriginA)
	handler.ServeHTTP(w, req)

	assert.Equal(t, []string{"Accept-Encoding", "Origin"}, w.Header().Values("Vary"))
	assert.Equal(t, "Vary: Accept-Encoding, Origin", corsCharHeaderLines(w.Header())[3])
}

func Test_CORSContract_ParseOriginsExact(t *testing.T) {
	cases := []struct {
		name string
		in   string
		exp  map[string]bool
	}{
		{name: "empty", in: "", exp: map[string]bool{"*": true}},
		{name: "single space", in: " ", exp: map[string]bool{"*": true}},
		{name: "single comma", in: ",", exp: map[string]bool{"*": true}},
		{name: "wildcard with spaces", in: "  *  ", exp: map[string]bool{"*": true}},
		{name: "wildcard mixed with explicit origins", in: "*," + corsCharOriginA,
			exp: map[string]bool{"*": true, corsCharOriginA: true}},
		{name: "duplicates collapse", in: corsCharOriginA + "," + corsCharOriginA,
			exp: map[string]bool{corsCharOriginA: true}},
		{name: "tabs and newlines are trimmed", in: "\t" + corsCharOriginA + "\n",
			exp: map[string]bool{corsCharOriginA: true}},
		{name: "empty entries dropped", in: corsCharOriginA + ", ," + corsCharOriginB,
			exp: map[string]bool{corsCharOriginA: true, corsCharOriginB: true}},
		{name: "semicolons are not separators", in: corsCharOriginA + ";" + corsCharOriginB,
			exp: map[string]bool{corsCharOriginA + ";" + corsCharOriginB: true}},
		{name: "trailing comma", in: corsCharOriginA + ",", exp: map[string]bool{corsCharOriginA: true}},
	}

	for i := range cases {
		tc := &cases[i]

		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.exp, parseOrigins(tc.in))
		})
	}
}

// Test_CORSContract_WildcardBeatsExplicitMatch pins that a "*" entry anywhere in
// the list short-circuits dynamic matching and suppresses Vary entirely.

// Test_CORSContract_WildcardBeatsExplicitMatch pins that a "*" entry anywhere in
// the list short-circuits dynamic matching and suppresses Vary entirely.
func Test_CORSContract_WildcardBeatsExplicitMatch(t *testing.T) {
	routes := []string{http.MethodGet}
	cfg := map[string]string{corsCharKeyOrigin: corsCharOriginA + ",*"}

	w, _ := corsCharRun(t, cfg, &routes, http.MethodGet, corsCharOriginA)

	assert.Equal(t, corsCharSorted([]string{
		corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS"), corsCharOriginLine("*"),
	}), corsCharHeaderLines(w.Header()))
	assert.Empty(t, w.Header().Values("Vary"))
}

// Test_CORSContract_OptionsSkipsInnerHandlerForEveryConfig pins that OPTIONS
// short-circuits regardless of configuration or origin match.

// Test_CORSContract_OptionsSkipsInnerHandlerForEveryConfig pins that OPTIONS
// short-circuits regardless of configuration or origin match.
func Test_CORSContract_OptionsSkipsInnerHandlerForEveryConfig(t *testing.T) {
	configs := []map[string]string{
		nil,
		{},
		{corsCharKeyOrigin: "*"},
		{corsCharKeyOrigin: corsCharOriginA},
		{corsCharKeyOrigin: corsCharOriginA, corsCharKeyMethods: "GET"},
		{"x-garbage": "boom"},
	}

	origins := []string{"", corsCharOriginA, corsCharOriginEvil}

	for i, cfg := range configs {
		for _, origin := range origins {
			routes := []string{http.MethodGet}
			w, spy := corsCharRun(t, cfg, &routes, http.MethodOptions, origin)

			assert.Equal(t, http.StatusOK, w.Code, "config %d origin %q", i, origin)
			assert.Empty(t, w.Body.String(), "config %d origin %q", i, origin)
			assert.Equal(t, 0, spy.called, "config %d origin %q", i, origin)
		}
	}
}

// Test_CORSContract_NonOptionsMethodsAlwaysReachInner pins that every
// non-OPTIONS method falls through to the inner handler unchanged.

// Test_CORSContract_NonOptionsMethodsAlwaysReachInner pins that every
// non-OPTIONS method falls through to the inner handler unchanged.
func Test_CORSContract_NonOptionsMethodsAlwaysReachInner(t *testing.T) {
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodTrace, http.MethodConnect,
	}

	for _, m := range methods {
		routes := []string{http.MethodGet}
		w, spy := corsCharRun(t, map[string]string{corsCharKeyOrigin: "*"}, &routes, m, corsCharOriginA)

		assert.Equal(t, http.StatusFound, w.Code, "method %s", m)
		assert.Equal(t, corsCharBody, w.Body.String(), "method %s", m)
		assert.Equal(t, 1, spy.called, "method %s", m)
		assert.Equal(t, corsCharSorted([]string{
			corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS"), corsCharOriginLine("*"),
		}), corsCharHeaderLines(w.Header()), "method %s", m)
	}
}

// Test_CORSContract_SetMiddlewareHeadersDirectSnapshot pins the unexported
// helper directly, including the case where the passed allowedOrigins set
// disagrees with the config map (which the exported CORS wrapper cannot do).

// TestCanonicalizeConfig_Precedence covers the fold directly, including the tie-break among
// non-canonical spellings where no key is the canonical one.
func TestCanonicalizeConfig_Precedence(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
		want map[string]string
	}{
		{
			name: "canonical spelling beats every other",
			in:   map[string]string{"x-custom": "lower", "X-Custom": "canonical", "X-CUSTOM": "upper"},
			want: map[string]string{"X-Custom": "canonical"},
		},
		{
			name: "without a canonical spelling the smallest key wins",
			in:   map[string]string{"x-custom": "lower", "X-CUSTOM": "upper"},
			want: map[string]string{"X-Custom": "upper"},
		},
		{
			name: "distinct headers are all kept",
			in:   map[string]string{"x-one": "1", "X-Two": "2"},
			want: map[string]string{"X-One": "1", "X-Two": "2"},
		},
		{
			name: "empty config stays empty",
			in:   map[string]string{},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Repeated, because the property under test is independence from map iteration order.
			for range 50 {
				assert.Equal(t, tt.want, canonicalizeConfig(tt.in))
			}
		})
	}
}
