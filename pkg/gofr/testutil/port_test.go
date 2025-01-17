package testutil

import (
	"fmt"
	"net"
	"os"
	"strconv"
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

func TestNewServerConfigs(t *testing.T) {
	env := NewServerConfigs(t)

	// Check HTTP_PORT
	httpPortEnv := os.Getenv("HTTP_PORT")
	require.NotEmpty(t, httpPortEnv, "HTTP_PORT environment variable should be set")
	assert.Equal(t, strconv.Itoa(env.HTTPPort), httpPortEnv, "HTTP_PORT should match the configured HTTPPort")
	assert.NotZero(t, env.HTTPPort, "HTTPPort should not be zero")
	assert.Equal(t, env.HTTPHost, "http://localhost:"+httpPortEnv, "HTTPHost should match environment variable")

	// Check METRICS_PORT
	metricsPortEnv := os.Getenv("METRICS_PORT")
	require.NotEmpty(t, metricsPortEnv, "METRICS_PORT environment variable should be set")
	assert.Equal(t, strconv.Itoa(env.MetricsPort), metricsPortEnv, "METRICS_PORT should match the configured MetricsPort")
	assert.NotZero(t, env.MetricsPort, "MetricsPort should not be zero")
	assert.Equal(t, env.MetricsHost, "http://localhost:"+metricsPortEnv, "MetricsHost should match environment variable")

	// Check GRPC_PORT
	grpcPortEnv := os.Getenv("GRPC_PORT")
	require.NotEmpty(t, grpcPortEnv, "GRPC_PORT environment variable should be set")
	assert.Equal(t, strconv.Itoa(env.GRPCPort), grpcPortEnv, "GRPC_PORT should match the configured GRPCPort")
	assert.NotZero(t, env.GRPCPort, "GRPCPort should not be zero")
	assert.Equal(t, env.GRPCHost, "localhost:"+grpcPortEnv, "GRPCHost should match environment variable")
}
