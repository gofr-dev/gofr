package rbac

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// coverageRoutes is an application route table shaped like the one GoFr hands the check: one entry
// per method and path, and a static directory served at /static.
func coverageRoutes() []string {
	return []string{
		"GET /api/users",
		"POST /api/users",
		"GET /api/users/{id}",
		"DELETE /api/users/{id}",
		"GET /api/posts",
		"* /static",
		"* /static/{path:.*}",
	}
}

// coverageBuiltInRoutes is the part of the route table GoFr registers itself.
func coverageBuiltInRoutes() []string {
	return []string{
		"GET /.well-known/health",
		"GET /.well-known/alive",
		"GET /favicon.ico",
	}
}

// guard is a non-public rule requiring one permission.
func guard(path string, methods ...string) EndpointMapping {
	return EndpointMapping{Path: path, Methods: methods, RequiredPermissions: []string{"p:x"}}
}

// coveringRules guards every non-built-in route in coverageRoutes.
func coveringRules() []EndpointMapping {
	return []EndpointMapping{
		guard("/api/users", http.MethodGet, http.MethodPost),
		guard("/api/users/{id}", http.MethodGet, http.MethodDelete),
		guard("/api/posts", http.MethodGet),
		guard("/static", "*"),
		guard("/static/{path:.*}", "*"),
	}
}

// Markers that tell the three error lines apart.
const (
	deadLine      = "rules match no registered route"
	uncoveredLine = "routes covered by no rule"
	partialLine   = "routes only partly covered"
)

// routeMismatchCase is one case of TestConfig_ReportRouteMismatches.
type routeMismatchCase struct {
	desc          string
	endpoints     []EndpointMapping
	extraRoutes   []string // application routes added to coverageRoutes for this case
	wantDead      []string // entries expected on the dead-rule line
	wantUncovered []string // entries expected on the uncovered-route line
	wantPartial   []string // entries expected on the partly-covered line
	notLogged     []string // substrings that must appear in no error line
	wantWarn      []string // substrings expected in the warning output
}

func routeMismatchCases() []routeMismatchCase {
	return []routeMismatchCase{
		{
			desc:      "every route covered and every rule live",
			endpoints: coveringRules(),
		},
		{
			desc:      "a typo in a path segment is a dead rule",
			endpoints: append(coveringRules(), guard("/api/user/{id}", http.MethodDelete)),
			wantDead:  []string{"DELETE /api/user/{id}"},
		},
		{
			desc:      "a method the route does not register is a dead rule",
			endpoints: append(coveringRules(), guard("/api/posts", http.MethodDelete)),
			wantDead:  []string{"DELETE /api/posts"},
		},
		{
			desc: "a wildcard-method rule is live and covers every method",
			endpoints: []EndpointMapping{
				guard("/api/users", "*"),
				guard("/api/users/{id}"),
				guard("/api/posts", "*"),
				guard("/static", "*"),
				guard("/static/{path:.*}", "*"),
			},
		},
		{
			desc: "a trailing slash makes a different path",
			endpoints: append(coveringRules()[1:],
				guard("/api/users/", http.MethodGet, http.MethodPost)),
			wantDead:      []string{"GET /api/users/", "POST /api/users/"},
			wantUncovered: []string{"GET /api/users", "POST /api/users"},
		},
		{
			desc: "a relative path matches nothing",
			endpoints: append(coveringRules()[1:],
				guard("api/users", http.MethodGet, http.MethodPost)),
			wantDead:      []string{"GET api/users", "POST api/users"},
			wantUncovered: []string{"GET /api/users", "POST /api/users"},
		},
		{
			desc: "a catch-all does not cover the path it hangs off",
			endpoints: append(coveringRules()[:3],
				guard("/static/{path:.*}", "*")),
			wantUncovered: []string{"* /static"},
			notLogged:     []string{"* /static/{path:.*}"},
		},
		{
			desc: "a constraint narrower than the route covers it only partly",
			endpoints: []EndpointMapping{
				guard("/api/users", http.MethodGet, http.MethodPost),
				guard("/api/users/{id:[0-9]+}", http.MethodGet, http.MethodDelete),
				guard("/api/posts", http.MethodGet),
				guard("/static", "*"),
				guard("/static/{path:.*}", "*"),
			},
			wantPartial: []string{
				"GET /api/users/{id} (by GET /api/users/{id:[0-9]+})",
				"DELETE /api/users/{id} (by DELETE /api/users/{id:[0-9]+})",
			},
		},
		{
			desc: "a method list narrower than a static route covers it only partly",
			endpoints: append(coveringRules()[:4],
				guard("/static/{path:.*}", http.MethodGet, http.MethodHead)),
			wantPartial: []string{"* /static/{path:.*} (by GET /static/{path:.*} and HEAD /static/{path:.*})"},
		},
	}
}

