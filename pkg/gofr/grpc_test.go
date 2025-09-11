package gofr

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// nonExitingMockLogger embeds MockLogger but overrides Fatal methods to not exit.
type nonExitingMockLogger struct {
	*logging.MockLogger
}

func (n *nonExitingMockLogger) Fatal(args ...any) {
	// Just log as error instead of exiting
	n.MockLogger.Error(args...)
}

func (n *nonExitingMockLogger) Fatalf(format string, args ...any) {
	// Just log as error instead of exiting
	n.MockLogger.Errorf(format, args...)
}

// setupGRPCMetricExpectations sets up mock expectations for gRPC metrics.
func setupGRPCMetricExpectations(mockMetrics *container.MockMetrics) {
	mockMetrics.EXPECT().NewGauge("grpc_server_status", "gRPC server status (1=running, 0=stopped)").AnyTimes()
	mockMetrics.EXPECT().NewCounter("grpc_server_errors_total", "Total gRPC server errors").AnyTimes()
	mockMetrics.EXPECT().NewCounter("grpc_services_registered_total", "Total gRPC services registered").AnyTimes()
	mockMetrics.EXPECT().SetGauge("grpc_server_status", gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "grpc_server_errors_total").AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
}

// setupTestGRPCServer creates a mock container and gRPC server for testing.
func setupTestGRPCServer(t *testing.T, port int, enableReflection bool) (*container.Container, *container.Mocks, *grpcServer) {
	t.Helper()

	c, mocks := container.NewMockContainer(t)
	setupGRPCMetricExpectations(mocks.Metrics)

	cfg := createMockGRPCConfig(t, port, enableReflection)
	g, err := newGRPCServer(c, port, cfg)
	require.NoError(t, err)

	return c, mocks, g
}

// createTestInterceptors creates sample interceptors for testing.
func createTestInterceptors() []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		},
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		},
	}
}

//nolint:thelper // createMockGRPCConfig creates a mock config with gRPC-specific settings.
func createMockGRPCConfig(t *testing.T, port int, enableReflection bool) config.Config {
	// Use a default metrics port if t is nil (for cases where we don't need a real free port)
	metricsPort := 8080
	if t != nil {
		metricsPort = testutil.GetFreePort(t)
	}

	configMap := map[string]string{
		"GRPC_PORT":    strconv.Itoa(port),
		"METRICS_PORT": strconv.Itoa(metricsPort),
	}

	if enableReflection {
		configMap["GRPC_ENABLE_REFLECTION"] = "true"
	} else {
		configMap["GRPC_ENABLE_REFLECTION"] = "false"
	}

	return config.NewMockConfig(configMap)
}

func TestNewGRPCServer(t *testing.T) {
	c, mocks, g := setupTestGRPCServer(t, 9999, false)

	assert.NotNil(t, g, "TEST Failed.\n")
	assert.NotNil(t, c, "Container should not be nil")
	assert.NotNil(t, mocks, "Mocks should not be nil")
}

func TestGRPCServer_AddServerOptions(t *testing.T) {
	_, _, g := setupTestGRPCServer(t, 9999, false)

	option1 := grpc.ConnectionTimeout(5 * time.Second)
	option2 := grpc.MaxRecvMsgSize(1024 * 1024)

	g.addServerOptions(option1, option2)

	assert.Len(t, g.options, 2)
}

func TestGRPCServer_AddUnaryInterceptors(t *testing.T) {
	_, _, g := setupTestGRPCServer(t, 9999, false)

	interceptors := createTestInterceptors()
	g.addUnaryInterceptors(interceptors...)

	assert.Len(t, g.interceptors, 4) // 2 default + 2 test interceptors
}

func TestGRPCServer_CreateServer(t *testing.T) {
	_, _, g := setupTestGRPCServer(t, 9999, false)

	err := g.createServer()
	require.NoError(t, err)
	assert.NotNil(t, g.server)
}

func TestGRPCServer_RegisterService(t *testing.T) {
	_, _, g := setupTestGRPCServer(t, 9999, false)

	err := g.createServer()
	require.NoError(t, err)

	healthServer := health.NewServer()
	desc := &grpc_health_v1.Health_ServiceDesc

	g.server.RegisterService(desc, healthServer)

	services := g.server.GetServiceInfo()
	_, ok := services["grpc.health.v1.Health"]
	assert.True(t, ok, "health service should be registered")
}

