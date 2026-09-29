package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

func testLogger() logging.Logger {
	return logging.NewMockLogger(logging.ERROR)
}

func writeTokenFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// stdoutCapture redirects os.Stdout to a pipe and drains it in a background
// goroutine, so callers can poll String() while a concurrent writer is still
// emitting — unlike testutil.StdoutOutputForFunc, which only returns the output
// after the function under test has fully returned.
type stdoutCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newConcurrentStdoutCapture(t *testing.T) *stdoutCapture {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	old := os.Stdout
	os.Stdout = w

	c := &stdoutCapture{}
	done := make(chan struct{})

	go func() {
		defer close(done)

		b := make([]byte, 1024)

		for {
			n, readErr := r.Read(b)
			if n > 0 {
				c.mu.Lock()
				c.buf.Write(b[:n])
				c.mu.Unlock()
			}

			if readErr != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		os.Stdout = old
		_ = w.Close()

		<-done

		_ = r.Close()
	})

	return c
}

func (c *stdoutCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.buf.String()
}

func TestNewFileTokenAuthConfig(t *testing.T) {
	validPath := writeTokenFile(t, "initial-token")
	missingPath := filepath.Join(t.TempDir(), "does-not-exist")
	emptyPath := writeTokenFile(t, "   \n\t ")

	tests := []struct {
		name        string
		opts        []FileTokenAuthOption
		expectError bool
	}{
		{"valid token", []FileTokenAuthOption{WithTokenFilePath(validPath)}, false},
		{"missing file", []FileTokenAuthOption{WithTokenFilePath(missingPath)}, true},
		{"empty file", []FileTokenAuthOption{WithTokenFilePath(emptyPath)}, true},
		{"negative interval is ignored", []FileTokenAuthOption{WithTokenFilePath(validPath), WithRefreshInterval(-1 * time.Second)}, false},
		{"empty path option is ignored (falls back to default, which won't exist)", []FileTokenAuthOption{WithTokenFilePath("")}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewFileTokenAuthConfig(tc.opts...)
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

	cfg, err := NewFileTokenAuthConfig(WithTokenFilePath(path), WithRefreshInterval(50*time.Millisecond))
	require.NoError(t, err)

	assert.NoError(t, cfg.Close())
	assert.NoError(t, cfg.Close())
}

func TestFileTokenAuthConfig_RefreshPicksUpRotation(t *testing.T) {
	path := writeTokenFile(t, "token-v1")

	cfg, err := NewFileTokenAuthConfig(WithTokenFilePath(path), WithRefreshInterval(20*time.Millisecond))
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

	cfg, err := NewFileTokenAuthConfig(WithTokenFilePath(path), WithRefreshInterval(time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	svc := NewHTTPService(srv.URL, testLogger(), nil, cfg)

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

	cfg, err := NewFileTokenAuthConfig(WithTokenFilePath(path), WithRefreshInterval(time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	svc := NewHTTPService(srv.URL, testLogger(), nil, cfg)

	resp, err := svc.GetWithHeaders(context.Background(), "", nil, map[string]string{
		"Authorization": "Bearer pre-existing",
	})
	require.Error(t, err)

	// The error must carry AuthErr (parity with BasicAuth/OAuth/APIKey)
	// while still exposing the underlying sentinel through the wrap.
	var authErr AuthErr
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

	cfg, err := NewFileTokenAuthConfig(WithTokenFilePath(path), WithRefreshInterval(time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	svc := NewHTTPService(srv.URL, testLogger(), nil, cfg)
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
		{"QUERY", func() (*http.Response, error) { return svc.Query(ctx, "", nil, nil) }},
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

// TestFileTokenAuthConfig_RefreshFailureLogsWarning verifies that the logger
// injected via Observable receives WARN-level entries when background
// refresh fails (cached token continues to serve).
func TestFileTokenAuthConfig_RefreshFailureLogsWarning(t *testing.T) {
	path := writeTokenFile(t, "token-v1")

	// Capture stdout concurrently so we can wait until the warning is actually
	// observed rather than sleeping a fixed interval and hoping the background
	// refresh goroutine was scheduled in time (which is only heuristic).
	logs := newConcurrentStdoutCapture(t)

	cfg, err := NewFileTokenAuthConfig(WithTokenFilePath(path), WithRefreshInterval(10*time.Millisecond))
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	// Inject the logger the way NewHTTPService would via the Observable hook.
	cfg.SetLogger(logging.NewMockLogger(logging.WARN))

	// Remove the token file so the next refresh tick fails to read it.
	require.NoError(t, os.Remove(path))

	// Before any refresh runs, the eagerly-read cached token is available.
	tok, tokErr := cfg.currentToken()
	require.NoError(t, tokErr)
	require.Equal(t, "token-v1", tok)

	// Wait until a refresh failure is actually logged. This passes the moment the
	// warning appears and only fails if it never appears within the ceiling, so a
	// briefly-starved goroutine no longer flakes the test.
	assert.Eventually(t, func() bool {
		return strings.Contains(logs.String(), "failed to refresh token")
	}, 2*time.Second, 10*time.Millisecond)

	// After repeated failed refreshes, the cached token must still be served —
	// a vanished token file must not clear what was already loaded.
	tok, tokErr = cfg.currentToken()
	require.NoError(t, tokErr)
	assert.Equal(t, "token-v1", tok)
}

// TestFileTokenAuthConfig_ObservableInjectionViaNewHTTPService verifies that
// NewHTTPService wires the logger automatically via the Observable hook — the
// user no longer has to plumb it through the constructor.
func TestFileTokenAuthConfig_ObservableInjectionViaNewHTTPService(t *testing.T) {
	path := writeTokenFile(t, "token-v1")

	logs := newConcurrentStdoutCapture(t)

	cfg, err := NewFileTokenAuthConfig(WithTokenFilePath(path), WithRefreshInterval(10*time.Millisecond))
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	// Registering the option through NewHTTPService is what should inject
	// the logger — no explicit SetLogger call here.
	_ = NewHTTPService("http://example.invalid", logging.NewMockLogger(logging.WARN), nil, cfg)

	require.NoError(t, os.Remove(path))

	// Wait until the auto-injected logger actually surfaces a refresh failure,
	// rather than sleeping a fixed interval and hoping the goroutine was scheduled.
	assert.Eventually(t, func() bool {
		return strings.Contains(logs.String(), "failed to refresh token")
	}, 2*time.Second, 10*time.Millisecond)
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

	cfg, err := NewFileTokenAuthConfig(WithTokenFilePath(path), WithRefreshInterval(time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() { _ = cfg.Close() })

	pool := &ConnectionPoolConfig{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	}

	svc := NewHTTPService(srv.URL, testLogger(), nil, cfg, pool)

	resp, err := svc.Get(context.Background(), "", nil)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
