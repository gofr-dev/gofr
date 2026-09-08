//go:build gofr_nogrpc

package gofr

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

// A binary built with -tags gofr_nogrpc must still construct, authenticate and
// shut down. Only the gRPC server is gone. Everything here is unreachable in the
// default build.

func TestGRPCDisabled_AppStillBuildsAndShutsDown(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	t.Setenv("HTTP_PORT", strconv.Itoa(testutil.GetFreePort(t)))

	app := New()

	require.NotNil(t, app.grpcServer, "the runner is still constructed, so Shutdown has something to call")
	require.NoError(t, app.grpcServer.Shutdown(context.Background()))
}

// startGRPCServer is gated on grpcRegistered, and the only thing that sets it is
// RegisterService, which does not exist in this build. This asserts the gate
// rather than the absence of the method, which the compiler already enforces.
func TestGRPCDisabled_ServerNeverStarts(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	t.Setenv("HTTP_PORT", strconv.Itoa(testutil.GetFreePort(t)))

	app := New()

	assert.False(t, app.grpcRegistered, "nothing in this build can register a gRPC service")
}

// HTTP auth is not gRPC's to take away: EnableBasicAuth and friends must still
// install their HTTP middleware, with the gRPC half quietly skipped.
func TestGRPCDisabled_HTTPAuthStillWorks(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	t.Setenv("HTTP_PORT", strconv.Itoa(testutil.GetFreePort(t)))

	app := New()

	require.NotPanics(t, func() {
		app.EnableBasicAuth("user", "pass")
		app.EnableAPIKeyAuth("key1")
	})
}
