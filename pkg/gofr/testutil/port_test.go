package testutil

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFreePort(t *testing.T) {
	port := GetFreePort(t)
	assert.Positive(t, port, "Expected port to be greater than 0")

	// Test that the port is actually free by trying to listen on it
	listener, err := net.Listen("tcp", "localhost:"+fmt.Sprintf("%d", port))
	require.NoError(t, err, "Expected to be able to listen on the free port")

	listener.Close()
}
