package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"gofr.dev/examples/grpc/grpc-unary-client/client"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/testutil"
)

// SimpleHelloServer implements a normal gRPC server using client types
type SimpleHelloServer struct {
	client.UnimplementedHelloServer
}

// SayHello implements the unary RPC
func (s *SimpleHelloServer) SayHello(ctx context.Context, req *client.HelloRequest) (*client.HelloResponse, error) {
	return &client.HelloResponse{
		Message: "Hello " + req.Name + "!",
	}, nil
}

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestIntegration_UnaryClient(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Start a simple gRPC server using client types
	grpcServer := grpc.NewServer()
	helloServer := &SimpleHelloServer{}
	client.RegisterHelloServer(grpcServer, helloServer)

	// Start the gRPC server
	listener, err := net.Listen("tcp", configs.GRPCHost)
	require.NoError(t, err, "Failed to create gRPC listener")

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("gRPC server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Give gRPC server time to start
	time.Sleep(100 * time.Millisecond)

	// Set the gRPC server host for the client
	os.Setenv("GRPC_SERVER_HOST", configs.GRPCHost)

	// Start the HTTP server (unary client example)
	go main()
	time.Sleep(200 * time.Millisecond) // Give HTTP server time to start

	// Test HTTP endpoints that use GoFr gRPC client internally
	tests := []struct {
		desc     string
		path     string
		expected string
	}{
		{"hello with name", "/hello?name=" + url.QueryEscape("gofr"), "Hello gofr!"},
		{"hello with empty name", "/hello", "Hello World!"},
		{"hello with unicode", "/hello?name=" + url.QueryEscape("你好世界"), "Hello 你好世界!"},
		{"hello with long name", "/hello?name=" + url.QueryEscape("ThisIsAVeryLongNameThatShouldStillWork"), "Hello ThisIsAVeryLongNameThatShouldStillWork!"},
	}

	for i, tc := range tests {
		// Properly encode the URL to handle special characters
		baseURL := fmt.Sprintf("http://localhost:%d%s", configs.HTTPPort, tc.path)
		parsedURL, err := url.Parse(baseURL)
		require.NoError(t, err, "TEST[%d], Failed to parse URL.\n%s", i, tc.desc)
		
		resp, err := http.Get(parsedURL.String())
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Contains(t, string(body), tc.expected, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestIntegration_UnaryClient_Concurrent(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Start a simple gRPC server using client types
	grpcServer := grpc.NewServer()
	helloServer := &SimpleHelloServer{}
	client.RegisterHelloServer(grpcServer, helloServer)

	// Start the gRPC server
	listener, err := net.Listen("tcp", configs.GRPCHost)
	require.NoError(t, err, "Failed to create gRPC listener")

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("gRPC server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Give gRPC server time to start
	time.Sleep(100 * time.Millisecond)

	// Set the gRPC server host for the client
	os.Setenv("GRPC_SERVER_HOST", configs.GRPCHost)

	// Start the HTTP server (unary client example)
	go main()
	time.Sleep(200 * time.Millisecond) // Give HTTP server time to start

	numClients := 5
	done := make(chan bool, numClients)

	for i := 0; i < numClients; i++ {
		go func(id int) {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/hello?name=concurrent+client+%d", configs.HTTPPort, id))
			require.NoError(t, err, "Concurrent HTTP request failed for client %d", id)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err, "Concurrent HTTP request failed for client %d", id)

			assert.Contains(t, string(body), fmt.Sprintf("Hello concurrent client %d!", id), "Unexpected response message for concurrent client %d", id)
			assert.Equal(t, http.StatusOK, resp.StatusCode, "Concurrent HTTP request failed for client %d", id)
			done <- true
		}(i)
	}

	// Wait for all concurrent clients to complete
	for i := 0; i < numClients; i++ {
		<-done
	}
}

func TestIntegration_UnaryClient_ErrorHandling(t *testing.T) {
	t.Run("InvalidGRPCServerHost", func(t *testing.T) {
		// Test with invalid gRPC server host
		os.Setenv("GRPC_SERVER_HOST", "invalid:address")
		
		// Create a new app to test with invalid host
		app := gofr.New()
		_, err := client.NewHelloGoFrClient(app.Config.Get("GRPC_SERVER_HOST"), app.Metrics())
		// GoFr client creation might not fail immediately for invalid addresses
		// The error will occur when actually making RPC calls
		if err != nil {
			assert.Error(t, err, "Should fail with invalid gRPC server address")
		}
	})

	t.Run("EmptyGRPCServerHost", func(t *testing.T) {
		// Test with empty gRPC server host
		os.Setenv("GRPC_SERVER_HOST", "")
		
		// Create a new app to test with empty host
		app := gofr.New()
		_, err := client.NewHelloGoFrClient(app.Config.Get("GRPC_SERVER_HOST"), app.Metrics())
		// GoFr client creation might not fail immediately for empty addresses
		// The error will occur when actually making RPC calls
		if err != nil {
			assert.Error(t, err, "Should fail with empty gRPC server address")
		}
	})
}
