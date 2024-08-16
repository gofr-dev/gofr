package gofr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestNewGRPCServer(t *testing.T) {
	c := container.Container{
		Logger: logging.NewLogger(logging.DEBUG),
	}

	g := newGRPCServer(&c, 9999)

	assert.NotNil(t, g, "TEST Failed.\n")
}

func TestGRPC_ServerRun(t *testing.T) {
	testCases := []struct {
		desc       string
		grcpServer *grpc.Server
		port       int
		expLog     string
	}{
		{"net.Listen() error", nil, 99999, "error in starting gRPC server"},
		{"server.Serve() error", new(grpc.Server), 10000, "error in starting gRPC server"},
	}

	for i, tc := range testCases {
		f := func() {
			c := &container.Container{
				Logger: logging.NewLogger(logging.INFO),
			}

			g := &grpcServer{
				server: tc.grcpServer,
				port:   tc.port,
			}

			g.Run(c)
		}

		out := testutil.StderrOutputForFunc(f)

		assert.Contains(t, out, tc.expLog, "TEST[%d], Failed.\n", i)
	}
}

func TestGRPC_ServerShutdown(t *testing.T) {
	c := container.Container{
		Logger: logging.NewLogger(logging.DEBUG),
	}

	g := newGRPCServer(&c, 9999)

	go g.Run(&c)

	// Wait for the server to start
	time.Sleep(10 * time.Millisecond)

	// Create a context with a timeout to test the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := g.Shutdown(ctx)
	require.NoError(t, err, "TestGRPC_ServerShutdown Failed.\n")
}

func TestGRPC_ServerShutdown_ContextCanceled(t *testing.T) {
	c := container.Container{
		Logger: logging.NewLogger(logging.DEBUG),
	}

	g := newGRPCServer(&c, 9999)

	go g.Run(&c)

	// Wait for the server to start
	time.Sleep(10 * time.Millisecond)

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- g.Shutdown(ctx)
	}()

	// Cancel the context immediately
	cancel()

	err := <-errChan
	require.ErrorContains(t, err, "context canceled", "Expected error due to context cancellation")
}

func Test_injectContainer_Fails(t *testing.T) {
	// Case: container is an unaddressable or unexported field
	type fail struct {
		c1 *container.Container
	}

	c, _ := container.NewMockContainer(t)
	srv1 := &fail{}
	err := injectContainer(srv1, c)

	require.ErrorIs(t, err, errNonAddressable)
	require.Nil(t, srv1.c1)

	// Case: server is passed as unadressable(non-pointer)
	srv3 := fail{}
	out := testutil.StdoutOutputForFunc(func() {
		cont, _ := container.NewMockContainer(t)
		err = injectContainer(srv3, cont)

		assert.NoError(t, err)
	})

	assert.Contains(t, out, "cannot inject container into non-addressable implementation of `fail`, consider using pointer")
}

func Test_injectContainer(t *testing.T) {
	c, _ := container.NewMockContainer(t)

	// embedded container
	type success1 struct {
		*container.Container
	}

	srv1 := &success1{}
	err := injectContainer(srv1, c)

	require.NoError(t, err)
	require.NotNil(t, srv1.Container)

	// pointer type container
	type success2 struct {
		C *container.Container
	}

	srv2 := &success2{}
	err = injectContainer(srv2, c)

	require.NoError(t, err)
	require.NotNil(t, srv2.C)

	// non pointer type container
	type success3 struct {
		C container.Container
	}

	srv3 := &success3{}
	err = injectContainer(srv3, c)

	require.NoError(t, err)
	require.NotNil(t, srv3.C)
}
