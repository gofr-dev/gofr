package server

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func TestGoFrHelloServer_Creation(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	t.Run("HelloGoFrServerCreation", func(t *testing.T) {
		// Test GoFr's HelloGoFrServer creation
		app := gofr.New()
		helloServer := &HelloGoFrServer{}

		assert.NotNil(t, helloServer, "GoFr hello server should not be nil")
		assert.NotNil(t, app, "GoFr app should not be nil")

		// Test that it implements the GoFr interface
		var _ HelloServerWithGofr = helloServer
	})

	t.Run("HelloServerWrapperCreation", func(t *testing.T) {
		// Test GoFr's HelloServerWrapper creation
		app := gofr.New()
		helloServer := &HelloGoFrServer{}
		wrapper := &HelloServerWrapper{
			server: helloServer,
		}

		assert.NotNil(t, wrapper, "GoFr hello server wrapper should not be nil")
		assert.Equal(t, helloServer, wrapper.server, "Wrapper should contain the server")
		assert.NotNil(t, app, "GoFr app should not be nil")
	})
}

func TestGoFrHelloServer_Methods(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	// Test GoFr's hello server methods
	helloServer := &HelloGoFrServer{}
	ctx := createTestContext()

	t.Run("SayHelloMethodExists", func(t *testing.T) {
		// Test that GoFr's SayHello method exists and accepts correct parameters
		// Create a mock request in the context using the wrapper
		ctx.Request = &HelloRequestWrapper{
			HelloRequest: &HelloRequest{
				Name: "test-name",
			},
		}

		// Test GoFr's SayHello method signature
		resp, err := helloServer.SayHello(ctx)
		require.NoError(t, err, "GoFr SayHello should not fail")
		assert.NotNil(t, resp, "SayHello response should not be nil")

		// Verify the response type
		helloResp, ok := resp.(*HelloResponse)
		assert.True(t, ok, "Response should be HelloResponse")
		assert.Contains(t, helloResp.Message, "test-name", "Response should contain the name")
	})

	t.Run("SayHelloWithEmptyName", func(t *testing.T) {
		// Test GoFr's SayHello with empty name (should default to "World")
		ctx.Request = &HelloRequestWrapper{
			HelloRequest: &HelloRequest{
				Name: "",
			},
		}

		resp, err := helloServer.SayHello(ctx)
		require.NoError(t, err, "GoFr SayHello with empty name should not fail")
		assert.NotNil(t, resp, "SayHello response should not be nil")

		helloResp, ok := resp.(*HelloResponse)
		assert.True(t, ok, "Response should be HelloResponse")
		assert.Contains(t, helloResp.Message, "World", "Empty name should default to World")
	})
}

func TestGoFrHelloServer_ContextIntegration(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	helloServer := &HelloGoFrServer{}

	t.Run("ContextBinding", func(t *testing.T) {
		// Test GoFr's context binding functionality
		ctx := createTestContext()
		ctx.Request = &HelloRequestWrapper{
			HelloRequest: &HelloRequest{
				Name: "context-test",
			},
		}

		resp, err := helloServer.SayHello(ctx)
		require.NoError(t, err, "GoFr SayHello should not fail")
		assert.NotNil(t, resp, "SayHello response should not be nil")

		helloResp, ok := resp.(*HelloResponse)
		assert.True(t, ok, "Response should be HelloResponse")
		assert.Contains(t, helloResp.Message, "context-test", "Response should contain the context name")
	})

	t.Run("ContextTypeCompliance", func(t *testing.T) {
		// Test that GoFr's methods expect *gofr.Context specifically
		ctx := createTestContext()
		ctx.Request = &HelloRequestWrapper{
			HelloRequest: &HelloRequest{
				Name: "type-test",
			},
		}

		// Verify the method signature expects *gofr.Context
		var _ func(*gofr.Context) (any, error) = helloServer.SayHello

		// Ensure the call compiles (even if it fails at runtime)
		_, _ = helloServer.SayHello(ctx)
	})
}

