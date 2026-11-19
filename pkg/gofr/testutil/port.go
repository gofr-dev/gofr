package testutil

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// Environment variables the framework reads its listen ports from.
const (
	httpPortEnv    = "HTTP_PORT"
	metricsPortEnv = "METRICS_PORT"
	grpcPortEnv    = "GRPC_PORT"
)

// The range test ports are drawn from. It sits below every range an OS uses for the local port of
// an outgoing connection (Linux 32768-60999 by default, macOS and Windows 49152-65535), because a
// port from there is also handed to any connection another process opens, and the tests of the
// examples run as concurrent processes that each open Redis, SQL and HTTP connections: a port taken
// from the kernel by listening on :0 could be gone again before the app under test binds it.
const (
	minTestPort = 20000
	maxTestPort = 32000

	// maxPortAttempts bounds the search, which only repeats when a candidate is in use.
	maxPortAttempts = 100
)

// handedOutPorts remembers the ports this process was given, so that two tests of one process never
// share a port, even when the first has not bound it yet.
//
//nolint:gochecknoglobals // process-wide on purpose: it is shared by every test in the binary.
var (
	handedOutMu    sync.Mutex
	handedOutPorts = map[int]struct{}{}
)

// GetFreePort returns a free port that is ready to use for tests.
func GetFreePort(t *testing.T) int {
	t.Helper()

	port, err := reserveFreePort(t.Context())
	require.NoError(t, err, "Failed to get a free port.")

	return port
}

// reserveFreePort picks a random port in [minTestPort, maxTestPort) that nothing listens on. It is
// the plumbing behind GetFreePort and ReserveServerPorts, which differ only in how they report a
// failure.
func reserveFreePort(ctx context.Context) (int, error) {
	var lastErr error

	for range maxPortAttempts {
		port := minTestPort + rand.IntN(maxTestPort-minTestPort) //nolint:gosec // a spread, not a secret

		if alreadyHandedOut(port) {
			continue
		}

		// Probe the wildcard address: that is what the framework binds, and a listener on localhost
		// alone would miss a conflict on another interface.
		listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", ":"+strconv.Itoa(port))
		if err != nil {
			lastErr = err

			continue
		}

		if err := listener.Close(); err != nil {
			return 0, err
		}

		return port, nil
	}

	return 0, fmt.Errorf("no free port in %d-%d after %d attempts: %w", minTestPort, maxTestPort, maxPortAttempts, lastErr)
}

// alreadyHandedOut reports whether port was given out before, and marks it as given out if not.
func alreadyHandedOut(port int) bool {
	handedOutMu.Lock()
	defer handedOutMu.Unlock()

	if _, ok := handedOutPorts[port]; ok {
		return true
	}

	handedOutPorts[port] = struct{}{}

	return false
}

// ServiceConfigs holds the configuration details for different server components.
type ServiceConfigs struct {
	HTTPPort    int
	HTTPHost    string
	MetricsPort int
	MetricsHost string
	GRPCPort    int
	GRPCHost    string
}

// Get implements config.Config.
func (*ServiceConfigs) Get(string) string {
	return ""
}

// GetOrDefault implements config.Config.
func (*ServiceConfigs) GetOrDefault(string, string) string {
	return ""
}

// NewServerConfigs sets up server configurations for testing and returns a ServiceConfigs struct.
// It dynamically assigns free ports for HTTP, Metrics, and gRPC services, sets up environment variables for them,
// and returns a struct with the configured values.
func NewServerConfigs(t *testing.T) *ServiceConfigs {
	t.Helper()

	httpPort := GetFreePort(t)
	metricsPort := GetFreePort(t)
	grpcPort := GetFreePort(t)

	t.Setenv(httpPortEnv, strconv.Itoa(httpPort))
	t.Setenv(metricsPortEnv, strconv.Itoa(metricsPort))
	t.Setenv(grpcPortEnv, strconv.Itoa(grpcPort))

	return newServiceConfigs(httpPort, metricsPort, grpcPort)
}

// ReserveServerPorts is NewServerConfigs for a caller that has no *testing.T to fail on — a
// TestMain, which gets only a *testing.M, is the usual one. It reports a failure by returning an
// error, and sets the ports with os.Setenv rather than t.Setenv, there being no test scope to
// restore them at the end of. Prefer NewServerConfigs wherever a *testing.T is in hand.
func ReserveServerPorts() (*ServiceConfigs, error) {
	// A slice rather than a map: the order the ports are reserved in, and the name that appears in
	// an error, are then the same on every run.
	names := []string{httpPortEnv, metricsPortEnv, grpcPortEnv}
	ports := make([]int, len(names))

	for i, name := range names {
		port, err := reserveFreePort(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to reserve a free %s: %w", name, err)
		}

		ports[i] = port

		if err := os.Setenv(name, strconv.Itoa(port)); err != nil {
			return nil, fmt.Errorf("failed to set %s: %w", name, err)
		}
	}

	return newServiceConfigs(ports[0], ports[1], ports[2]), nil
}

func newServiceConfigs(httpPort, metricsPort, grpcPort int) *ServiceConfigs {
	return &ServiceConfigs{
		HTTPPort:    httpPort,
		HTTPHost:    fmt.Sprintf("http://localhost:%d", httpPort),
		MetricsPort: metricsPort,
		MetricsHost: fmt.Sprintf("http://localhost:%d", metricsPort),
		GRPCPort:    grpcPort,
		GRPCHost:    fmt.Sprintf("localhost:%d", grpcPort),
	}
}
