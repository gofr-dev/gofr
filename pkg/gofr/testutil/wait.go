package testutil

import (
	"context"
	"net/http"
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
