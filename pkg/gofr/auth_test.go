package gofr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/testutil"
)

// newAuthTestApp returns an App whose router answers 200 on "/" so a test can tell
// whether an auth middleware was installed by the status of an unauthenticated request.
func newAuthTestApp(t *testing.T) *App {
	t.Helper()

	a := &App{
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
			port:   testutil.GetFreePort(t),
		},
		container: container.NewContainer(config.NewMockConfig(nil)),
	}

	a.httpServer.router.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	return a
}

// unauthenticatedStatus sends a request carrying no credentials through the app's router.
func unauthenticatedStatus(t *testing.T, a *App) int {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	a.httpServer.router.ServeHTTP(rec, req)

	return rec.Code
}

func TestEnableBasicAuthWithError(t *testing.T) {
	tests := []struct {
		desc       string
		args       []string
		wantErr    string
		wantStatus int
	}{
		{desc: "credential pairs install the middleware", args: []string{"user", "pass"}, wantStatus: http.StatusUnauthorized},
		{desc: "no credentials", args: nil, wantErr: "no credentials provided", wantStatus: http.StatusOK},
		{desc: "odd number of arguments", args: []string{"user", "pass", "orphan"},
			wantErr: "invalid number of arguments", wantStatus: http.StatusOK},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			a := newAuthTestApp(t)

			err := a.EnableBasicAuthWithError(tc.args...)

			if tc.wantErr == "" {
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				require.ErrorContains(t, err, tc.wantErr, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			assert.Equal(t, tc.wantStatus, unauthenticatedStatus(t, a), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestEnableOAuthWithError(t *testing.T) {
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer jwks.Close()

	tests := []struct {
		desc       string
		endpoint   string
		wantErr    string
		wantStatus int
	}{
		{desc: "valid endpoint installs the middleware", endpoint: jwks.URL + "/.well-known/jwks.json",
			wantStatus: http.StatusUnauthorized},
		{desc: "unparsable URL", endpoint: "://bad", wantErr: "invalid JWKS endpoint URL", wantStatus: http.StatusOK},
		{desc: "empty URL", endpoint: "", wantErr: "missing scheme or host", wantStatus: http.StatusOK},
		{desc: "path only", endpoint: "/.well-known/jwks.json", wantErr: "missing scheme or host", wantStatus: http.StatusOK},
		{desc: "scheme without host", endpoint: "http://", wantErr: "missing scheme or host", wantStatus: http.StatusOK},
		{desc: "unsupported scheme", endpoint: "ftp://host/.well-known/jwks.json",
			wantErr: "unsupported scheme", wantStatus: http.StatusOK},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			a := newAuthTestApp(t)

			err := a.EnableOAuthWithError(tc.endpoint, 600)

			if tc.wantErr == "" {
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.NotNil(t, a.container.GetHTTPService("gofr_oauth"), "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				require.ErrorContains(t, err, tc.wantErr, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Nil(t, a.container.GetHTTPService("gofr_oauth"), "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			assert.Equal(t, tc.wantStatus, unauthenticatedStatus(t, a), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
