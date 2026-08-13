package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// startHTTPServerAfter starts an HTTP server serving the liveness path on addr once delay has
// elapsed, mimicking an example whose startup is slower than the caller's first request.
func startHTTPServerAfter(t *testing.T, delay time.Duration) (host string) {
	t.Helper()

	port := GetFreePort(t)

	mux := http.NewServeMux()
	mux.HandleFunc(alivePath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: fmt.Sprintf("localhost:%d", port), Handler: mux, ReadHeaderTimeout: time.Second}

	go func() {
		time.Sleep(delay)

		_ = srv.ListenAndServe()
	}()

	t.Cleanup(func() {
		_ = srv.Close()
	})

	return fmt.Sprintf("http://localhost:%d", port)
}

// startGRPCServerAfter starts a bare gRPC server on addr once delay has elapsed.
func startGRPCServerAfter(t *testing.T, delay time.Duration) (addr string) {
	t.Helper()

	port := GetFreePort(t)
	addr = fmt.Sprintf("localhost:%d", port)
	srv := grpc.NewServer()

	go func() {
		time.Sleep(delay)

		listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
		if err != nil {
			return
		}

		_ = srv.Serve(listener)
	}()

	t.Cleanup(srv.Stop)

	return addr
}

func TestWaitForHTTPServer(t *testing.T) {
	tests := []struct {
		desc       string
		startDelay time.Duration
	}{
		{"server already listening", 0},
		{"server starts after the first probe", 300 * time.Millisecond},
	}

	for i, tc := range tests {
		host := startHTTPServerAfter(t, tc.startDelay)

		WaitForHTTPServer(t, host)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, host+alivePath, http.NoBody)
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestWaitForStdoutContains(t *testing.T) {
	tests := []struct {
		desc      string
		writeWait time.Duration
		// writes are logged in order, with a pause between them so each lands in its own read.
		// Splitting the awaited substring across two of them puts it across a read boundary, which
		// the incremental scan has to carry over rather than miss.
		writes []string
	}{
		{"line already written when f returns", 0, []string{"starting up\nready to serve\n"}},
		{"line written well after f returns", 300 * time.Millisecond, []string{"starting up\nready to serve\n"}},
		{"substring straddles a read boundary", 0, []string{"starting up\nrea", "dy to serve\n"}},
	}

	for i, tc := range tests {
		out := WaitForStdoutContains(t, "ready", func() {
			go func() {
				time.Sleep(tc.writeWait)

				for _, w := range tc.writes {
					fmt.Fprint(os.Stdout, w)
					// Long enough for the reader to drain what has been written so far.
					time.Sleep(100 * time.Millisecond)
				}
			}()
		})

		require.Contains(t, out, "ready to serve", "TEST[%d], Failed.\n%s", i, tc.desc)
		// Everything written before the awaited line is returned as well.
		require.Contains(t, out, "starting up", "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestWaitForGRPCServer(t *testing.T) {
	tests := []struct {
		desc       string
		startDelay time.Duration
	}{
		{"server already listening", 0},
		{"server starts after the first probe", 300 * time.Millisecond},
	}

	for i, tc := range tests {
		addr := startGRPCServerAfter(t, tc.startDelay)

		WaitForGRPCServer(t, addr)

		conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		require.NoError(t, conn.Close(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestAwaitGRPCServer(t *testing.T) {
	tests := []struct {
		desc       string
		startDelay time.Duration
	}{
		{"server already listening", 0},
		{"server starts after the first probe", 300 * time.Millisecond},
	}

	for i, tc := range tests {
		addr := startGRPCServerAfter(t, tc.startDelay)

		require.NoError(t, AwaitGRPCServer(addr), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

// TestWaitForGRPC_Timeout covers the failure path the exported wrappers share. It exercises
// waitForGRPC rather than WaitForGRPCServer or AwaitGRPCServer so the deadline can be a short one:
// the wrappers hardcode serverStartTimeout, and a test is not going to wait 30s to see it.
func TestWaitForGRPC_Timeout(t *testing.T) {
	// A port reserved and released: nothing is listening, so the wait cannot succeed.
	addr := fmt.Sprintf("localhost:%d", GetFreePort(t))

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	err := waitForGRPC(ctx, addr)

	require.ErrorIs(t, err, errServerNotReady)
	require.ErrorContains(t, err, addr, "the error should name the server that did not come up")
}
