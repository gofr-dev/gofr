package middleware

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
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
				"Access-Control-Allow-Headers": corsCharDefaultAllowHeaders,
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
				"Access-Control-Allow-Headers": corsCharDefaultAllowHeaders + ", clientid",
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
				"Access-Control-Allow-Headers": corsCharDefaultAllowHeaders,
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
				"Access-Control-Allow-Headers": corsCharDefaultAllowHeaders,
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
				"Access-Control-Allow-Headers": corsCharDefaultAllowHeaders,
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
				"Access-Control-Allow-Headers": corsCharDefaultAllowHeaders,
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

// ---------------------------------------------------------------------------
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
			name:   "lower cased allow-headers key bypasses the concat branch and replaces the defaults",
			config: map[string]string{"access-control-allow-headers": "only-this"}, method: http.MethodGet, routes: twoRoutes,
			expLines: []string{corsCharKeyHeaders + ": only-this", baseMethods, corsCharOriginLine("*")},
			expCode:  http.StatusFound, expBody: corsCharBody, expInner: 1,
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
			name:   "lower cased origin key cannot inject an origin through the custom header loop",
			config: map[string]string{"access-control-allow-origin": corsCharOriginEvil}, method: http.MethodGet, routes: twoRoutes,
			// No origin was negotiated (the allow-list is unset, so it is the
			// "*" default) and the custom loop must not emit one of its own.
			expLines: []string{corsCharAllowHeadersLine, baseMethods, corsCharOriginLine("*")},
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
func Test_CORSContract_RoutesReadAtRequestTime(t *testing.T) {
	routes := []string{http.MethodGet}
	handler := CORS(nil, &routes)(&corsCharSpyHandler{})

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody))
	require.Equal(t, "GET, OPTIONS", first.Header().Get(corsCharKeyMethods))

	routes = []string{http.MethodGet, http.MethodPost}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody))
	assert.Equal(t, "GET, POST, OPTIONS", second.Header().Get(corsCharKeyMethods))

	// Replacing the whole slice through the same variable is also picked up.
	routes = []string{http.MethodDelete}

	third := httptest.NewRecorder()
	handler.ServeHTTP(third, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody))
	assert.Equal(t, "DELETE, OPTIONS", third.Header().Get(corsCharKeyMethods))
}

// Test_CORSContract_RoutesBackingArrayAliasing pins a latent aliasing quirk:
// setMiddlewareHeaders does `routes = append(routes, "OPTIONS")` on a copy of
// the dereferenced slice header. When cap > len the append writes "OPTIONS"
// into the CALLER's backing array at index len, clobbering whatever was there.
// This is NOT visible through the caller's slice (its length is unchanged) but
// it is visible through any alias with a larger length, and it is silently
// overwritten again by the caller's next append.
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

// Test_CORSContract_NoAliasingWhenCapEqualsLen pins the complementary case:
// when cap == len the append allocates, so the caller's array is untouched.
func Test_CORSContract_NoAliasingWhenCapEqualsLen(t *testing.T) {
	routes := []string{http.MethodGet}
	require.Equal(t, len(routes), cap(routes))

	w, _ := corsCharRun(t, nil, &routes, http.MethodGet, "")

	require.Equal(t, "GET, OPTIONS", w.Header().Get(corsCharKeyMethods))
	assert.Equal(t, []string{http.MethodGet}, routes)
	assert.Equal(t, 1, cap(routes))
}

// Test_CORSContract_ParseOriginsEvaluatedOnce pins that the allowed-origin set
// is computed ONCE at construction time, so mutating the config map afterwards
// does not change origin matching, even though the header-setting loop does
// read the map on every request.
func Test_CORSContract_ParseOriginsEvaluatedOnce(t *testing.T) {
	cfg := map[string]string{corsCharKeyOrigin: corsCharOriginA}
	routes := []string{http.MethodGet}
	handler := CORS(cfg, &routes)(&corsCharSpyHandler{})

	cfg[corsCharKeyOrigin] = corsCharOriginB

	// The stale set still matches the ORIGINAL origin.
	stale := httptest.NewRecorder()
	reqA := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody)
	reqA.Header.Set("Origin", corsCharOriginA)
	handler.ServeHTTP(stale, reqA)

	assert.Equal(t, corsCharSorted([]string{
		corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS"),
		corsCharOriginLine(corsCharOriginA), corsCharVaryLine,
	}), corsCharHeaderLines(stale.Header()))

	// The newly configured origin is NOT honored.
	fresh := httptest.NewRecorder()
	reqB := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody)
	reqB.Header.Set("Origin", corsCharOriginB)
	handler.ServeHTTP(fresh, reqB)

	assert.Equal(t, corsCharSorted([]string{
		corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS"),
	}), corsCharHeaderLines(fresh.Header()))
}

