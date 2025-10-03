package testutil

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// GetFreePort asks the kernel for a free open port that is ready to use for tests.
func GetFreePort(t *testing.T) int {
	t.Helper()

	lc := net.ListenConfig{}
	listener, err := lc.Listen(t.Context(), "tcp", "localhost:0")
	require.NoError(t, err, "Failed to get a free port.")

	port := listener.Addr().(*net.TCPAddr).Port

	err = listener.Close()
	require.NoError(t, err, "Failed to get a free port.")

	return port
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

	t.Setenv("HTTP_PORT", strconv.Itoa(httpPort))
	t.Setenv("METRICS_PORT", strconv.Itoa(metricsPort))
	t.Setenv("GRPC_PORT", strconv.Itoa(grpcPort))

	return &ServiceConfigs{
		HTTPPort:    httpPort,
		HTTPHost:    fmt.Sprintf("http://localhost:%d", httpPort),
		MetricsPort: metricsPort,
		MetricsHost: fmt.Sprintf("http://localhost:%d", metricsPort),
		GRPCPort:    grpcPort,
		GRPCHost:    fmt.Sprintf("localhost:%d", grpcPort),
	}
}
