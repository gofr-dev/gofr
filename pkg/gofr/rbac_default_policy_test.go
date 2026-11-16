package gofr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

// rbacDenyingAdmin guards GET /admin and opens GET /open, and denies everything else. It reads the
// role from X-User-Role.
const rbacDenyingAdmin = `{"defaultPolicy":"deny","roleHeader":"X-User-Role",` +
	`"roles":[{"name":"admin","permissions":["admin:read"]}],` +
	`"endpoints":[{"path":"/admin","methods":["GET"],"requiredPermissions":["admin:read"]},` +
	`{"path":"/open","methods":["GET"],"public":true},` + rbacPublicStatic + `]}`

// rbacFirstConfig leaves defaultPolicy unset and guards GET /first, reading the role from X-First-Role.
const rbacFirstConfig = `{"roleHeader":"X-First-Role",` +
	`"roles":[{"name":"a","permissions":["first:read"]}],` +
	`"endpoints":[{"path":"/first","methods":["GET"],"requiredPermissions":["first:read"]},` + rbacPublicStatic + `]}`

// rbacSecondDenyingConfig denies by default and guards GET /second, reading the role from X-Second-Role.
const rbacSecondDenyingConfig = `{"defaultPolicy":"deny","roleHeader":"X-Second-Role",` +
	`"roles":[{"name":"b","permissions":["second:read"]}],` +
	`"endpoints":[{"path":"/second","methods":["GET"],"requiredPermissions":["second:read"]},` + rbacPublicStatic + `]}`

type defaultPolicyCase struct {
	desc    string
	configs []string
	method  string
	path    string
	headers map[string]string
	want    int
	mode    string // GOFR_RBAC_ROUTE_CHECK; empty leaves it unset
}

func defaultPolicyCases() []defaultPolicyCase {
	admin := map[string]string{"X-User-Role": "admin"}
	only := []string{rbacDenyingAdmin}

	return append([]defaultPolicyCase{
		{"covered route is unchanged", only, http.MethodGet, "/admin", admin, http.StatusOK, ""},
		{"covered route without a role is still 401", only, http.MethodGet, "/admin", nil, http.StatusUnauthorized, ""},
		{"public route is served", only, http.MethodGet, "/open", nil, http.StatusOK, ""},
		{"C3: a route no rule covers, no role, is 401", only, http.MethodGet, "/unruled", nil, http.StatusUnauthorized, ""},
		{"C3: a route no rule covers, with a role, is 403", only, http.MethodGet, "/unruled", admin, http.StatusForbidden, ""},
		{"F2: a method no rule covers on a covered path is 403", only, http.MethodDelete, "/admin", admin, http.StatusForbidden, ""},
		{"C2: an unregistered path keeps its 404", only, http.MethodGet, "/nope", admin, http.StatusNotFound, ""},
		{"C2: a method the path does not serve keeps its 405", only, http.MethodPost, "/admin", admin, http.StatusMethodNotAllowed, ""},
		{"C4: a CORS preflight to a route RBAC would refuse is answered first", only, http.MethodOptions, "/files/x.txt", nil,
			http.StatusOK, ""},
		{"Q2: GoFr's alive route is served", only, http.MethodGet, "/.well-known/alive", nil, http.StatusOK, ""},
		{"Q2: GoFr's openapi route is served", only, http.MethodGet, "/.well-known/openapi.json", nil, http.StatusOK, ""},
		{"Q2: GoFr's swagger route is served", only, http.MethodGet, "/.well-known/swagger", nil, http.StatusOK, ""},
		{"Q2: GoFr's favicon is not refused", only, http.MethodGet, faviconPath, nil, http.StatusOK, ""},
		{"Q2: an application route under /.well-known with no role is 401", only, http.MethodGet, "/.well-known/jwks.json", nil,
			http.StatusUnauthorized, ""},
		{"Q2: an application route under /.well-known with a role is 403", only, http.MethodGet, "/.well-known/jwks.json", admin,
			http.StatusForbidden, ""},
		{desc: "Q4: deny is enforced when the route check is off", configs: only, method: http.MethodGet, path: "/unruled",
			headers: admin, want: http.StatusForbidden, mode: "off"},
		{desc: "Q4: deny is enforced when the route check is fail", configs: only, method: http.MethodGet, path: "/unruled",
			headers: admin, want: http.StatusForbidden, mode: "fail"},
		{desc: "Q3: with the route check off the configs are still linked, so the first config's route is enforced by it",
			configs: []string{rbacFirstConfig, rbacSecondDenyingConfig}, method: http.MethodGet, path: "/first",
			headers: map[string]string{"X-First-Role": "a"}, want: http.StatusOK, mode: "off"},
	}, twoConfigDefaultPolicyCases()...)
}