// Test_CORSContract_ConfigMutationAffectsHeaderLoop pins the other half: the
// custom-header loop DOES read the map per request, so keys added after
// construction show up immediately.
func Test_CORSContract_ConfigMutationAffectsHeaderLoop(t *testing.T) {
	cfg := map[string]string{}
	routes := []string{http.MethodGet}
	handler := CORS(cfg, &routes)(&corsCharSpyHandler{})

	before := httptest.NewRecorder()
	handler.ServeHTTP(before, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody))
	require.Equal(t, corsCharSorted([]string{
		corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS"), corsCharOriginLine("*"),
	}), corsCharHeaderLines(before.Header()))

	cfg["Access-Control-Max-Age"] = "42"
	cfg[corsCharKeyHeaders] = "clientid"
	cfg[corsCharKeyMethods] = "GET, PATCH"

	after := httptest.NewRecorder()
	handler.ServeHTTP(after, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", http.NoBody))

	assert.Equal(t, corsCharSorted([]string{
		corsCharAllowHeadersLine + ", clientid",
		corsCharMethodsLine("GET, PATCH"),
		corsCharOriginLine("*"),
		"Access-Control-Max-Age: 42",
	}), corsCharHeaderLines(after.Header()))
}

// Test_CORSContract_CustomHeaderLoopIsOrderIndependent pins that Go's random
// map iteration order over the custom-header loop cannot change the final
// header set.
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
func Test_CORSContract_SetMiddlewareHeadersDirectSnapshot(t *testing.T) {
	cases := []struct {
		name     string
		config   map[string]string
		routes   []string
		origin   string
		allowed  map[string]bool
		expLines []string
	}{
		{
			name: "nil allowed origins set emits no origin header", config: map[string]string{},
			routes: []string{http.MethodGet}, origin: corsCharOriginA, allowed: nil,
			expLines: []string{corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS")},
		},
		{
			name: "empty allowed origins set emits no origin header", config: map[string]string{},
			routes: []string{http.MethodGet}, origin: corsCharOriginA, allowed: map[string]bool{},
			expLines: []string{corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS")},
		},
		{
			name: "false valued entry does not match", config: map[string]string{},
			routes: []string{http.MethodGet}, origin: corsCharOriginA,
			allowed:  map[string]bool{corsCharOriginA: false},
			expLines: []string{corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS")},
		},
		{
			name:   "empty string origin can be allowed and is echoed as an empty header",
			config: map[string]string{}, routes: []string{http.MethodGet}, origin: "",
			allowed: map[string]bool{"": true},
			expLines: []string{
				corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS"),
				corsCharKeyOrigin + ": ", corsCharVaryLine,
			},
		},
		{
			name: "allowed set overrides what the config map says", config: map[string]string{corsCharKeyOrigin: corsCharOriginA},
			routes: []string{http.MethodGet}, origin: corsCharOriginEvil,
			allowed: map[string]bool{corsCharOriginEvil: true},
			expLines: []string{
				corsCharAllowHeadersLine, corsCharMethodsLine("GET, OPTIONS"),
				corsCharOriginLine(corsCharOriginEvil), corsCharVaryLine,
			},
		},
	}

	for i := range cases {
		tc := &cases[i]

		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			setMiddlewareHeaders(tc.config, tc.routes, w, tc.origin, tc.allowed)

			assert.Equal(t, corsCharSorted(tc.expLines), corsCharHeaderLines(w.Header()))
		})
	}
}
