package gofr

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/rbac"
	"gofr.dev/pkg/gofr/testutil"
)

// rbacEndpointsOnly guards "/" with a permission and names no source, as an extractor config does.
const rbacEndpointsOnly = `{"endpoints":[{"path":"/","methods":["GET"],"requiredPermissions":["admin:read"]}]}`

type rbacExtractorCase struct {
	desc       string
	content    string
	env        map[string]string
	nilFn      bool
	noPath     bool     // the config sits at the default path and no path is given
	noFile     bool     // no config at the default path either
	perms      []string // what the extractor grants
	block      bool     // the extractor waits for its deadline
	wantErr    string
	wantWarn   bool // the invalid-timeout warning is logged
	wantStatus int
	wantCalls  int
	wantWindow time.Duration // when set, the deadline the extractor saw is within a second of it
}

func rbacExtractorCases() []rbacExtractorCase {
	return []rbacExtractorCase{
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
		{desc: "a timeout that does not parse keeps RBAC on with the default", content: rbacEndpointsOnly,
			env: map[string]string{"RBAC_EXTRACTOR_TIMEOUT": "abc"}, wantWarn: true,
			wantStatus: http.StatusForbidden, wantCalls: 1, wantWindow: 5 * time.Second},
		{desc: "a zero timeout keeps RBAC on with the default", content: rbacEndpointsOnly,
			env: map[string]string{"RBAC_EXTRACTOR_TIMEOUT": "0"}, wantWarn: true,
			wantStatus: http.StatusForbidden, wantCalls: 1, wantWindow: 5 * time.Second},
		{desc: "a negative timeout keeps RBAC on with the default", content: rbacEndpointsOnly,
			env: map[string]string{"RBAC_EXTRACTOR_TIMEOUT": "-1s"}, wantWarn: true,
			wantStatus: http.StatusForbidden, wantCalls: 1, wantWindow: 5 * time.Second},
		{desc: "an unset timeout is the default without a warning", content: rbacEndpointsOnly,
			wantStatus: http.StatusForbidden, wantCalls: 1, wantWindow: 5 * time.Second},
		{desc: "the default path is used when no path is given", content: rbacEndpointsOnly, noPath: true,
			perms: []string{"admin:read"}, wantStatus: http.StatusOK, wantCalls: 1},
		{desc: "no default file disables authorization", noPath: true, noFile: true,
			wantErr: "no RBAC config file found", wantStatus: http.StatusOK},
	}
}

// pathsUnless is the config path argument: none when skip is set, so the default paths are searched.
func pathsUnless(skip bool, path string) []string {
	if skip {
		return nil
	}

	return []string{path}
}

func TestEnableRBACWithExtractor(t *testing.T) {
	for i, tc := range rbacExtractorCases() {
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

			var window time.Duration

			var fn rbac.Extractor

			if !tc.nilFn {
				fn = func(c *container.Container, r *http.Request) ([]string, error) {
					calls++

					deadline, _ := r.Context().Deadline()
					window = time.Until(deadline)

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
			var logs string

			// warnings and info go to stdout, errors to stderr
			stdout := testutil.StdoutOutputForFunc(func() {
				logs = testutil.StderrOutputForFunc(func() {
					a = newAuthTestApp(t)
					a.Config = config.NewMockConfig(tc.env)

					err = a.EnableRBACWithExtractor(fn, pathsUnless(tc.noPath, path)...)
				})
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
			assert.Equal(t, tc.wantWarn, strings.Contains(stdout, "invalid RBAC_EXTRACTOR_TIMEOUT"),
				"TEST[%d], Failed.\n%s", i, tc.desc)

			if tc.wantWindow > 0 {
				assert.InDelta(t, tc.wantWindow.Seconds(), window.Seconds(), 1, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}
