package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"gofr.dev/examples/grpc/grpc-unary-server/server"
	"gofr.dev/pkg/gofr"
	gofrGrpc "gofr.dev/pkg/gofr/grpc"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestIntegration_UnaryServer(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(100 * time.Millisecond) // Giving some time to start the server

	// Create gRPC client connection
	conn, err := grpc.Dial(configs.GRPCHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to connect to unary server")
	defer conn.Close()

	client := server.NewHelloClient(conn)

	tests := []struct {
		desc     string
		name     string
		expected string
	}{
		{"hello with name", "gofr", "Hello gofr!"},
		{"hello with empty name", "", "Hello World!"},
		{"hello with special chars", "!@#$%^&*", "Hello !@#$%^&*!"},
		{"hello with unicode", "你好世界", "Hello 你好世界!"},
		{"hello with long name", "ThisIsAVeryLongNameThatShouldStillWork", "Hello ThisIsAVeryLongNameThatShouldStillWork!"},
	}

	for i, tc := range tests {
		resp, err := client.SayHello(context.Background(), &server.HelloRequest{
			Name: tc.name,
		})

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.expected, resp.Message, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestIntegration_UnaryServer_Concurrent(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(100 * time.Millisecond) // Giving some time to start the server

	// Create gRPC client connection
	conn, err := grpc.Dial(configs.GRPCHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to connect to unary server")
	defer conn.Close()

	client := server.NewHelloClient(conn)

	numClients := 5
	done := make(chan bool, numClients)

	for i := 0; i < numClients; i++ {
		go func(id int) {
			resp, err := client.SayHello(context.Background(), &server.HelloRequest{
				Name: "concurrent client " + string(rune(id)),
			})
			require.NoError(t, err, "Concurrent SayHello RPC failed for client %d", id)
			assert.Contains(t, resp.Message, "concurrent client", "Unexpected response message for concurrent client %d", id)
			done <- true
		}(i)
	}

	// Wait for all concurrent clients to complete
	for i := 0; i < numClients; i++ {
		<-done
	}
}

func TestIntegration_UnaryServer_ErrorHandling(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(100 * time.Millisecond) // Giving some time to start the server

	// Create gRPC client connection
	conn, err := grpc.Dial(configs.GRPCHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to connect to unary server")
	defer conn.Close()

	client := server.NewHelloClient(conn)

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.SayHello(ctx, &server.HelloRequest{
			Name: "cancel test",
		})
		assert.Error(t, err, "Context cancellation should return error")
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("TimeoutHandling", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond) // Very short timeout
		defer cancel()

		_, err := client.SayHello(ctx, &server.HelloRequest{
			Name: "timeout test",
		})
		assert.Error(t, err, "Timeout should return error")
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestHelloProtoMethods(t *testing.T) {
	// Test HelloRequest methods
	req := &server.HelloRequest{Name: "John"}
	assert.Equal(t, "John", req.GetName())
	assert.Equal(t, "name:\"John\"", req.String())

	// Test HelloResponse methods
	resp := &server.HelloResponse{Message: "Hello World"}
	assert.Equal(t, "Hello World", resp.GetMessage())
	assert.Equal(t, "message:\"Hello World\"", resp.String())
}

func TestIntegration_UnaryServer_RateLimited(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		app := gofr.New()

		rateLimiterCfg := middleware.RateLimiterConfig{
			RequestsPerSecond: 2,
			Burst:             2,
		}

		app.AddGRPCUnaryInterceptors(
			gofrGrpc.UnaryRateLimitInterceptor(ctx, rateLimiterCfg, app.Logger(), app.Metrics()),
		)

		server.RegisterHelloServerWithGofr(app, server.NewHelloGoFrServer())

		app.Run()
	}()

	time.Sleep(200 * time.Millisecond)

	conn, err := grpc.NewClient(configs.GRPCHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to connect to rate-limited server")

	defer conn.Close()

	client := server.NewHelloClient(conn)

	// Should succeed
	for i := 0; i < 2; i++ {
		resp, callErr := client.SayHello(context.Background(), &server.HelloRequest{Name: "gofr"})
		require.NoError(t, callErr, "Request %d should succeed within burst", i+1)
		assert.Equal(t, "Hello gofr!", resp.GetMessage())
	}

	// Should hit the rate limit
	_, callErr := client.SayHello(context.Background(), &server.HelloRequest{Name: "gofr"})
	require.Error(t, callErr, "3rd request should be rate limited")

	st, ok := status.FromError(callErr)
	require.True(t, ok, "Error should be a gRPC status")
	assert.Equal(t, codes.ResourceExhausted, st.Code(), "Should return RESOURCE_EXHAUSTED")
}
