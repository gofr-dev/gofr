package gofr

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

// rbacGuardingRoot requires a permission on "/", so an unauthenticated request is rejected
// with 401 only when the RBAC middleware is installed.
const rbacGuardingRoot = `{"roleHeader":"X-User-Role",` +
	`"roles":[{"name":"admin","permissions":["admin:read"]}],` +
	`"endpoints":[{"path":"/","methods":["GET"],"requiredPermissions":["admin:read"]}]}`

const rbacInheritanceGuardingRoot = `{"roleHeader":"X-User-Role",` +
	`"roles":[{"name":"viewer","permissions":["users:read"]},` +
	`{"name":"editor","permissions":["users:write"],"inheritsFrom":["viewer"]}],` +
	`"endpoints":[{"path":"/","methods":["GET"],"requiredPermissions":["users:read"]}]}`

const rbacGuardingRootYAML = `roleHeader: X-User-Role
roles:
  - name: admin
    permissions: ["admin:read"]
endpoints:
  - path: /
    methods: ["GET"]
    requiredPermissions: ["admin:read"]
`

// writeRBACConfig writes content to dir/name, creating parent directories, and returns the path.
func writeRBACConfig(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

type enableRBACTestCase struct {
	desc       string
	setup      func(t *testing.T) []string // returns the EnableRBAC arguments
	wantErr    string
	wantStatus int
}

// enableRBACTestCases lists the EnableRBAC scenarios: the valid ones install the middleware
// (401 on "/"), the failing ones leave authorization disabled (200 on "/").
func enableRBACTestCases() []enableRBACTestCase {
	return []enableRBACTestCase{
		{
			desc: "valid JSON config",
			setup: func(t *testing.T) []string {
				t.Helper()
				return []string{writeRBACConfig(t, t.TempDir(), "rbac.json", rbacGuardingRoot)}
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "valid YAML config",
			setup: func(t *testing.T) []string {
				t.Helper()
				return []string{writeRBACConfig(t, t.TempDir(), "rbac.yaml", rbacGuardingRootYAML)}
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "config with role inheritance",
			setup: func(t *testing.T) []string {
				t.Helper()
				return []string{writeRBACConfig(t, t.TempDir(), "rbac.json", rbacInheritanceGuardingRoot)}
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "default path",
			setup: func(t *testing.T) []string {
				t.Helper()

				dir := t.TempDir()
				writeRBACConfig(t, dir, "configs/rbac.yaml", rbacGuardingRootYAML)
				t.Chdir(dir)

				return nil
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "no default file",
			setup: func(t *testing.T) []string {
				t.Helper()
				t.Chdir(t.TempDir())

				return nil
			},
			wantErr:    "no RBAC config file found",
			wantStatus: http.StatusOK,
		},
		{
			desc: "missing file",
			setup: func(t *testing.T) []string {
				t.Helper()
				return []string{filepath.Join(t.TempDir(), "does-not-exist.yaml")}
			},
			wantErr:    "failed to read RBAC config file",
			wantStatus: http.StatusOK,
		},
		{
			desc: "invalid JSON",
			setup: func(t *testing.T) []string {
				t.Helper()
				return []string{writeRBACConfig(t, t.TempDir(), "rbac.json", `invalid json content{`)}
			},
			wantErr:    "failed to parse JSON config file",
			wantStatus: http.StatusOK,
		},
		{
			desc: "unsupported extension",
			setup: func(t *testing.T) []string {
				t.Helper()
				return []string{writeRBACConfig(t, t.TempDir(), "rbac.txt", rbacGuardingRoot)}
			},
			wantErr:    "unsupported config file format",
			wantStatus: http.StatusOK,
		},
	}
}

func TestEnableRBAC(t *testing.T) {
	for i, tc := range enableRBACTestCases() {
		t.Run(tc.desc, func(t *testing.T) {
			args := tc.setup(t)

			var (
				a   *App
				err error
			)

			logs := testutil.StderrOutputForFunc(func() {
				a = newAuthTestApp(t)
				err = a.EnableRBAC(args...)
			})

			if tc.wantErr == "" {
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.NotContains(t, logs, "DISABLED", "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				require.ErrorContains(t, err, tc.wantErr, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Contains(t, logs, "Authorization is DISABLED", "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			assert.Equal(t, tc.wantStatus, unauthenticatedStatus(t, a), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

// rbacPublicStatic opens the ./static directory New() serves when this package's tests run: the
// directory's own path and every path under it.
const rbacPublicStatic = `{"path":"/static","methods":["*"],"public":true},` +
	`{"path":"/static/{path:.*}","methods":["*"],"public":true}`

// rbacGuardingPing guards the only application route, GET /ping.
const rbacGuardingPing = `{"roleHeader":"X-User-Role",` +
	`"roles":[{"name":"admin","permissions":["admin:read"]}],` +
	`"endpoints":[{"path":"/ping","methods":["GET"],"requiredPermissions":["admin:read"]},` + rbacPublicStatic + `]}`

// rbacWithDeadRule adds a rule whose path has a typo, so it matches no registered route.
const rbacWithDeadRule = `{"roleHeader":"X-User-Role",` +
	`"roles":[{"name":"admin","permissions":["admin:read"]}],` +
	`"endpoints":[{"path":"/ping","methods":["GET"],"requiredPermissions":["admin:read"]},` +
	`{"path":"/api/user/{id}","methods":["DELETE"],"requiredPermissions":["admin:read"]},` + rbacPublicStatic + `]}`

// rbacLeavingPingUncovered has no dead rule, but no rule covers GET /ping either.
const rbacLeavingPingUncovered = `{"roleHeader":"X-User-Role",` +
	`"roles":[{"name":"admin","permissions":["admin:read"]}],` +
	`"endpoints":[{"path":"/.well-known/alive","methods":["GET"],"public":true},` + rbacPublicStatic + `]}`

// routeCheckFailed starts every ERROR line the route check writes.
const routeCheckFailed = "RBAC route check failed"

func TestApp_prepareHTTPServer(t *testing.T) {
	tests := []struct {
		desc        string
		config      string
		mode        string     // GOFR_RBAC_ROUTE_CHECK; empty leaves it unset
		setup       func(*App) // extra registration before the check runs
		wantLog     []string   // on stderr, where ERROR lines go
		notLog      []string   // must not appear on stderr
		wantOutcome startupOutcome
	}{
		{
			desc:   "rules that all match a route log nothing",
			config: rbacGuardingPing,
			notLog: []string{routeCheckFailed},
		},
		{
			desc:    "a dead rule is logged as an error",
			config:  rbacWithDeadRule,
			wantLog: []string{routeCheckFailed, "DELETE /api/user/{id}"},
		},
		{
			desc:    "an uncovered route is logged as an error",
			config:  rbacLeavingPingUncovered,
			wantLog: []string{routeCheckFailed, "GET /ping"},
		},
		{
			desc:    "warn logs a mismatch and keeps starting",
			config:  rbacLeavingPingUncovered,
			mode:    "warn",
			wantLog: []string{routeCheckFailed, "GET /ping"},
		},
		{
			desc:        "fail logs a mismatch and stops startup",
			config:      rbacLeavingPingUncovered,
			mode:        "fail",
			wantLog:     []string{routeCheckFailed, "GET /ping", "GOFR_RBAC_ROUTE_CHECK=fail"},
			wantOutcome: startupFailed,
		},
		{
			desc:   "fail keeps starting when nothing is wrong",
			config: rbacGuardingPing,
			mode:   "fail",
			notLog: []string{routeCheckFailed},
		},
		{
			desc:   "off skips the check",
			config: rbacLeavingPingUncovered,
			mode:   "off",
			notLog: []string{routeCheckFailed, "GET /ping"},
		},
		{
			desc:    "an unknown mode is reported and treated as warn",
			config:  rbacLeavingPingUncovered,
			mode:    "strict",
			wantLog: []string{"invalid GOFR_RBAC_ROUTE_CHECK", "strict", routeCheckFailed, "GET /ping"},
		},
		{
			desc:    "a static directory at the root is checked",
			config:  rbacGuardingPing,
			setup:   func(a *App) { a.AddStaticFiles("/", t.TempDir()) },
			wantLog: []string{routeCheckFailed, "* /{path:.*}"},
		},
		{
			desc:    "an application route under /.well-known is checked, GoFr's own are not",
			config:  rbacGuardingPing,
			setup:   func(a *App) { a.GET("/.well-known/custom", func(*Context) (any, error) { return "ok", nil }) },
			wantLog: []string{routeCheckFailed, "GET /.well-known/custom"},
			notLog:  []string{"/.well-known/health", "/.well-known/alive", "favicon.ico"},
		},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			testutil.NewServerConfigs(t)

			if tc.mode != "" {
				t.Setenv("GOFR_RBAC_ROUTE_CHECK", tc.mode)
			}

			outcome, stderr := runPrepareHTTPServer(t, tc.config, tc.setup)

			assert.Equal(t, tc.wantOutcome, outcome, "TEST[%d], Failed.\n%s", i, tc.desc)

			for _, want := range tc.wantLog {
				assert.Contains(t, stderr, want, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			for _, unwanted := range tc.notLog {
				assert.NotContains(t, stderr, unwanted, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

// runPrepareHTTPServer builds an app serving GET /ping with the RBAC config content, applies setup
// when it is set, and runs prepareHTTPServer, returning its outcome and what it wrote to stderr.
func runPrepareHTTPServer(t *testing.T, config string, setup func(*App)) (outcome startupOutcome, stderr string) {
	t.Helper()

	path := writeRBACConfig(t, t.TempDir(), "rbac.json", config)

	stderr = testutil.StderrOutputForFunc(func() {
		a := New()
		a.GET("/ping", func(*Context) (any, error) { return "pong", nil })

		if setup != nil {
			setup(a)
		}

		require.NoError(t, a.EnableRBAC(path))

		outcome = a.prepareHTTPServer()
	})

	return outcome, stderr
}

// TestRun_RBACRouteCheckFailExitsNonZero checks that GOFR_RBAC_ROUTE_CHECK=fail reaches the
// process: an orchestrator reads only the exit status, so a refused start that exited 0 would be
// recorded as a success.
func TestRun_RBACRouteCheckFailExitsNonZero(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("GOFR_RBAC_ROUTE_CHECK", "fail")

	path := writeRBACConfig(t, t.TempDir(), "rbac.json", rbacLeavingPingUncovered)

	var codes []int

	_ = testutil.StderrOutputForFunc(func() {
		a := New()
		a.GET("/ping", func(*Context) (any, error) { return "pong", nil })
		require.NoError(t, a.EnableRBAC(path))
		a.exit = func(code int) { codes = append(codes, code) }

		a.Run()
	})

	assert.Equal(t, []int{exitCodeStartupFailed}, codes)
}

// TestRun_RBACRouteCheckFailStopsBeforeServers checks that a failed route check is the last thing a
// refused start does: the MCP port is not looked at, and no HTTP server is announced.
func TestRun_RBACRouteCheckFailStopsBeforeServers(t *testing.T) {
	tests := []struct {
		desc      string
		enableMCP bool // with an MCP_PORT that cannot be served, which fails startup if looked at
	}{
		{desc: "no HTTP server is announced"},
		{desc: "the MCP port is not looked at", enableMCP: true},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			testutil.NewServerConfigs(t)
			t.Setenv("GOFR_RBAC_ROUTE_CHECK", "fail")
			t.Setenv("MCP_PORT", "70000")

			path := writeRBACConfig(t, t.TempDir(), "rbac.json", rbacLeavingPingUncovered)

			var stderr string

			stdout := testutil.StdoutOutputForFunc(func() {
				stderr = testutil.StderrOutputForFunc(func() {
					a := New()
					a.GET("/ping", func(*Context) (any, error) { return "pong", nil })

					if tc.enableMCP {
						a.EnableMCP()
					}

					require.NoError(t, a.EnableRBAC(path))
					a.exit = func(int) {}

					a.Run()
				})
			})

			assert.Contains(t, stderr, routeCheckFailed, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.NotContains(t, stderr, "MCP server cannot start", "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.NotContains(t, stdout, "Registered HTTP server on port", "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
