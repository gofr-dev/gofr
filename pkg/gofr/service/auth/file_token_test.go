package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

func testLogger() service.Logger {
	return logging.NewMockLogger(logging.ERROR)
}

func writeTokenFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestNewFileTokenAuthConfig(t *testing.T) {
	validPath := writeTokenFile(t, "initial-token")

	tests := []struct {
		name        string
		path        string
		interval    time.Duration
		expectError bool
	}{
		{"valid token", validPath, 0, false},
		{"missing file", filepath.Join(t.TempDir(), "does-not-exist"), 0, true},
		{"empty file", writeTokenFile(t, "   \n\t "), 0, true},
		{"negative interval defaults to 30s", validPath, -1 * time.Second, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewFileTokenAuthConfig(tc.path, tc.interval)
			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, cfg)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			t.Cleanup(func() { _ = cfg.Close() })

			tok, err := cfg.currentToken()
			require.NoError(t, err)
			assert.Equal(t, "initial-token", tok)
		})
	}
}

func TestFileTokenAuthConfig_CloseIsIdempotent(t *testing.T) {
	path := writeTokenFile(t, "tok")

	cfg, err := NewFileTokenAuthConfig(path, 50*time.Millisecond)
	require.NoError(t, err)

	assert.NoError(t, cfg.Close())
	assert.NoError(t, cfg.Close())
}

func TestFileTokenAuthConfig_RefreshPicksUpRotation(t *testing.T) {
	path := writeTokenFile(t, "token-v1")

	cfg, err := NewFileTokenAuthConfig(path, 20*time.Millisecond)
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	require.NoError(t, os.WriteFile(path, []byte("token-v2"), 0o600))

	assert.Eventually(t, func() bool {
		tok, err := cfg.currentToken()
		return err == nil && tok == "token-v2"
	}, time.Second, 20*time.Millisecond)
}

func TestFileTokenAuthConfig_InjectsBearerHeader(t *testing.T) {
	var seenAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	path := writeTokenFile(t, "secret-token")

	cfg, err := NewFileTokenAuthConfig(path, time.Hour)
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	svc := service.NewHTTPService(srv.URL, testLogger(), nil, cfg)

	_, err = svc.Get(context.Background(), "", nil)
	require.NoError(t, err)

	assert.Equal(t, "Bearer secret-token", seenAuth)
}

func TestFileTokenAuthConfig_RejectsExistingAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	path := writeTokenFile(t, "tok")

	cfg, err := NewFileTokenAuthConfig(path, time.Hour)
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	svc := service.NewHTTPService(srv.URL, testLogger(), nil, cfg)

	_, err = svc.GetWithHeaders(context.Background(), "", nil, map[string]string{
		"Authorization": "Bearer pre-existing",
	})
	require.Error(t, err)
}

// TestFileTokenAuthConfig_WorksWithConnectionPoolConfig locks in the regression
// reviewer flagged on PR #3244: an auth decorator defined outside the service
// package must still let ConnectionPoolConfig reach the underlying *httpService,
// otherwise pool / circuit-breaker / retry options silently no-op when combined
// with this auth type.
func TestFileTokenAuthConfig_WorksWithConnectionPoolConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer combo-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	path := writeTokenFile(t, "combo-token")

	cfg, err := NewFileTokenAuthConfig(path, time.Hour)
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	pool := &service.ConnectionPoolConfig{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	}

	svc := service.NewHTTPService(srv.URL, testLogger(), nil, cfg, pool)

	resp, err := svc.Get(context.Background(), "", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
