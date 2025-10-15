package client

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

// createTestContext creates a test gofr.Context
func createTestContext() *gofr.Context {
	container := &container.Container{}
	return &gofr.Context{
		Context:   context.Background(),
		Container: container,
	}
}

func TestGoFrHelloClientWrapper_Creation(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	t.Run("NewHelloGoFrClient", func(t *testing.T) {
		// Set HTTP port to avoid port conflicts
		os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
		
		// Test GoFr's NewHelloGoFrClient function
		conn, err := grpc.Dial(configs.GRPCHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err, "Connection creation should not fail immediately")
		defer conn.Close()

		app := gofr.New()
		helloClient, err := NewHelloGoFrClient(configs.GRPCHost, app.Metrics())
		require.NoError(t, err, "GoFr hello client creation should not fail")
		assert.NotNil(t, helloClient, "GoFr hello client should not be nil")
		
		// Test that it implements the GoFr interface
		var _ HelloGoFrClient = helloClient
	})

	t.Run("HelloClientWrapperInterface", func(t *testing.T) {
		// Set HTTP port to avoid port conflicts
		os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
		
		// Test GoFr's interface compliance
		conn, err := grpc.Dial(configs.GRPCHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err, "Connection creation should not fail immediately")
		defer conn.Close()

		app := gofr.New()
		helloClient, err := NewHelloGoFrClient(configs.GRPCHost, app.Metrics())
		require.NoError(t, err, "GoFr hello client creation should not fail")
		
		// Test HelloGoFrClient interface compliance
		var _ HelloGoFrClient = helloClient
		
		// Test that wrapper has the correct GoFr type
		wrapper, ok := helloClient.(*HelloClientWrapper)
		assert.True(t, ok, "Should be able to cast to GoFr HelloClientWrapper")
		assert.NotNil(t, wrapper.client, "Underlying hello client should not be nil")
	})
}

func TestGoFrHelloClientWrapper_Methods(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's wrapper methods without actual gRPC calls
	app := gofr.New()
	helloClient, err := NewHelloGoFrClient(configs.GRPCHost, app.Metrics())
	require.NoError(t, err, "GoFr hello client creation should not fail")
	ctx := createTestContext()

	t.Run("SayHelloMethodExists", func(t *testing.T) {
		// Test that GoFr's SayHello method exists and accepts correct parameters
		req := &HelloRequest{
			Name: "test-name",
		}
		
		// This will fail due to connection, but we're testing GoFr's method signature
		_, err := helloClient.SayHello(ctx, req)
		assert.Error(t, err, "Should fail with invalid connection, but method should exist")
	})

	t.Run("HealthClientEmbedded", func(t *testing.T) {
		// Test that GoFr's HelloGoFrClient embeds HealthClient
		// The HelloGoFrClient interface should include HealthClient methods
		var _ HealthClient = helloClient
	})
}

func TestGoFrHelloClientWrapper_ContextIntegration(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's context integration
	app := gofr.New()
	helloClient, err := NewHelloGoFrClient(configs.GRPCHost, app.Metrics())
	require.NoError(t, err, "GoFr hello client creation should not fail")

	t.Run("ContextParameter", func(t *testing.T) {
		// Test that GoFr's methods accept *gofr.Context
		ctx := createTestContext()
		req := &HelloRequest{
			Name: "test-name",
		}
		
		// Test that the method signature is correct for GoFr context
		_, err := helloClient.SayHello(ctx, req)
		assert.Error(t, err, "Should fail with invalid connection")
		
		// Test that context is properly passed (even though call fails)
		assert.NotNil(t, ctx, "GoFr context should not be nil")
	})

	t.Run("ContextTypeCompliance", func(t *testing.T) {
		// Test that GoFr's methods expect *gofr.Context specifically
		ctx := createTestContext()
		req := &HelloRequest{
			Name: "test-name",
		}
		
		// Verify the method signature expects *gofr.Context
		var _ func(*gofr.Context, *HelloRequest, ...grpc.CallOption) (*HelloResponse, error) = helloClient.SayHello
		
		// Ensure the call compiles (even if it fails at runtime)
		_, _ = helloClient.SayHello(ctx, req)
	})
}

func TestGoFrHelloClientWrapper_MultipleInstances(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's client creation with multiple instances
	t.Run("MultipleHelloClients", func(t *testing.T) {
		app := gofr.New()
		
		client1, err := NewHelloGoFrClient(configs.GRPCHost, app.Metrics())
		require.NoError(t, err, "First GoFr hello client creation should not fail")
		
		client2, err := NewHelloGoFrClient(configs.GRPCHost, app.Metrics())
		require.NoError(t, err, "Second GoFr hello client creation should not fail")

		assert.NotNil(t, client1, "First GoFr hello client should not be nil")
		assert.NotNil(t, client2, "Second GoFr hello client should not be nil")
		assert.NotEqual(t, client1, client2, "GoFr hello client instances should be different")
	})
}

func TestGoFrHelloClientWrapper_ErrorHandling(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Test GoFr's error handling patterns
	t.Run("InvalidAddressHandling", func(t *testing.T) {
		// Set HTTP port to avoid port conflicts
		os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
		
		// Test GoFr's handling of invalid addresses
		app := gofr.New()
		helloClient, err := NewHelloGoFrClient("invalid:address", app.Metrics())
		require.NoError(t, err, "Client creation should not fail immediately")
		
		ctx := createTestContext()
		req := &HelloRequest{
			Name: "test-name",
		}

		// Test GoFr's error handling
		_, err = helloClient.SayHello(ctx, req)
		assert.Error(t, err, "GoFr should handle invalid address errors")
	})

	t.Run("EmptyAddressHandling", func(t *testing.T) {
		// Set HTTP port to avoid port conflicts
		os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
		
		// Test GoFr's handling of empty addresses
		app := gofr.New()
		helloClient, err := NewHelloGoFrClient("", app.Metrics())
		require.NoError(t, err, "Client creation should not fail immediately")
		
		ctx := createTestContext()
		req := &HelloRequest{
			Name: "test-name",
		}

		// Test GoFr's error handling
		_, err = helloClient.SayHello(ctx, req)
		assert.Error(t, err, "GoFr should handle empty address errors")
	})
}

func TestGoFrHelloClientWrapper_ConcurrentAccess(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's concurrent access patterns
	t.Run("ConcurrentSayHelloCalls", func(t *testing.T) {
		app := gofr.New()
		helloClient, err := NewHelloGoFrClient(configs.GRPCHost, app.Metrics())
		require.NoError(t, err, "GoFr hello client creation should not fail")

		numGoroutines := 5
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				ctx := createTestContext()
				req := &HelloRequest{
					Name: "concurrent-test",
				}
				
				// This will fail due to connection, but we're testing GoFr's concurrency
				_, err := helloClient.SayHello(ctx, req)
				assert.Error(t, err, "Should fail with invalid connection")
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})
}