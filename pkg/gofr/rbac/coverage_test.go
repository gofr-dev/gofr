package rbac

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCoverageRouter builds a router shaped like GoFr's: one route per method and path, the built-in
// well-known routes, a static-file prefix, and the PathPrefix("/") catch-all registered last.
func newCoverageRouter() *mux.Router {
	noop := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	router := mux.NewRouter()

	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/api/users"},
		{http.MethodGet, "/api/users/{id}"},
		{http.MethodDelete, "/api/users/{id}"},
		{http.MethodGet, "/api/posts"},
		{http.MethodGet, "/.well-known/health"},
		{http.MethodGet, "/.well-known/alive"},
		{http.MethodGet, faviconPath},
	} {
		router.NewRoute().Methods(r.method).Path(r.path).Handler(noop)
	}

	router.NewRoute().PathPrefix("/static/").Handler(noop)
	router.PathPrefix("/").Handler(noop)

	return router
}

// guard is a non-public rule requiring one permission.
func guard(path string, methods ...string) EndpointMapping {
	return EndpointMapping{Path: path, Methods: methods, RequiredPermissions: []string{"p:x"}}
}

// coveringRules guards every non-built-in route in newCoverageRouter.
func coveringRules() []EndpointMapping {
	return []EndpointMapping{
		guard("/api/users", http.MethodGet, http.MethodPost),
		guard("/api/users/{id}", http.MethodGet, http.MethodDelete),
		guard("/api/posts", http.MethodGet),
		guard("/static/{path:.*}", "*"),
	}
}

func TestConfig_CheckRoutes(t *testing.T) {
	testCases := []struct {
		desc          string
		endpoints     []EndpointMapping
		wantDead      []string // "METHOD path" rules expected under ErrDeadRules
		wantUncovered []string // "METHOD path" routes expected under ErrUncoveredRoutes
		notInErr      []string // substrings that must not appear in the error
		wantWarn      []string // substrings expected in the warning output
	}{
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
				guard("/static/{path:.*}", "*"),
			},
		},
		{
			desc:      "a rule under a static prefix is live and covers the prefix",
			endpoints: append(coveringRules()[:3], guard("/static/app.js", http.MethodGet)),
		},
		{
			desc:      "the PathPrefix catch-all does not make a rule live",
			endpoints: append(coveringRules(), guard("/nowhere", http.MethodGet)),
			wantDead:  []string{"GET /nowhere"},
		},
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
				guard("/static/{path:.*}", "*"),
			},
			wantUncovered: []string{"GET /api/posts", "POST /api/users"},
			notInErr:      []string{"/.well-known", faviconPath, "GET /api/users,", "/api/users/{id}"},
		},
		{
			desc:          "an uncovered static prefix is reported",
			endpoints:     coveringRules()[:3],
			wantUncovered: []string{"* /static/"},
		},
		{
			desc:          "dead rules and uncovered routes are both reported",
			endpoints:     append(coveringRules()[1:], guard("/api/user", http.MethodGet)),
			wantDead:      []string{"GET /api/user"},
			wantUncovered: []string{"GET /api/users", "POST /api/users"},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			logger := &mockLogger{}
			config := newTestConfig(t, tc.endpoints, nil)
			config.Logger = logger

			err := config.CheckRoutes(newCoverageRouter())

			assertRouteCheckPart(t, err, ErrDeadRules, tc.wantDead, "TEST[%d], Failed.\n%s", i, tc.desc)
			assertRouteCheckPart(t, err, ErrUncoveredRoutes, tc.wantUncovered, "TEST[%d], Failed.\n%s", i, tc.desc)

			for _, unwanted := range tc.notInErr {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.NotContains(t, err.Error(), unwanted, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			warnings := strings.Join(logger.warnLogs, "\n")

			for _, want := range tc.wantWarn {
				assert.Contains(t, warnings, want, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

// assertRouteCheckPart checks that err carries sentinel and lists every entry of want when want is
// non-empty, and that it does not carry sentinel otherwise.
func assertRouteCheckPart(t *testing.T, err, sentinel error, want []string, msgAndArgs ...any) {
	t.Helper()

	if len(want) == 0 {
		assert.NotErrorIs(t, err, sentinel, msgAndArgs...)

		return
	}

	require.ErrorIs(t, err, sentinel, msgAndArgs...)

	for _, entry := range want {
		assert.Contains(t, err.Error(), entry, msgAndArgs...)
	}
}
