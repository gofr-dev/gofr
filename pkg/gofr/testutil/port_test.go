package testutil

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFreePort(t *testing.T) {
	port := GetFreePort(t)
	assert.Positive(t, port, "Expected port to be greater than 0")

	// Test that the port is actually free by trying to listen on it
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	require.NoError(t, err, "Expected to be able to listen on the free port")

	_ = listener.Close()
}

func TestServerConfigsProvider(t *testing.T) {
	env := ServerConfigsProvider(t)

	assert.NotZero(t, env.HTTPPort, "HTTPPort should not be zero")
	assert.Equal(t, env.HTTPHost, "http://localhost:"+os.Getenv("HTTP_PORT"), "HTTPHost should match environment variable")

	assert.NotZero(t, env.MetricsPort, "MetricsPort should not be zero")
	assert.Equal(t, env.MetricsHost, "http://localhost:"+os.Getenv("METRICS_PORT"), "MetricsHost should match environment variable")

	assert.NotZero(t, env.GRPCPort, "GRPCPort should not be zero")
	assert.Equal(t, env.GRPCHost, "localhost:"+os.Getenv("GRPC_PORT"), "GRPCHost should match environment variable")
}
