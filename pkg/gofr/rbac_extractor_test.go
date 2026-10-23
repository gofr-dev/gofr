package gofr

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/rbac"
	"gofr.dev/pkg/gofr/testutil"
)

// rbacEndpointsOnly guards "/" with a permission and names no source, as an extractor config does.
const rbacEndpointsOnly = `{"endpoints":[{"path":"/","methods":["GET"],"requiredPermissions":["admin:read"]}]}`

func TestEnableRBACWithOptions(t *testing.T) {
	testCases := []struct {
		desc       string
		content    string
		env        map[string]string
		nilFn      bool
		noPath     bool     // the config sits at the default path and no path option is given
		noOptions  bool     // no option at all, so the call is EnableRBAC
		noFile     bool     // no config at the default path either
		perms      []string // what the extractor grants
		block      bool     // the extractor waits for its deadline
		wantErr    string
		wantStatus int
		wantCalls  int
	}{
		{desc: "a grant serves the request", content: rbacEndpointsOnly, perms: []string{"admin:read"},
			wantStatus: http.StatusOK, wantCalls: 1},
		{desc: "no grant is a 403", content: rbacEndpointsOnly,
			wantStatus: http.StatusForbidden, wantCalls: 1},
		{desc: "RBAC_EXTRACTOR_TIMEOUT sets the deadline", content: rbacEndpointsOnly, block: true,
			env: map[string]string{"RBAC_EXTRACTOR_TIMEOUT": "20ms"}, wantStatus: http.StatusServiceUnavailable, wantCalls: 1},
		{desc: "a nil extractor disables authorization", content: rbacEndpointsOnly, nilFn: true,
			wantErr: "extractor is nil", wantStatus: http.StatusOK},
		{desc: "a claim path beside the extractor disables authorization",
			content: `{"jwtClaimPath":"role",` + rbacEndpointsOnly[1:],
			wantErr: "cannot be combined with an extractor", wantStatus: http.StatusOK},
		{desc: "a timeout that does not parse disables authorization", content: rbacEndpointsOnly,
			env: map[string]string{"RBAC_EXTRACTOR_TIMEOUT": "soon"}, wantErr: "RBAC_EXTRACTOR_TIMEOUT", wantStatus: http.StatusOK},
		{desc: "a zero timeout disables authorization", content: rbacEndpointsOnly,
			env: map[string]string{"RBAC_EXTRACTOR_TIMEOUT": "0s"}, wantErr: "RBAC_EXTRACTOR_TIMEOUT", wantStatus: http.StatusOK},
		{desc: "a negative timeout disables authorization", content: rbacEndpointsOnly,
			env: map[string]string{"RBAC_EXTRACTOR_TIMEOUT": "-1s"}, wantErr: "RBAC_EXTRACTOR_TIMEOUT", wantStatus: http.StatusOK},
		{desc: "the default path is used when no path option is given", content: rbacEndpointsOnly, noPath: true,
			perms: []string{"admin:read"}, wantStatus: http.StatusOK, wantCalls: 1},
		{desc: "no options behaves as EnableRBAC", content: rbacGuardingRootYAML, noPath: true, noOptions: true,
			wantStatus: http.StatusUnauthorized},
		{desc: "no options and no default file disables authorization", noOptions: true, noPath: true, noFile: true,
			wantErr: "no RBAC config file found", wantStatus: http.StatusOK},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			dir := t.TempDir()
			path := writeRBACConfig(t, dir, "configs/rbac.yaml", tc.content)

			if tc.noPath {
				t.Chdir(dir)
			}

			if tc.noFile {
				require.NoError(t, os.Remove(path))
			}

			var a *App

			calls := 0

			var fn rbac.Extractor

			if !tc.nilFn {
				fn = func(c *container.Container, r *http.Request) ([]string, error) {
					calls++

					assert.Same(t, a.container, c, "TEST[%d], the extractor must get the app's container.\n%s", i, tc.desc)

					if tc.block {
						<-r.Context().Done()

						return nil, r.Context().Err()
					}

					return tc.perms, nil
				}
			}

			var err error

			// the app logs to the stderr it sees when it is created, so it is created inside the capture
			logs := testutil.StderrOutputForFunc(func() {
				a = newAuthTestApp(t)
				a.Config = config.NewMockConfig(tc.env)

				var opts []RBACOption

				if !tc.noOptions {
					opts = append(opts, WithRBACExtractor(fn))
				}

				if !tc.noPath {
					opts = append(opts, WithRBACConfigPath(path))
				}

				err = a.EnableRBACWithOptions(opts...)
			})

			if tc.wantErr == "" {
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.NotContains(t, logs, "DISABLED", "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				require.ErrorContains(t, err, tc.wantErr, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Contains(t, logs, "Authorization is DISABLED", "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			assert.Equal(t, tc.wantStatus, unauthenticatedStatus(t, a), "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantCalls, calls, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
