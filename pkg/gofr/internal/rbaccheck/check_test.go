package rbaccheck

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/rbac"
)

// toRoutes turns "METHOD /template" labels into routes, built in when builtIn is set.
func toRoutes(builtIn bool, labels ...string) []Route {
	routes := make([]Route, len(labels))

	for i, label := range labels {
		method, path, _ := strings.Cut(label, " ")
		routes[i] = Route{Method: method, Path: path, BuiltIn: builtIn}
	}

	return routes
}

// coverageRoutes is a route table shaped like the one GoFr hands the check: one entry per method
// and path, a static directory served at /static, and GoFr's own routes.
func coverageRoutes(extra ...string) []Route {
	routes := toRoutes(false, append([]string{
		"GET /api/users",
		"POST /api/users",
		"GET /api/users/{id}",
		"DELETE /api/users/{id}",
		"GET /api/posts",
		"* /static",
		"* /static/{path:.*}",
	}, extra...)...)

	return append(routes, toRoutes(true,
		"GET /.well-known/health",
		"GET /.well-known/alive",
		"GET /favicon.ico",
	)...)
}

// guard is a non-public rule requiring one permission.
func guard(path string, methods ...string) rbac.EndpointMapping {
	return rbac.EndpointMapping{Path: path, Methods: methods, RequiredPermissions: []string{"p:x"}}
}

// open is a public rule.
func open(path string, methods ...string) rbac.EndpointMapping {
	return rbac.EndpointMapping{Path: path, Methods: methods, Public: true}
}

// coveringRules guards every non-built-in route in coverageRoutes.
func coveringRules() []rbac.EndpointMapping {
	return []rbac.EndpointMapping{
		guard("/api/users", http.MethodGet, http.MethodPost),
		guard("/api/users/{id}", http.MethodGet, http.MethodDelete),
		guard("/api/posts", http.MethodGet),
		guard("/static", "*"),
		guard("/static/{path:.*}", "*"),
	}
}

// checkCase is one case of TestCheck.
type checkCase struct {
	desc        string
	endpoints   []rbac.EndpointMapping
	extraRoutes []string // application routes added to coverageRoutes for this case
	want        Mismatches
}

func mismatchCases() []checkCase {
	return []checkCase{
		{
			desc:      "every route covered and every rule live",
			endpoints: coveringRules(),
		},
		{
			desc:      "a typo in a path segment is a dead rule",
			endpoints: append(coveringRules(), guard("/api/user/{id}", http.MethodDelete)),
			want:      Mismatches{DeadRules: []string{"DELETE /api/user/{id}"}},
		},
		{
			desc:      "a method the route does not register is a dead rule",
			endpoints: append(coveringRules(), guard("/api/posts", http.MethodDelete)),
			want:      Mismatches{DeadRules: []string{"DELETE /api/posts"}},
		},
		{
			desc: "a wildcard-method rule is live and covers every method",
			endpoints: []rbac.EndpointMapping{
				guard("/api/users", "*"),
				guard("/api/users/{id}"),
				guard("/api/posts", "*"),
				guard("/static", "*"),
				guard("/static/{path:.*}", "*"),
			},
		},
		{
			desc:      "a trailing slash makes a different path",
			endpoints: append(coveringRules()[1:], guard("/api/users/", http.MethodGet, http.MethodPost)),
			want: Mismatches{
				DeadRules: []string{"GET /api/users/", "POST /api/users/"},
				Uncovered: []string{"GET /api/users", "POST /api/users"},
			},
		},
		{
			desc:      "a relative path matches nothing",
			endpoints: append(coveringRules()[1:], guard("api/users", http.MethodGet, http.MethodPost)),
			want: Mismatches{
				DeadRules: []string{"GET api/users", "POST api/users"},
				Uncovered: []string{"GET /api/users", "POST /api/users"},
			},
		},
		{
			desc:      "a catch-all does not cover the path it hangs off",
			endpoints: append(coveringRules()[:3], guard("/static/{path:.*}", "*")),
			want:      Mismatches{Uncovered: []string{"* /static"}},
		},
		{
			desc: "a constraint narrower than the route covers it only partly",
			endpoints: []rbac.EndpointMapping{
				guard("/api/users", http.MethodGet, http.MethodPost),
				guard("/api/users/{id:[0-9]+}", http.MethodGet, http.MethodDelete),
				guard("/api/posts", http.MethodGet),
				guard("/static", "*"),
				guard("/static/{path:.*}", "*"),
			},
			want: Mismatches{PartlyCovered: []string{
				"DELETE /api/users/{id} (by DELETE /api/users/{id:[0-9]+})",
				"GET /api/users/{id} (by GET /api/users/{id:[0-9]+})",
			}},
		},
		{
			desc:      "a method list narrower than a static route covers it only partly",
			endpoints: append(coveringRules()[:4], guard("/static/{path:.*}", http.MethodGet, http.MethodHead)),
			want: Mismatches{PartlyCovered: []string{
				"* /static/{path:.*} (by GET /static/{path:.*} and HEAD /static/{path:.*})",
			}},
		},
	}
}

