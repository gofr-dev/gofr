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

	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func testLogger() logging.Logger {
	return logging.NewMockLogger(logging.ERROR)
}

func testFS() file.FileSystem {
	return file.NewLocalFileSystem(logging.NewMockLogger(logging.ERROR))
}

func writeTokenFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestNewFileTokenAuthConfig(t *testing.T) {
	validPath := writeTokenFile(t, "initial-token")
	fs := testFS()

	tests := []struct {
		name        string
		fs          file.FileSystem
		path        string
		interval    time.Duration
		expectError bool
	}{
		{"valid token", fs, validPath, 0, false},
		{"nil file system", nil, validPath, 0, true},
		{"missing file", fs, filepath.Join(t.TempDir(), "does-not-exist"), 0, true},
		{"empty file", fs, writeTokenFile(t, "   \n\t "), 0, true},
		{"negative interval defaults to 30s", fs, validPath, -1 * time.Second, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewFileTokenAuthConfig(tc.fs, testLogger(), tc.path, tc.interval)
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

	cfg, err := NewFileTokenAuthConfig(testFS(), testLogger(), path, 50*time.Millisecond)
	require.NoError(t, err)

	assert.NoError(t, cfg.Close())
	assert.NoError(t, cfg.Close())
}

func TestFileTokenAuthConfig_RefreshPicksUpRotation(t *testing.T) {
	path := writeTokenFile(t, "token-v1")

	cfg, err := NewFileTokenAuthConfig(testFS(), testLogger(), path, 20*time.Millisecond)
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

	cfg, err := NewFileTokenAuthConfig(testFS(), testLogger(), path, time.Hour)
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	svc := service.NewHTTPService(srv.URL, testLogger(), nil, cfg)

	resp, err := svc.Get(context.Background(), "", nil)
	require.NoError(t, err)

	_ = resp.Body.Close()

	assert.Equal(t, "Bearer secret-token", seenAuth)
}

func TestFileTokenAuthConfig_RejectsExistingAuthHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	path := writeTokenFile(t, "tok")

	cfg, err := NewFileTokenAuthConfig(testFS(), testLogger(), path, time.Hour)
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	svc := service.NewHTTPService(srv.URL, testLogger(), nil, cfg)

	resp, err := svc.GetWithHeaders(context.Background(), "", nil, map[string]string{
		"Authorization": "Bearer pre-existing",
	})
	require.Error(t, err)

	// The error must carry service.AuthErr (parity with BasicAuth/OAuth/APIKey)
	// while still exposing the underlying sentinel through the wrap.
	var authErr service.AuthErr
	require.ErrorAs(t, err, &authErr)
	require.ErrorIs(t, err, errAuthHeaderPresent)

	if resp != nil {
		_ = resp.Body.Close()
	}
}

func TestFileTokenAuthConfig_InjectsBearerHeaderAllVerbs(t *testing.T) {
	var seenAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	path := writeTokenFile(t, "verb-token")

	cfg, err := NewFileTokenAuthConfig(testFS(), testLogger(), path, time.Hour)
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	svc := service.NewHTTPService(srv.URL, testLogger(), nil, cfg)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() (*http.Response, error)
	}{
		{"GET", func() (*http.Response, error) { return svc.Get(ctx, "", nil) }},
		{"POST", func() (*http.Response, error) { return svc.Post(ctx, "", nil, nil) }},
		{"PUT", func() (*http.Response, error) { return svc.Put(ctx, "", nil, nil) }},
		{"PATCH", func() (*http.Response, error) { return svc.Patch(ctx, "", nil, nil) }},
		{"DELETE", func() (*http.Response, error) { return svc.Delete(ctx, "", nil) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seenAuth = ""

			resp, err := tc.call()
			require.NoError(t, err)

			_ = resp.Body.Close()

			assert.Equal(t, "Bearer verb-token", seenAuth)
		})
	}
}

// TestFileTokenAuthConfig_RefreshFailureLogsWarning verifies that a failed
// background refresh is surfaced to the logger instead of being swallowed
// silently (the cached token continues to serve).
func TestFileTokenAuthConfig_RefreshFailureLogsWarning(t *testing.T) {
	path := writeTokenFile(t, "token-v1")

	out := testutil.StdoutOutputForFunc(func() {
		cfg, err := NewFileTokenAuthConfig(testFS(), logging.NewMockLogger(logging.WARN), path, 20*time.Millisecond)
		require.NoError(t, err)

		t.Cleanup(func() { _ = cfg.Close() })

		// Remove the token file so the next refresh tick fails to read it.
		require.NoError(t, os.Remove(path))

		assert.Eventually(t, func() bool {
			// Cached token must remain available despite refresh failures.
			tok, tokErr := cfg.currentToken()
			return tokErr == nil && tok == "token-v1"
		}, time.Second, 20*time.Millisecond)

		// Give the refresh loop time to log at least one warning.
		time.Sleep(100 * time.Millisecond)
	})

	assert.Contains(t, out, "failed to refresh token")
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

	cfg, err := NewFileTokenAuthConfig(testFS(), testLogger(), path, time.Hour)
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

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
