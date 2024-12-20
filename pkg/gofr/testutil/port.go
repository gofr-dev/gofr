package testutil

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// GetFreePort asks the kernel for a free open port that is ready to use for tests.
func GetFreePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "Failed to get a free port.")

	port := listener.Addr().(*net.TCPAddr).Port

	err = listener.Close()
	require.NoError(t, err, "Failed to get a free port.")

	return port
}