func TestGoFrHelloServer_Registration(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	// Test GoFr's server registration functionality
	t.Run("RegisterHelloServerWithGofr", func(t *testing.T) {
		// Test GoFr's RegisterHelloServerWithGofr function
		app := gofr.New()
		helloServer := &HelloGoFrServer{}

		// This should not panic and should register the server
		assert.NotPanics(t, func() {
			RegisterHelloServerWithGofr(app, helloServer)
		}, "RegisterHelloServerWithGofr should not panic")
	})
}

func TestGoFrHelloServer_HealthIntegration(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	// Test GoFr's health integration
	t.Run("HealthIntegration", func(t *testing.T) {
		app := gofr.New()

		helloServer := &HelloGoFrServer{}

		// Register the server to set up health checks
		RegisterHelloServerWithGofr(app, helloServer)

		// Test that health server is properly integrated
		healthServer := getOrCreateHealthServer()
		assert.NotNil(t, healthServer, "Health server should be available")

		// Create a context for the health check
		ctx := createTestContext()

		// Check that Hello service is registered as serving
		req := &healthpb.HealthCheckRequest{
			Service: "Hello",
		}
		resp, err := healthServer.Check(ctx, req)
		require.NoError(t, err, "Health check should not fail")
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status, "Hello service should be serving")
	})
}

func TestGoFrHelloServer_MultipleInstances(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	// Test GoFr's multiple server instances
	t.Run("MultipleHelloServers", func(t *testing.T) {
		app := gofr.New()

		server1 := &HelloGoFrServer{}
		server2 := &HelloGoFrServer{}

		assert.NotNil(t, server1, "First GoFr hello server should not be nil")
		assert.NotNil(t, server2, "Second GoFr hello server should not be nil")
		// Check that they are different objects (different memory addresses)
		assert.True(t, server1 != server2, "GoFr hello server instances should be different objects")
		assert.NotNil(t, app, "GoFr app should not be nil")

		// Test that both can be created (but not registered to avoid duplicate service error)
		assert.NotNil(t, server1, "First server should be valid")
		assert.NotNil(t, server2, "Second server should be valid")
	})
}

func TestNewHelloGoFrServer(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	t.Run("NewHelloGoFrServerCreation", func(t *testing.T) {
		// Test GoFr's NewHelloGoFrServer function
		server := NewHelloGoFrServer()

		assert.NotNil(t, server, "NewHelloGoFrServer should not return nil")
		assert.NotNil(t, server.health, "Health server should be initialized")

		// Test that it implements the GoFr interface
		var _ HelloServerWithGofr = server
	})
}

func TestHelloServerWrapper_SayHello(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	t.Run("SayHelloWrapper", func(t *testing.T) {
		// Create a mock server implementation
		mockServer := &mockHelloServer{}

		// Create wrapper
		wrapper := &HelloServerWrapper{
			server:       mockServer,
			healthServer: getOrCreateHealthServer(),
			Container:    &container.Container{},
		}

		// Test SayHello method
		ctx := context.Background()
		req := &HelloRequest{Name: "test"}

		resp, err := wrapper.SayHello(ctx, req)

		require.NoError(t, err, "SayHello should not fail")
		assert.NotNil(t, resp, "Response should not be nil")
		assert.Equal(t, "Hello test!", resp.Message, "Response message should match")
	})

	t.Run("SayHelloWithError", func(t *testing.T) {
		// Create a mock server that returns an error
		mockServer := &mockHelloServerWithError{}

		// Create wrapper
		wrapper := &HelloServerWrapper{
			server:       mockServer,
			healthServer: getOrCreateHealthServer(),
			Container:    &container.Container{},
		}

		// Test SayHello method with error
		ctx := context.Background()
		req := &HelloRequest{Name: "error"}

		resp, err := wrapper.SayHello(ctx, req)

		assert.Error(t, err, "SayHello should return error")
		assert.Nil(t, resp, "Response should be nil on error")
		assert.Contains(t, err.Error(), "test error", "Error message should match")
	})

	t.Run("SayHelloWithWrongResponseType", func(t *testing.T) {
		// Create a mock server that returns wrong type
		mockServer := &mockHelloServerWrongType{}

		// Create wrapper
		wrapper := &HelloServerWrapper{
			server:       mockServer,
			healthServer: getOrCreateHealthServer(),
			Container:    &container.Container{},
		}

		// Test SayHello method with wrong response type
		ctx := context.Background()
		req := &HelloRequest{Name: "wrong"}

		resp, err := wrapper.SayHello(ctx, req)

		assert.Error(t, err, "SayHello should return error for wrong response type")
		assert.Nil(t, resp, "Response should be nil on error")
		assert.Contains(t, err.Error(), "unexpected response type", "Error message should indicate wrong type")
	})
}