// builtInAndPublicCases are the cases of TestConfig_ReportRouteMismatches about built-in routes,
// public rules, and more than one kind of mismatch at once.
func builtInAndPublicCases() []routeMismatchCase {
	return []routeMismatchCase{
		{
			desc: "a rule for a built-in route is live",
			endpoints: append(coveringRules(),
				EndpointMapping{Path: "/.well-known/health", Methods: []string{http.MethodGet}, Public: true}),
		},
		{
			desc: "a dead public rule is a warning, not an error",
			endpoints: append(coveringRules(),
				EndpointMapping{Path: "/healthz", Methods: []string{http.MethodGet}, Public: true}),
			wantWarn: []string{"GET /healthz"},
		},
		{
			desc: "a public rule covers its route",
			endpoints: append(coveringRules()[1:],
				EndpointMapping{Path: "/api/users", Methods: []string{"*"}, Public: true}),
		},
		{
			desc: "uncovered routes are an error, built-in routes left out",
			endpoints: []EndpointMapping{
				guard("/api/users", http.MethodGet),
				guard("/api/users/{id}", "*"),
				guard("/static", "*"),
				guard("/static/{path:.*}", "*"),
			},
			wantUncovered: []string{"GET /api/posts", "POST /api/users"},
			notLogged:     []string{"/.well-known", "/favicon.ico", "GET /api/users,", "/api/users/{id}"},
		},
		{
			desc:          "an application route under /.well-known is checked",
			endpoints:     coveringRules(),
			extraRoutes:   []string{"GET /.well-known/custom"},
			wantUncovered: []string{"GET /.well-known/custom"},
		},
		{
			desc:          "dead rules and uncovered routes are both reported",
			endpoints:     append(coveringRules()[1:], guard("/api/user", http.MethodGet)),
			wantDead:      []string{"GET /api/user"},
			wantUncovered: []string{"GET /api/users", "POST /api/users"},
		},
	}
}

