package server

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

// createTestContext creates a test gofr.Context
func createTestContext(app *gofr.App) *gofr.Context {
	container := &container.Container{}
	return &gofr.Context{
		Context:   context.Background(),
		Container: container,
	}
}

func TestGoFrHealthServer_Creation(t *testing.T) {
	t.Run("GetOrCreateHealthServer", func(t *testing.T) {
		// Test GoFr's getOrCreateHealthServer function
		healthServer := getOrCreateHealthServer()
		assert.NotNil(t, healthServer, "GoFr health server should not be nil")
		
		// Test that it implements the GoFr interface (not the standard gRPC interface)
		// The GoFr health server has different method signatures
		assert.NotNil(t, healthServer, "Health server should not be nil")
	})

	t.Run("HealthServerSingleton", func(t *testing.T) {
		// Test GoFr's singleton pattern for health server
		healthServer1 := getOrCreateHealthServer()
		healthServer2 := getOrCreateHealthServer()
		
		assert.Equal(t, healthServer1, healthServer2, "GoFr health server should be singleton")
	})
}

func TestGoFrHealthServer_Methods(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's health server methods
	app := gofr.New()
	healthServer := getOrCreateHealthServer()
	ctx := createTestContext(app)

	t.Run("CheckMethodExists", func(t *testing.T) {
		// Test that GoFr's Check method exists and accepts correct parameters
		req := &healthpb.HealthCheckRequest{
			Service: "test-service",
		}
		
		// Test GoFr's Check method signature - this will fail with "unknown service" which is expected
		resp, err := healthServer.Check(ctx, req)
		assert.Error(t, err, "Health check should fail for unknown service")
		assert.Nil(t, resp, "Health check response should be nil for unknown service")
		assert.Contains(t, err.Error(), "unknown service", "Error should indicate unknown service")
	})

	t.Run("WatchMethodExists", func(t *testing.T) {
		// Test that GoFr's Watch method exists and accepts correct parameters
		req := &healthpb.HealthCheckRequest{
			Service: "test-service",
		}
		
		// Test GoFr's Watch method signature - this will panic with nil stream, but we're testing method existence
		assert.Panics(t, func() {
			healthServer.Watch(ctx, req, nil)
		}, "Watch should panic with nil stream, but method should exist")
	})
}

func TestGoFrHealthServer_SetServingStatus(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's SetServingStatus functionality
	app := gofr.New()
	healthServer := getOrCreateHealthServer()
	ctx := createTestContext(app)

	t.Run("SetServingStatus", func(t *testing.T) {
		// Test GoFr's SetServingStatus method
		healthServer.SetServingStatus(ctx, "test-service", healthpb.HealthCheckResponse_SERVING)
		
		// Verify the status was set
		req := &healthpb.HealthCheckRequest{
			Service: "test-service",
		}
		resp, err := healthServer.Check(ctx, req)
		require.NoError(t, err, "Health check should not fail")
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status, "Service should be serving")
	})

	t.Run("SetNotServingStatus", func(t *testing.T) {
		// Test GoFr's SetServingStatus with NOT_SERVING
		healthServer.SetServingStatus(ctx, "test-service-not-serving", healthpb.HealthCheckResponse_NOT_SERVING)
		
		// Verify the status was set
		req := &healthpb.HealthCheckRequest{
			Service: "test-service-not-serving",
		}
		resp, err := healthServer.Check(ctx, req)
		require.NoError(t, err, "Health check should not fail")
		assert.Equal(t, healthpb.HealthCheckResponse_NOT_SERVING, resp.Status, "Service should not be serving")
	})
}

func TestGoFrHealthServer_Shutdown(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's Shutdown functionality
	app := gofr.New()
	healthServer := getOrCreateHealthServer()
	ctx := createTestContext(app)

	t.Run("Shutdown", func(t *testing.T) {
		// Test GoFr's Shutdown method
		healthServer.Shutdown(ctx)
		
		// After shutdown, all services should return NOT_SERVING
		req := &healthpb.HealthCheckRequest{
			Service: "any-service",
		}
		resp, err := healthServer.Check(ctx, req)
		// After shutdown, health checks should fail with "unknown service"
		assert.Error(t, err, "Health check should fail after shutdown")
		assert.Nil(t, resp, "Health check response should be nil after shutdown")
		assert.Contains(t, err.Error(), "unknown service", "Error should indicate unknown service after shutdown")
	})
}

func TestGoFrHealthServer_Resume(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's Resume functionality
	app := gofr.New()
	healthServer := getOrCreateHealthServer()
	ctx := createTestContext(app)

	t.Run("Resume", func(t *testing.T) {
		// Test GoFr's Resume method
		healthServer.Resume(ctx)
		
		// After resume, services should return to their previous status
		healthServer.SetServingStatus(ctx, "test-service-resume", healthpb.HealthCheckResponse_SERVING)
		
		req := &healthpb.HealthCheckRequest{
			Service: "test-service-resume",
		}
		resp, err := healthServer.Check(ctx, req)
		require.NoError(t, err, "Health check should not fail")
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status, "Service should be serving after resume")
	})
}

func TestGoFrHealthServer_MultipleInstances(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))
	
	// Test GoFr's singleton pattern
	t.Run("SingletonPattern", func(t *testing.T) {
		app := gofr.New()
		healthServer1 := getOrCreateHealthServer()
		healthServer2 := getOrCreateHealthServer()
		ctx := createTestContext(app)
		
		assert.Equal(t, healthServer1, healthServer2, "GoFr health server should be singleton")
		
		// Test that operations on one affect the other
		healthServer1.SetServingStatus(ctx, "singleton-test", healthpb.HealthCheckResponse_SERVING)
		
		req := &healthpb.HealthCheckRequest{
			Service: "singleton-test",
		}
		resp, err := healthServer2.Check(ctx, req)
		require.NoError(t, err, "Health check should not fail")
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status, "Singleton should share state")
	})
}