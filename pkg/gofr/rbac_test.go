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

func TestEnableRBACWithError(t *testing.T) {
	tests := []struct {
		desc       string
		setup      func(t *testing.T) []string // returns the EnableRBACWithError arguments
		wantErr    string
		wantStatus int
	}{
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

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			args := tc.setup(t)
			a := newAuthTestApp(t)

			err := a.EnableRBACWithError(args...)

			if tc.wantErr == "" {
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				require.ErrorContains(t, err, tc.wantErr, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			assert.Equal(t, tc.wantStatus, unauthenticatedStatus(t, a), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestEnableRBAC(t *testing.T) {
	tests := []struct {
		desc         string
		configPath   func(t *testing.T) string
		wantDisabled bool
		wantStatus   int
	}{
		{
			desc: "valid config installs the middleware",
			configPath: func(t *testing.T) string {
				t.Helper()
				return writeRBACConfig(t, t.TempDir(), "rbac.json", rbacGuardingRoot)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "missing file logs that authorization is disabled",
			configPath: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "does-not-exist.yaml")
			},
			wantDisabled: true,
			wantStatus:   http.StatusOK,
		},
		{
			desc: "invalid config logs that authorization is disabled",
			configPath: func(t *testing.T) string {
				t.Helper()
				return writeRBACConfig(t, t.TempDir(), "rbac.json", `invalid json content{`)
			},
			wantDisabled: true,
			wantStatus:   http.StatusOK,
		},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			path := tc.configPath(t)

			var a *App

			logs := testutil.StderrOutputForFunc(func() {
				a = newAuthTestApp(t)
				a.EnableRBAC(path)
			})

			if tc.wantDisabled {
				assert.Contains(t, logs, "Authorization is DISABLED", "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Contains(t, logs, "EnableRBACWithError", "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				assert.NotContains(t, logs, "DISABLED", "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			assert.Equal(t, tc.wantStatus, unauthenticatedStatus(t, a), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
