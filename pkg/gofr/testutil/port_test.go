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

func TestReserveServerPorts(t *testing.T) {
	// os.Setenv rather than t.Setenv, so restore what this test displaces.
	for _, key := range []string{"HTTP_PORT", "METRICS_PORT", "GRPC_PORT"} {
		t.Setenv(key, os.Getenv(key))
	}

	configs, err := ReserveServerPorts()
	require.NoError(t, err, "ReserveServerPorts should not fail")

	tests := []struct {
		desc     string
		envKey   string
		port     int
		host     string
		expected string
	}{
		{"http", "HTTP_PORT", configs.HTTPPort, configs.HTTPHost, "http://localhost:"},
		{"metrics", "METRICS_PORT", configs.MetricsPort, configs.MetricsHost, "http://localhost:"},
		{"grpc", "GRPC_PORT", configs.GRPCPort, configs.GRPCHost, "localhost:"},
	}

	for i, tc := range tests {
		assert.NotZero(t, tc.port, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, strconv.Itoa(tc.port), os.Getenv(tc.envKey), "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.expected+strconv.Itoa(tc.port), tc.host, "TEST[%d], Failed.\n%s", i, tc.desc)

		// The reserved port must actually be bindable.
		lc := net.ListenConfig{}

		listener, err := lc.Listen(t.Context(), "tcp", fmt.Sprintf("localhost:%d", tc.port))
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		require.NoError(t, listener.Close(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

// ephemeralPortRange is the range the OS hands out as the local port of an outgoing connection. On
// Linux it is configurable and read from the kernel; elsewhere, or where it is unreadable, the
// fallback is a superset of what macOS and Windows use, so it is safe.
func ephemeralPortRange() (low, high int) {
	low, high = 32768, 65535

	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_local_port_range")
	if err != nil {
		return low, high
	}

	var l, h int
	if _, err := fmt.Sscanf(string(data), "%d %d", &l, &h); err != nil {
		return low, high
	}

	return l, h
}

func Test_reserveFreePort(t *testing.T) {
	low, high := ephemeralPortRange()

	ports := make([]int, 0, 200)

	for range cap(ports) {
		port, err := reserveFreePort(t.Context())
		require.NoError(t, err)

		ports = append(ports, port)
	}

	tests := []struct {
		desc  string
		check func(t *testing.T, port int)
	}{
		{"outside the range outgoing connections draw their local port from", func(t *testing.T, port int) {
			t.Helper()

			assert.False(t, port >= low && port <= high, "port %d is inside the ephemeral range %d-%d", port, low, high)
		}},
		{"bindable on all interfaces, as the framework does", func(t *testing.T, port int) {
			t.Helper()

			listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", ":"+strconv.Itoa(port))
			require.NoError(t, err)
			require.NoError(t, listener.Close())
		}},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			for _, port := range ports {
				tc.check(t, port)
			}
		})
	}

	t.Run("never the same port twice in one process", func(t *testing.T) {
		seen := make(map[int]bool, len(ports))

		for _, port := range ports {
			require.False(t, seen[port], "port %d was handed out twice", port)

			seen[port] = true
		}
	})
}
