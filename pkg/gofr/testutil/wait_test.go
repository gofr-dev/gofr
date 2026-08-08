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
	}{
		{"line already written when f returns", 0},
		{"line written well after f returns", 300 * time.Millisecond},
	}

	for i, tc := range tests {
		out := WaitForStdoutContains(t, "ready", func() {
			go func() {
				time.Sleep(tc.writeWait)

				fmt.Fprintln(os.Stdout, "starting up\nready to serve")
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
