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
		desc      string
		endpoints []EndpointMapping
		wantDead  []string // "METHOD path" entries expected in the returned error
		wantWarn  []string // substrings expected in the warning output
		notInWarn []string // substrings that must not appear in the warning output
	}{
		{
			desc:      "every route covered and every rule live",
			endpoints: coveringRules(),
			notInWarn: []string{"GET", "POST", "DELETE"},
		},
		{
			desc: "a typo in a path segment is a dead rule",
			endpoints: append(coveringRules(),
				guard("/api/user/{id}", http.MethodDelete)),
			wantDead: []string{"DELETE /api/user/{id}"},
		},
		{
			desc: "a method the route does not register is a dead rule",
			endpoints: []EndpointMapping{
				guard("/api/posts", http.MethodGet, http.MethodDelete),
			},
			wantDead: []string{"DELETE /api/posts"},
		},
		{
			desc: "a wildcard-method rule is live when the path matches",
			endpoints: []EndpointMapping{
				guard("/api/posts", "*"),
				guard("/api/users/{id}"),
			},
		},
		{
			desc: "a rule under a static prefix is live",
			endpoints: []EndpointMapping{
				guard("/static/app.js", http.MethodGet),
			},
		},
		{
			desc: "the PathPrefix catch-all does not make a rule live",
			endpoints: []EndpointMapping{
				guard("/nowhere", http.MethodGet),
			},
			wantDead: []string{"GET /nowhere"},
		},
		{
			desc: "a rule for a built-in route is live",
			endpoints: []EndpointMapping{
				{Path: "/.well-known/health", Methods: []string{http.MethodGet}, Public: true},
			},
		},
		{
			desc: "a dead public rule is a warning, not an error",
			endpoints: append(coveringRules(),
				EndpointMapping{Path: "/healthz", Methods: []string{http.MethodGet}, Public: true}),
			wantWarn: []string{"GET /healthz"},
		},
		{
			desc: "uncovered routes are listed in one warning, built-in routes left out",
			endpoints: []EndpointMapping{
				guard("/api/users", http.MethodGet),
				guard("/api/users/{id}", "*"),
				guard("/static/{path:.*}", "*"),
			},
			wantWarn:  []string{"2 registered route(s)", "POST /api/users", "GET /api/posts"},
			notInWarn: []string{"/.well-known", faviconPath, "GET /api/users,", "/api/users/{id}"},
		},
		{
			desc:      "an uncovered static prefix is reported",
			endpoints: coveringRules()[:3],
			wantWarn:  []string{"* /static/"},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			logger := &mockLogger{}
			config := newTestConfig(t, tc.endpoints, nil)
			config.Logger = logger

			err := config.CheckRoutes(newCoverageRouter())

			if len(tc.wantDead) == 0 {
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				require.ErrorIs(t, err, ErrDeadRules, "TEST[%d], Failed.\n%s", i, tc.desc)

				for _, dead := range tc.wantDead {
					assert.Contains(t, err.Error(), dead, "TEST[%d], Failed.\n%s", i, tc.desc)
				}
			}

			warnings := strings.Join(logger.warnLogs, "\n")

			for _, want := range tc.wantWarn {
				assert.Contains(t, warnings, want, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			for _, unwanted := range tc.notInWarn {
				assert.NotContains(t, warnings, unwanted, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}