// twoConfigDefaultPolicyCases are the cases of TestApp_RBACDefaultPolicy where two configs are installed,
// the second one denying by default.
func twoConfigDefaultPolicyCases() []defaultPolicyCase {
	both := []string{rbacFirstConfig, rbacSecondDenyingConfig}

	return []defaultPolicyCase{
		{"Q3: a route the first config covers is served", both, http.MethodGet, "/first",
			map[string]string{"X-First-Role": "a"}, http.StatusOK, ""},
		{"Q3: a route the first config covers is enforced by it, not denied by the second", both, http.MethodGet, "/first",
			map[string]string{"X-Second-Role": "b"}, http.StatusUnauthorized, ""},
		{"Q3: a route the second config covers is served", both, http.MethodGet, "/second",
			map[string]string{"X-Second-Role": "b"}, http.StatusOK, ""},
		{"O2: a route neither covers is denied by the deny config with its role header", both, http.MethodGet, "/neither",
			map[string]string{"X-Second-Role": "b"}, http.StatusForbidden, ""},
		{"O2: the deny config does not read the other config's role header", both, http.MethodGet, "/neither",
			map[string]string{"X-First-Role": "a"}, http.StatusUnauthorized, ""},
		{"Q3: a config left on allow does not make a route no config covers open", both, http.MethodGet, "/neither", nil,
			http.StatusUnauthorized, ""},
	}
}

func TestApp_RBACDefaultPolicy(t *testing.T) {
	testutil.NewServerConfigs(t)
	setupOpenAPIStaticFile(t, []byte(`{"openapi":"3.0.0"}`))

	tests := defaultPolicyCases()

	for _, matcher := range []string{"mux", "trie"} {
		t.Run(matcher, func(t *testing.T) {
			t.Setenv("GOFR_ROUTER", matcher)

			got := make([]int, len(tests))

			// The logger captures os.Stdout when it is created, so each app is built inside the capture.
			testutil.StdoutOutputForFunc(func() {
				apps := map[string]*App{}

				for i, tc := range tests {
					key := tc.mode + "|" + strings.Join(tc.configs, "|")

					app, ok := apps[key]
					if !ok {
						t.Setenv("GOFR_RBAC_ROUTE_CHECK", tc.mode)

						app = newDefaultPolicyApp(t, tc.configs)
						apps[key] = app
					}

					req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, http.NoBody)
					for k, v := range tc.headers {
						req.Header.Set(k, v)
					}

					w := httptest.NewRecorder()
					app.httpServer.router.ServeHTTP(w, req)

					got[i] = w.Code
				}
			})

			for i, tc := range tests {
				assert.Equal(t, tc.want, got[i], "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

// newDefaultPolicyApp builds an app with the routes the cases use, installs the RBAC configs in
// order and prepares the server, which links the configs.
func newDefaultPolicyApp(t *testing.T, configs []string) *App {
	t.Helper()

	app := New()
	handler := func(*Context) (any, error) { return "ok", nil }

	for _, path := range []string{"/admin", "/open", "/unruled", "/.well-known/jwks.json", "/first", "/second", "/neither"} {
		app.GET(path, handler)
	}

	app.DELETE("/admin", handler)

	// A static directory matches every method, OPTIONS included, and is neither built in nor the
	// catch-all, so only CORS answering first keeps a preflight to it from being refused.
	app.AddStaticFiles("/files", t.TempDir())

	for i, config := range configs {
		require.NoError(t, app.EnableRBAC(writeRBACConfig(t, t.TempDir(), fmt.Sprintf("rbac%d.json", i), config)))
	}

	require.Equal(t, startupOK, app.prepareHTTPServer())

	return app
}
