package gofr

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestNewGRPCServer(t *testing.T) {
	c := container.Container{
		Logger: logging.NewLogger(logging.DEBUG),
	}
	cfg := testutil.NewServerConfigs(t)
	g := newGRPCServer(&c, 9999, cfg)

	assert.NotNil(t, g, "TEST Failed.\n")
}
func TestGRPC_ServerRun(t *testing.T) {
	testCases := []struct {
		desc   string
		port   int
		expLog string
	}{
		{"net.Listen() error", 99999, "error in starting gRPC server"},   // Invalid port
		{"server.Serve() error", 10000, "error in starting gRPC server"}, // Port occupied
	}

	for i, tc := range testCases {
		f := func() {
			c := &container.Container{
				Logger: logging.NewLogger(logging.INFO),
			}

			// If testing "server.Serve() error", occupy the port first
			if tc.port == 10000 {
				lc := net.ListenConfig{}

				listener, err := lc.Listen(t.Context(), "tcp", fmt.Sprintf(":%d", tc.port))
				if err != nil {
					t.Fatalf("Failed to occupy port %d: %v", tc.port, err)
				}

				defer listener.Close() // Ensure cleanup
			}

			g := &grpcServer{
				port:   tc.port,
				config: getConfigs(t),
			}

			go func() {
				g.Run(c)
			}()

			// Give some time for the server to attempt startup
			time.Sleep(500 * time.Millisecond)

			_ = g.Shutdown(t.Context()) // Ensure shutdown
		}

		out := testutil.StderrOutputForFunc(f)
		assert.Contains(t, out, tc.expLog, "TEST[%d], Failed.\n", i)
	}
}

func TestGRPC_ServerShutdown(t *testing.T) {
	c := container.Container{
		Logger: logging.NewLogger(logging.DEBUG),
	}
	cfg := testutil.NewServerConfigs(t)
	g := newGRPCServer(&c, 9999, cfg)

	go g.Run(&c)

	// Wait for the server to start
	time.Sleep(10 * time.Millisecond)

	// Create a context with a timeout to test the shutdown
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	err := g.Shutdown(ctx)
	require.NoError(t, err, "TestGRPC_ServerShutdown Failed.\n")
}

func TestGRPC_ServerShutdown_ContextCanceled(t *testing.T) {
	c := container.Container{
		Logger: logging.NewLogger(logging.DEBUG),
	}
	cfg := testutil.NewServerConfigs(t)
	g := newGRPCServer(&c, 9999, cfg)

	go g.Run(&c)

	// Wait for the server to start
	time.Sleep(10 * time.Millisecond)

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(t.Context())

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

func TestGRPC_Shutdown_BeforeStart(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	c := &container.Container{Logger: logger}

	cfg := testutil.NewServerConfigs(t)
	g := newGRPCServer(c, 9999, cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	err := g.Shutdown(ctx)
	assert.NoError(t, err, "Expected shutdown to succeed even if server was not started")
}

func TestGRPC_ServerRun_WithInterceptorAndOptions(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	c := container.Container{Logger: logger}

	var interceptorExecutions []string

	// Define interceptors
	interceptor1 := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		interceptorExecutions = append(interceptorExecutions, "interceptor1")
		return handler(ctx, req)
	}

	interceptor2 := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		interceptorExecutions = append(interceptorExecutions, "interceptor2")
		return handler(ctx, req)
	}

	cnf := testutil.NewServerConfigs(t)
	app := New()

	// Add the server options and interceptors to the app
	app.AddGRPCServerOptions(
		grpc.ConnectionTimeout(5*time.Second),
		grpc.MaxRecvMsgSize(1024*1024))

	// Set interceptors
	app.AddGRPCUnaryInterceptors(interceptor1, interceptor2)

	// Register Health service
	healthServer := health.NewServer()

	app.grpcServer.createServer()

	grpc_health_v1.RegisterHealthServer(app.grpcServer.server, healthServer)

	// Start the server
	go app.grpcServer.Run(&c)

	defer func() {
		_ = app.Shutdown(t.Context())
	}()

	// Set the health status
	healthServer.SetServingStatus("healthCheck", grpc_health_v1.HealthCheckResponse_SERVING)

	// Wait for server to start
	addr := fmt.Sprintf("127.0.0.1:%d", cnf.GRPCPort)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	defer conn.Close()

	// Test Health Check directly
	healthClient := grpc_health_v1.NewHealthClient(conn)
	healthResp, err := healthClient.Check(t.Context(), &grpc_health_v1.HealthCheckRequest{Service: "healthCheck"})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, healthResp.Status)

	// Verify interceptors were called in order
	assert.Equal(t, []string{"interceptor1", "interceptor2"}, interceptorExecutions)
}

func TestApp_WithReflection(t *testing.T) {
	c := &container.Container{
		Logger: logging.NewLogger(logging.DEBUG),
	}
	app := New()
	app.container = c
	cfg := testutil.NewServerConfigs(t)
	app.grpcServer = newGRPCServer(c, 9999, cfg)
	app.grpcServer.createServer()

	services := app.grpcServer.server.GetServiceInfo()
	_, ok := services["grpc.reflection.v1alpha.ServerReflection"]
	assert.True(t, ok, "reflection service should be registered")
}
