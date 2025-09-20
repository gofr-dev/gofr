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
	lc := net.ListenConfig{}
	listener, err := lc.Listen(t.Context(), "tcp", fmt.Sprintf("localhost:%d", port))
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

// ...existing code...

func TestServiceConfigs_Get(t *testing.T) {
	cfg := &ServiceConfigs{
		HTTPPort:    8080,
		HTTPHost:    "http://localhost:8080",
		MetricsPort: 9090,
		MetricsHost: "http://localhost:9090",
		GRPCPort:    50051,
		GRPCHost:    "localhost:50051",
	}

	// Test Get method - it should always return empty string as per current implementation
	testCases := []struct {
		desc string
		key  string
	}{
		{
			desc: "empty key",
			key:  "",
		},
		{
			desc: "non-empty key",
			key:  "HTTP_PORT",
		},
		{
			desc: "random key",
			key:  "RANDOM_KEY",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := cfg.Get(tc.key)
			assert.Empty(t, result, "Get should return empty string for all keys")
		})
	}
}

func TestServiceConfigs_GetOrDefault(t *testing.T) {
	cfg := &ServiceConfigs{
		HTTPPort:    8080,
		HTTPHost:    "http://localhost:8080",
		MetricsPort: 9090,
		MetricsHost: "http://localhost:9090",
		GRPCPort:    50051,
		GRPCHost:    "localhost:50051",
	}

	// Test GetOrDefault method - it should always return empty string as per current implementation
	testCases := []struct {
		desc         string
		key          string
		defaultValue string
	}{
		{
			desc:         "empty key and default",
			key:          "",
			defaultValue: "",
		},
		{
			desc:         "non-empty key with default",
			key:          "HTTP_PORT",
			defaultValue: "8080",
		},
		{
			desc:         "random key with default",
			key:          "RANDOM_KEY",
			defaultValue: "some-default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := cfg.GetOrDefault(tc.key, tc.defaultValue)
			assert.Empty(t, result, "GetOrDefault should return empty string for all keys and defaults")
		})
	}
}