func TestGRPC_ServerRun(t *testing.T) {
	// Test invalid port case
	t.Run("net.Listen() error", func(t *testing.T) {
		out := testutil.StderrOutputForFunc(func() {
			c, mocks := container.NewMockContainer(t)
			setupGRPCMetricExpectations(mocks.Metrics)

			// Add expectations for error scenarios
			mocks.Metrics.EXPECT().IncrementCounter(gomock.Any(), "grpc_server_errors_total").AnyTimes()
			mocks.Metrics.EXPECT().SetGauge("grpc_server_status", gomock.Any()).AnyTimes()

			cfg := createMockGRPCConfig(t, 99999, false) // Invalid port
			g := &grpcServer{
				port:   99999, // Invalid port
				config: cfg,
			}

			// Create the server first
			err := g.createServer()
			if err != nil {
				t.Fatalf("Failed to create server: %v", err)
			}

			// Run the server in a goroutine
			done := make(chan bool)
			serverRoutine := func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("Server panicked: %v", r)
					}
					done <- true
				}()
				g.Run(c)
			}

			go serverRoutine()

			// Give some time for the server to attempt startup
			time.Sleep(500 * time.Millisecond)

			// Shutdown the server
			_ = g.Shutdown(t.Context())

			// Wait for the goroutine to finish
			<-done
		})

		// Assert that the expected log message was captured
		assert.Contains(t, out, "error in starting gRPC server", "Expected log message not found for invalid port test")
	})

	// Test port occupied case
	t.Run("server.Serve() error", func(t *testing.T) {
		// First, occupy a port
		occupiedPort := testutil.GetFreePort(t)
		listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", fmt.Sprintf(":%d", occupiedPort))
		require.NoError(t, err)

		defer listener.Close()

		out := testutil.StderrOutputForFunc(func() {
			c, mocks := container.NewMockContainer(t)
			setupGRPCMetricExpectations(mocks.Metrics)

			// Add expectations for error scenarios
			mocks.Metrics.EXPECT().IncrementCounter(gomock.Any(), "grpc_server_errors_total").AnyTimes()
			mocks.Metrics.EXPECT().SetGauge("grpc_server_status", gomock.Any()).AnyTimes()

			// Replace the logger with our custom logger that doesn't exit on Fatal
			mockLogger := &nonExitingMockLogger{MockLogger: logging.NewMockLogger(logging.DEBUG).(*logging.MockLogger)}
			c.Logger = mockLogger

			cfg := createMockGRPCConfig(t, occupiedPort, false) // Use the occupied port
			g := &grpcServer{
				port:   occupiedPort, // Use the occupied port
				config: cfg,
			}

			// Create the server first
			err := g.createServer()
			if err != nil {
				t.Fatalf("Failed to create server: %v", err)
			}

			// Run the server - this should call Fatalf but not exit
			g.Run(c)
		})

		// Assert that the expected log message was captured
		assert.Contains(t, out, "gRPC port", "Expected log message not found for occupied port test")
	})
}

func TestGRPC_ServerShutdown(t *testing.T) {
	c, _, g := setupTestGRPCServer(t, 9999, false)

	go g.Run(c)

	// Wait for the server to start
	time.Sleep(10 * time.Millisecond)

	// Create a context with a timeout to test the shutdown
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	err := g.Shutdown(ctx)
	require.NoError(t, err, "TestGRPC_ServerShutdown Failed.\n")
}

func TestGRPC_ServerShutdown_ContextCanceled(t *testing.T) {
	c, _, g := setupTestGRPCServer(t, 9999, false)

	go g.Run(c)

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
	_, _, g := setupTestGRPCServer(t, 9999, false)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	err := g.Shutdown(ctx)
	assert.NoError(t, err, "Expected shutdown to succeed even if server was not started")
}

func TestGRPC_ServerRun_WithInterceptorAndOptions(t *testing.T) {
	freePort := testutil.GetFreePort(t)
	c, _, g := setupTestGRPCServer(t, freePort, false)

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

	app := New()
	app.container = c
	app.grpcServer = g

	// Add the server options and interceptors to the app
	app.AddGRPCServerOptions(
		grpc.ConnectionTimeout(5*time.Second),
		grpc.MaxRecvMsgSize(1024*1024))

	// Set interceptors
	app.AddGRPCUnaryInterceptors(interceptor1, interceptor2)

	// Create the server first
	err := app.grpcServer.createServer()
	require.NoError(t, err)

	// Start the server in a goroutine
	go app.grpcServer.Run(c)

	// Wait for the server to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown the server immediately to avoid timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	defer cancel()

	err = app.grpcServer.Shutdown(ctx)
	require.NoError(t, err)

	// Verify that the server was created with the interceptors and options
	assert.NotNil(t, app.grpcServer.server)
	assert.Len(t, app.grpcServer.interceptors, 4) // 2 default + 2 test interceptors
	assert.Len(t, app.grpcServer.options, 4)      // 2 test options + 2 default (interceptor) options
}

func TestApp_WithReflection(t *testing.T) {
	c, _, g := setupTestGRPCServer(t, 9999, true) // Enable reflection

	app := New()
	app.container = c
	app.grpcServer = g

	err := app.grpcServer.createServer()
	require.NoError(t, err)

	services := app.grpcServer.server.GetServiceInfo()
	_, ok := services["grpc.reflection.v1alpha.ServerReflection"]
	assert.True(t, ok, "reflection service should be registered")
}