func TestConfig_ReportRouteMismatches(t *testing.T) {
	for i, tc := range append(routeMismatchCases(), builtInAndPublicCases()...) {
		t.Run(tc.desc, func(t *testing.T) {
			logger := &mockLogger{}
			config := newTestConfig(t, tc.endpoints, nil)
			config.Logger = logger

			got := config.ReportRouteMismatches(append(coverageRoutes(), tc.extraRoutes...), coverageBuiltInRoutes())

			wantMismatch := len(tc.wantDead)+len(tc.wantUncovered)+len(tc.wantPartial) > 0
			assert.Equal(t, wantMismatch, got, "TEST[%d], Failed.\n%s", i, tc.desc)

			assertLogLine(t, logger.errorLogs, deadLine, tc.wantDead, "TEST[%d], Failed.\n%s", i, tc.desc)
			assertLogLine(t, logger.errorLogs, uncoveredLine, tc.wantUncovered, "TEST[%d], Failed.\n%s", i, tc.desc)
			assertLogLine(t, logger.errorLogs, partialLine, tc.wantPartial, "TEST[%d], Failed.\n%s", i, tc.desc)

			errors := strings.Join(logger.errorLogs, "\n")
			for _, unwanted := range tc.notLogged {
				assert.NotContains(t, errors, unwanted, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			warnings := strings.Join(logger.warnLogs, "\n")
			for _, want := range tc.wantWarn {
				assert.Contains(t, warnings, want, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

// TestConfig_ReportRouteMismatches_NilLogger checks that a config with no logger still reports the
// mismatch through its result instead of panicking.
func TestConfig_ReportRouteMismatches_NilLogger(t *testing.T) {
	config := newTestConfig(t, coveringRules()[1:], nil)

	assert.True(t, config.ReportRouteMismatches(coverageRoutes(), coverageBuiltInRoutes()))
}

// assertLogLine checks that exactly one line carries marker and lists every entry of want when want
// is non-empty, and that no line carries marker otherwise.
func assertLogLine(t *testing.T, lines []string, marker string, want []string, msgAndArgs ...any) {
	t.Helper()

	var matched []string

	for _, line := range lines {
		if strings.Contains(line, marker) {
			matched = append(matched, line)
		}
	}

	if len(want) == 0 {
		assert.Empty(t, matched, msgAndArgs...)

		return
	}

	if !assert.Len(t, matched, 1, msgAndArgs...) {
		return
	}

	for _, entry := range want {
		assert.Contains(t, matched[0], entry, msgAndArgs...)
	}
}

func Test_routeEntry_overlaps(t *testing.T) {
	testCases := []struct {
		desc  string
		rule  string
		route string
		want  bool
	}{
		{"identical literals", "GET /api/users", "GET /api/users", true},
		{"different literals", "GET /api/users", "GET /api/posts", false},
		{"typo in a literal segment", "GET /api/user/{id}", "GET /api/users/{id}", false},
		{"free variable against literal", "GET /api/users/{id}", "GET /api/users/me", true},
		{"literal against free variable", "GET /api/users/me", "GET /api/users/{id}", true},
		{"free variable against an empty segment", "GET /api/users/{id}", "GET /api/users/", false},
		{"constraint admits the literal", "GET /api/users/{id:[0-9]+}", "GET /api/users/42", true},
		{"constraint rejects the literal", "GET /api/users/{id:[0-9]+}", "GET /api/users/me", false},
		{"route constraint rejects the literal", "GET /api/users/me", "GET /api/users/{id:[0-9]+}", false},
		{"variable against variable", "GET /api/users/{id:[0-9]+}", "GET /api/users/{id:[a-z]+}", true},
		{"more segments in the rule", "GET /api/users/{id}/posts", "GET /api/users/{id}", false},
		{"fewer segments in the rule", "GET /api", "GET /api/users", false},
		{"catch-all in the rule absorbs the rest", "GET /api/{path:.*}", "GET /api/users/{id}/posts", true},
		{"catch-all in the rule needs its slash", "GET /api/{path:.*}", "GET /api", false},
		{"catch-all in the rule matches an empty tail", "GET /api/{path:.*}", "GET /api/", true},
		{"catch-all in the route absorbs the rest", "GET /static/css/site.css", "GET /static/{path:.*}", true},
		{"catch-all after a different literal", "GET /admin/{path:.*}", "GET /api/users", false},
		{"trailing slash is a different path", "GET /api/users/", "GET /api/users", false},
		{"relative path is a different path", "GET api/users", "GET /api/users", false},
		{"root against root", "GET /", "GET /", true},
		{"root against a path", "GET /", "GET /api", false},
		{"constraint containing a slash spans segments", "GET /files/{path:[a-z/]+}", "GET /files/a/b/c", true},
		{"uncompilable constraint is assumed to overlap", "GET /api/{id:[}", "GET /api/users", true},
		{"space in a constraint is part of it", "GET /orders/{id: [0-9]+}", "GET /orders/42", false},
		{"two variables in one segment admit the literal", "GET /items/{a}-{b}", "GET /items/x-y", true},
		{"two variables in one segment reject the literal", "GET /items/{a}-{b}", "GET /items/abc", false},
		{"route constraint matching a slash spans segments", "GET /files/a/b", "GET /files/{p:[^.]+}", true},
		{"empty rule matches nothing", "GET ", "GET /api", false},
		{"different methods", "GET /api/users", "POST /api/users", false},
		{"wildcard rule method", "* /api/users", "POST /api/users", true},
		{"wildcard route method", "GET /static/{path:.*}", "* /static/{path:.*}", true},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rc := newRouteChecker()
			rule, route := rc.parseEntry(tc.rule), rc.parseEntry(tc.route)

			assert.Equal(t, tc.want, rule.overlaps(&route), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func Test_routeEntry_covers(t *testing.T) {
	testCases := []struct {
		desc  string
		rule  string
		route string
		want  bool
	}{
		{"identical literals", "GET /api/users", "GET /api/users", true},
		{"trailing slash in the rule", "GET /api/users/", "GET /api/users", false},
		{"relative rule path", "GET files", "GET /files", false},
		{"catch-all does not cover the path it hangs off", "GET /admin/{rest:.*}", "GET /admin", false},
		{"catch-all covers a path under it", "GET /admin/{rest:.*}", "GET /admin/users/{id}", true},
		{"catch-all covers an empty tail", "GET /api/{path:.*}", "GET /api/", true},
		{"non-empty catch-all does not cover an empty tail", "GET /api/{path:.+}", "GET /api/", false},
		{"catch-all covers a route catch-all", "GET /static/{p:.*}", "GET /static/{path:.*}", true},
		{"non-empty catch-all does not cover one that may be empty", "GET /static/{p:.+}", "GET /static/{path:.*}", false},
		{"free variable does not cover a route catch-all", "GET /static/{name}", "GET /static/{path:.*}", false},
		{"free variable covers a literal", "GET /api/users/{id}", "GET /api/users/me", true},
		{"free variable does not cover an empty segment", "GET /api/users/{id}", "GET /api/users/", false},
		{"free variable covers a free variable", "GET /api/users/{id}", "GET /api/users/{uid}", true},
		{"free variable covers a non-empty constraint", "GET /api/users/{id}", "GET /api/users/{id:[0-9]+}", true},
		{"free variable does not cover a constraint that admits empty", "GET /api/{id}", "GET /api/{id:[0-9]*}", false},
		{"constraint does not cover a free variable", "GET /api/users/{id:[0-9]+}", "GET /api/users/{id}", false},
		{"identical constraints", "GET /api/users/{id:[0-9]+}", "GET /api/users/{uid:[0-9]+}", true},
		{"different constraints", "GET /api/users/{id:[0-9]+}", "GET /api/users/{id:[a-z]+}", false},
		{"constraint admits the literal", "GET /api/users/{id:[0-9]+}", "GET /api/users/42", true},
		{"literal does not cover a variable", "GET /api/users/me", "GET /api/users/{id}", false},
		{"uncompilable constraint covers nothing", "GET /api/{id:[}", "GET /api/users", false},
		{"slash constraint covers a matching literal tail", "GET /files/{path:[a-z/]+}", "GET /files/a/b", true},
		{"slash constraint does not cover a variable", "GET /files/{path:[a-z/]+}", "GET /files/{name}", false},
		{"catch-all covers only as the rule's last segment", "GET /api/{rest:.*}/admin", "GET /api/users", false},
		{"two variables in one segment are not one free variable", "GET /items/{a}-{b}", "GET /items/{id}", false},
		{"two variables in one segment cover the same segment", "GET /items/{a}-{b}", "GET /items/{x}-{y}", true},
		{"two variables in one segment cover a literal they match", "GET /items/{a}-{b}", "GET /items/x-y", true},
		{"free variable covers two variables in one segment", "GET /items/{id}", "GET /items/{a}-{b}", true},
		{"free variable does not cover a constraint matching a slash", "GET /files/{x}", "GET /files/{p:[^.]+}", false},
		{"constraint matching a slash covers a literal tail", "GET /files/{p:[^.]+}", "GET /files/a/b", true},
		{"space in a constraint is part of it", "GET /orders/{id: [0-9]+}", "GET /orders/{id:[0-9]+}", false},
		{"same method", "GET /api/users", "GET /api/users", true},
		{"different method", "GET /api/users", "POST /api/users", false},
		{"wildcard rule covers any method", "* /api/users", "DELETE /api/users", true},
		{"wildcard rule covers a wildcard route", "* /static/{path:.*}", "* /static/{path:.*}", true},
		{"one method does not cover a wildcard route", "GET /static/{path:.*}", "* /static/{path:.*}", false},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rc := newRouteChecker()
			rule, route := rc.parseEntry(tc.rule), rc.parseEntry(tc.route)

			assert.Equal(t, tc.want, rule.covers(&route), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

// BenchmarkConfig_ReportRouteMismatches measures the check on an app with n routes, each guarded
// by its own rule, with a constrained variable in every path so the regex cache is exercised.
func BenchmarkConfig_ReportRouteMismatches(b *testing.B) {
	for _, n := range []int{50, 200, 1000} {
		b.Run(fmt.Sprintf("routes=%d", n), func(b *testing.B) {
			endpoints := make([]EndpointMapping, 0, n)
			routes := make([]string, 0, n)

			for i := range n {
				path := fmt.Sprintf("/api/v1/resource%d/{id:[0-9]+}/items", i)
				endpoints = append(endpoints, guard(path, http.MethodGet))
				routes = append(routes, "GET "+path)
			}

			config := newTestConfig(b, endpoints, nil)

			b.ReportAllocs()

			for b.Loop() {
				if config.ReportRouteMismatches(routes, nil) {
					b.Fatal("every route is covered, so no mismatch is expected")
				}
			}
		})
	}
}