func TestHelloServerWrapper_getGofrContext(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	t.Run("getGofrContext", func(t *testing.T) {
		// Create wrapper
		wrapper := &HelloServerWrapper{
			Container: &container.Container{},
		}

		// Test getGofrContext method
		ctx := context.Background()
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: "test"},
		}

		gofrCtx := wrapper.getGofrContext(ctx, req)

		assert.NotNil(t, gofrCtx, "GoFr context should not be nil")
		assert.Equal(t, ctx, gofrCtx.Context, "Context should match")
		assert.Equal(t, &container.Container{}, gofrCtx.Container, "Container should match")
		assert.Equal(t, req, gofrCtx.Request, "Request should match")
	})
}

func TestInstrumentedStream(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	t.Run("InstrumentedStreamContext", func(t *testing.T) {
		// Create a mock server stream
		mockStream := &mockServerStream{}

		// Create instrumented stream
		gofrCtx := createTestContext()
		stream := &instrumentedStream{
			ServerStream: mockStream,
			ctx:          gofrCtx,
			method:       "/Hello/Test",
		}

		// Test Context method
		ctx := stream.Context()
		assert.Equal(t, gofrCtx, ctx, "Context should match GoFr context")
	})

	t.Run("InstrumentedStreamSendMsg", func(t *testing.T) {
		// Create a mock server stream
		mockStream := &mockServerStream{}

		// Create instrumented stream
		gofrCtx := createTestContext()
		stream := &instrumentedStream{
			ServerStream: mockStream,
			ctx:          gofrCtx,
			method:       "/Hello/Test",
		}

		// Test SendMsg method
		msg := &HelloResponse{Message: "test"}
		err := stream.SendMsg(msg)

		assert.NoError(t, err, "SendMsg should not fail")
		assert.True(t, mockStream.sendMsgCalled, "SendMsg should be called on underlying stream")
	})

	t.Run("InstrumentedStreamRecvMsg", func(t *testing.T) {
		// Create a mock server stream
		mockStream := &mockServerStream{}

		// Create instrumented stream
		gofrCtx := createTestContext()
		stream := &instrumentedStream{
			ServerStream: mockStream,
			ctx:          gofrCtx,
			method:       "/Hello/Test",
		}

		// Test RecvMsg method
		msg := &HelloRequest{}
		err := stream.RecvMsg(msg)

		assert.NoError(t, err, "RecvMsg should not fail")
		assert.True(t, mockStream.recvMsgCalled, "RecvMsg should be called on underlying stream")
	})
}

// Mock implementations for testing
type mockHelloServer struct{}

func (m *mockHelloServer) SayHello(ctx *gofr.Context) (any, error) {
	req := &HelloRequest{}
	err := ctx.Bind(req)
	if err != nil {
		return nil, err
	}
	return &HelloResponse{Message: "Hello " + req.Name + "!"}, nil
}

type mockHelloServerWithError struct{}

func (m *mockHelloServerWithError) SayHello(ctx *gofr.Context) (any, error) {
	return nil, fmt.Errorf("test error")
}

type mockHelloServerWrongType struct{}

func (m *mockHelloServerWrongType) SayHello(ctx *gofr.Context) (any, error) {
	return "wrong type", nil
}

type mockServerStream struct {
	sendMsgCalled bool
	recvMsgCalled bool
}

func (m *mockServerStream) SendMsg(msg interface{}) error {
	m.sendMsgCalled = true
	return nil
}

func (m *mockServerStream) RecvMsg(msg interface{}) error {
	m.recvMsgCalled = true
	return nil
}

func (m *mockServerStream) SetHeader(metadata.MD) error {
	return nil
}

func (m *mockServerStream) SendHeader(metadata.MD) error {
	return nil
}

func (m *mockServerStream) SetTrailer(metadata.MD) {
}

func (m *mockServerStream) Context() context.Context {
	return context.Background()
}
