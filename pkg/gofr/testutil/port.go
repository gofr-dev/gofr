package testutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// Environment variables the framework reads its listen ports from.
const (
	httpPortEnv    = "HTTP_PORT"
	metricsPortEnv = "METRICS_PORT"
	grpcPortEnv    = "GRPC_PORT"
)

// GetFreePort asks the kernel for a free open port that is ready to use for tests.
func GetFreePort(t *testing.T) int {
	t.Helper()

	port, err := reserveFreePort(t.Context())
	require.NoError(t, err, "Failed to get a free port.")

	return port
}

// reserveFreePort asks the kernel for a free open port. It is the plumbing behind GetFreePort and
// ReserveServerPorts, which differ only in how they report a failure.
func reserveFreePort(ctx context.Context) (int, error) {
	lc := net.ListenConfig{}

	listener, err := lc.Listen(ctx, "tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	port := listener.Addr().(*net.TCPAddr).Port

	return port, listener.Close()
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
	ports := map[string]int{httpPortEnv: 0, metricsPortEnv: 0, grpcPortEnv: 0}

	for name := range ports {
		port, err := reserveFreePort(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to reserve a free %s: %w", name, err)
		}

		ports[name] = port

		if err := os.Setenv(name, strconv.Itoa(port)); err != nil {
			return nil, fmt.Errorf("failed to set %s: %w", name, err)
		}
	}

	return newServiceConfigs(ports[httpPortEnv], ports[metricsPortEnv], ports[grpcPortEnv]), nil
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