// builtInAndPublicCases are the cases of TestCheck about built-in routes, public rules, and more
// than one kind of mismatch at once.
func builtInAndPublicCases() []checkCase {
	return []checkCase{
		{
			desc:      "a rule for a built-in route is live",
			endpoints: append(coveringRules(), open("/.well-known/health", http.MethodGet)),
		},
		{
			desc:      "a dead public rule is reported apart from dead guards",
			endpoints: append(coveringRules(), open("/healthz", http.MethodGet)),
			want:      Mismatches{DeadPublicRules: []string{"GET /healthz"}},
		},
		{
			desc:      "a public rule covers its route",
			endpoints: append(coveringRules()[1:], open("/api/users", "*")),
		},
		{
			desc: "uncovered routes are reported, built-in routes left out",
			endpoints: []rbac.EndpointMapping{
				guard("/api/users", http.MethodGet),
				guard("/api/users/{id}", "*"),
				guard("/static", "*"),
				guard("/static/{path:.*}", "*"),
			},
			want: Mismatches{Uncovered: []string{"GET /api/posts", "POST /api/users"}},
		},
		{
			desc:        "an application route under /.well-known is checked",
			endpoints:   coveringRules(),
			extraRoutes: []string{"GET /.well-known/custom"},
			want:        Mismatches{Uncovered: []string{"GET /.well-known/custom"}},
		},
		{
			desc:      "dead rules and uncovered routes are both reported",
			endpoints: append(coveringRules()[1:], guard("/api/user", http.MethodGet)),
			want: Mismatches{
				DeadRules: []string{"GET /api/user"},
				Uncovered: []string{"GET /api/users", "POST /api/users"},
			},
		},
		{
			desc:      "a route method is read in upper case",
			endpoints: coveringRules(),
			extraRoutes: []string{
				"get /api/posts",
			},
		},
	}
}

func TestCheck(t *testing.T) {
	for i, tc := range append(mismatchCases(), builtInAndPublicCases()...) {
		t.Run(tc.desc, func(t *testing.T) {
			got := Check(tc.endpoints, coverageRoutes(tc.extraRoutes...))

			assert.Equal(t, tc.want, got, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestMismatches_Failed(t *testing.T) {
	testCases := []struct {
		desc       string
		mismatches Mismatches
		want       bool
	}{
		{"no mismatch", Mismatches{}, false},
		{"only dead public rules", Mismatches{DeadPublicRules: []string{"GET /healthz"}}, false},
		{"a dead rule", Mismatches{DeadRules: []string{"GET /api/user"}}, true},
		{"an uncovered route", Mismatches{Uncovered: []string{"GET /api/users"}}, true},
		{"a partly covered route", Mismatches{PartlyCovered: []string{"GET /a (by GET /a/{id:[0-9]+})"}}, true},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.mismatches.Failed(), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

// agreementConfig declares its rules in the shapes Check has to read the way the rbac matcher does:
// a lower-case method, no methods at all, and a (method, path) declared twice, where the second
// declaration replaces the first.
const agreementConfig = `{"roleHeader":"X-User-Role",
"roles":[{"name":"admin","permissions":["p:x"]}],
"endpoints":[
{"path":"/lower","methods":["get"],"requiredPermissions":["p:x"]},
{"path":"/any","requiredPermissions":["p:x"]},
{"path":"/dup","methods":["GET"],"public":true},
{"path":"/dup","methods":["GET"],"requiredPermissions":["p:x"]},
{"path":"/gone","methods":["GET"],"public":true},
{"path":"/gone","methods":["get"],"requiredPermissions":["p:x"]}
]}`

// TestCheck_AgreesWithMiddleware checks that Check reads the rules the way the rbac middleware
// enforces them. Check rebuilds the rules from Config.Endpoints rather than sharing the matcher's,
// so it repeats how the matcher reads a declaration; if either side changes that alone, the two
// halves of this test disagree.
func TestCheck_AgreesWithMiddleware(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rbac.json")
	require.NoError(t, os.WriteFile(path, []byte(agreementConfig), 0o600))

	config, err := rbac.LoadPermissions(path, nil, nil, nil)
	require.NoError(t, err)

	// Every rule is guarded once declarations are read as the matcher reads them, and only /gone
	// has no route.
	got := Check(config.Endpoints, toRoutes(false, "GET /lower", "DELETE /any", "GET /dup"))
	assert.Equal(t, Mismatches{DeadRules: []string{"GET /gone"}}, got)

	// The middleware guards every one of those requests: a request with no role never reaches the
	// handler.
	reached := false
	guarded := rbac.Middleware(config)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))

	for i, request := range []string{"GET /lower", "DELETE /any", "GET /dup", "GET /gone"} {
		method, target, _ := strings.Cut(request, " ")
		reached = false

		guarded.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody))

		assert.False(t, reached, "TEST[%d], Failed.\n%s is served without a role", i, request)
	}
}

// BenchmarkCheck measures the check on an app with n routes, each guarded by its own rule, with a
// constrained variable in every path so the regex cache is exercised.
func BenchmarkCheck(b *testing.B) {
	for _, n := range []int{50, 200, 1000} {
		b.Run(fmt.Sprintf("routes=%d", n), func(b *testing.B) {
			endpoints := make([]rbac.EndpointMapping, 0, n)
			routes := make([]Route, 0, n)

			for i := range n {
				path := fmt.Sprintf("/api/v1/resource%d/{id:[0-9]+}/items", i)
				endpoints = append(endpoints, guard(path, http.MethodGet))
				routes = append(routes, Route{Method: http.MethodGet, Path: path})
			}

			b.ReportAllocs()

			for b.Loop() {
				if got := Check(endpoints, routes); got.Failed() {
					b.Fatal("every route is covered, so no mismatch is expected")
				}
			}
		})
	}
}
