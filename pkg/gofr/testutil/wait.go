package testutil

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// alivePath duplicates service.AlivePath. It cannot be imported here: pkg/gofr/service's own
	// tests import this package, so testutil -> service would be an import cycle.
	alivePath = "/.well-known/alive"

	serverStartTimeout = 30 * time.Second
	serverPollInterval = 100 * time.Millisecond

	// stdoutReadBuffer is the chunk size the stdout capture reads with; a log line is far shorter.
	stdoutReadBuffer = 4096
)

// WaitForHTTPServer blocks until the server at host answers its liveness probe, and fails the
// test if it never does. Use it instead of sleeping a fixed duration after `go main()`: a sleep
// long enough for a loaded CI runner is wasted on every other run, and one short enough to be
// cheap races the listener.
func WaitForHTTPServer(t *testing.T, host string) {
	t.Helper()

	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, host+alivePath, http.NoBody)
		if err != nil {
			return false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}

		defer resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, serverStartTimeout, serverPollInterval, "HTTP server at %s did not start in time", host)
}

// WaitForGRPCServer blocks until a gRPC connection to addr reaches the ready state, and fails the
// test if it never does. gRPC examples that serve no HTTP port have no liveness endpoint to poll,
// and a bare TCP dial is not enough — the kernel accepts connections from the moment the listener
// exists, so it succeeds before the server is serving. Reaching ready means the HTTP/2 handshake
// completed, which only happens once it is.
func WaitForGRPCServer(t *testing.T, addr string) {
	t.Helper()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create a gRPC client for %s", addr)

	defer conn.Close()

	ctx, cancel := context.WithTimeout(t.Context(), serverStartTimeout)
	defer cancel()

	conn.Connect()

	for state := conn.GetState(); state != connectivity.Ready; state = conn.GetState() {
		require.True(t, conn.WaitForStateChange(ctx, state), "gRPC server at %s did not start in time", addr)
	}
}

// WaitForStdoutContains runs f with os.Stdout captured and blocks until the captured output
// contains substr, returning everything captured up to that point. It fails the test if substr
// never appears.
//
// Use it for an app whose only startup signal is a log line. An app that registers no routes -
// a subscriber-only one, say - leaves App.httpRegistered false, so GoFr starts no HTTP server
// and WaitForHTTPServer would wait for a port that never opens. StdoutOutputForFunc cannot serve
// here either: it reads the pipe only once f has returned, so f would have to sleep to give the
// app time to log, which is the race this package exists to remove.
func WaitForStdoutContains(t *testing.T, substr string, f func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err, "failed to create a pipe to capture stdout")

	old := os.Stdout
	os.Stdout = w

	var (
		mu       sync.Mutex
		captured strings.Builder
	)

	found := captureUntil(r, substr, &mu, &captured)

	f()

	var timedOut bool

	select {
	case <-found:
	case <-time.After(serverStartTimeout):
		timedOut = true
	}

	// Restoring before the assertion below keeps a failure message on the real stdout. Closing the
	// writer ends the reader; the app under test goes on writing to a closed pipe, as it does with
	// StdoutOutputForFunc too.
	os.Stdout = old
	_ = w.Close()

	mu.Lock()
	out := captured.String()
	mu.Unlock()

	if timedOut {
		require.Fail(t, "expected output never appeared on stdout",
			"waited %s for %q, captured:\n%s", serverStartTimeout, substr, out)
	}

	return out
}

// captureUntil drains r into out and closes the returned channel as soon as out holds substr. It
// reads concurrently rather than in one ReadAll at the end, so a caller can act on the signal
// while the process that writes it is still running.
func captureUntil(r *os.File, substr string, mu *sync.Mutex, out *strings.Builder) <-chan struct{} {
	found := make(chan struct{})

	go func() {
		var once sync.Once

		buf := make([]byte, stdoutReadBuffer)

		for {
			n, err := r.Read(buf)

			if n > 0 {
				mu.Lock()
				out.Write(buf[:n])
				hit := strings.Contains(out.String(), substr)
				mu.Unlock()

				if hit {
					once.Do(func() { close(found) })
				}
			}

			if err != nil {
				return
			}
		}
	}()

	return found
}
