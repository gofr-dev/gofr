package gofr

import (
	"net/http"
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

func TestEnableRBACWithExtractor(t *testing.T) {
	testCases := []struct {
		desc       string
		content    string
		env        map[string]string
		nilFn      bool
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
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			path := writeRBACConfig(t, t.TempDir(), "rbac.json", tc.content)

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
				err = a.EnableRBACWithExtractor(fn, path)
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
