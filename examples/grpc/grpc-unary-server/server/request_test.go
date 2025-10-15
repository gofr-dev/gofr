package server

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestHelloRequestWrapper_Context(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))

	t.Run("Context", func(t *testing.T) {
		// Create request wrapper
		ctx := context.Background()
		req := &HelloRequestWrapper{
			ctx:          ctx,
			HelloRequest: &HelloRequest{Name: "test"},
		}

		// Test Context method
		returnedCtx := req.Context()
		assert.Equal(t, ctx, returnedCtx, "Context should match")
	})
}

func TestHelloRequestWrapper_Param(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))

	t.Run("Param", func(t *testing.T) {
		// Create request wrapper
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: "test"},
		}

		// Test Param method (should return empty string)
		param := req.Param("name")
		assert.Equal(t, "", param, "Param should return empty string")
	})
}

func TestHelloRequestWrapper_PathParam(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))

	t.Run("PathParam", func(t *testing.T) {
		// Create request wrapper
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: "test"},
		}

		// Test PathParam method (should return empty string)
		param := req.PathParam("name")
		assert.Equal(t, "", param, "PathParam should return empty string")
	})
}

func TestHelloRequestWrapper_HostName(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))

	t.Run("HostName", func(t *testing.T) {
		// Create request wrapper
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: "test"},
		}

		// Test HostName method (should return empty string)
		hostname := req.HostName()
		assert.Equal(t, "", hostname, "HostName should return empty string")
	})
}

func TestHelloRequestWrapper_Params(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))

	t.Run("Params", func(t *testing.T) {
		// Create request wrapper
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: "test"},
		}

		// Test Params method (should return nil)
		params := req.Params("name")
		assert.Nil(t, params, "Params should return nil")
	})
}

func TestHelloRequestWrapper_Bind(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Set HTTP port to avoid port conflicts
	os.Setenv("HTTP_PORT", fmt.Sprintf("%d", configs.HTTPPort))

	t.Run("BindSuccess", func(t *testing.T) {
		// Create request wrapper
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: "test"},
		}

		// Test Bind method with valid pointer
		var target HelloRequest
		err := req.Bind(&target)

		require.NoError(t, err, "Bind should not fail")
		assert.Equal(t, "test", target.Name, "Name should be bound correctly")
	})

	t.Run("BindWithNonPointer", func(t *testing.T) {
		// Create request wrapper
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: "test"},
		}

		// Test Bind method with non-pointer (should fail)
		var target HelloRequest
		err := req.Bind(target)

		assert.Error(t, err, "Bind should fail with non-pointer")
		assert.Contains(t, err.Error(), "expected a pointer", "Error message should indicate pointer expected")
	})

	t.Run("BindWithNilPointer", func(t *testing.T) {
		// Create request wrapper
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: "test"},
		}

		// Test Bind method with nil pointer (should fail)
		err := req.Bind(nil)

		assert.Error(t, err, "Bind should fail with nil pointer")
		assert.Contains(t, err.Error(), "expected a pointer", "Error message should indicate pointer expected")
	})

	t.Run("BindWithEmptyRequest", func(t *testing.T) {
		// Create request wrapper with empty request
		req := &HelloRequestWrapper{
			HelloRequest: &HelloRequest{Name: ""},
		}

		// Test Bind method with empty request
		var target HelloRequest
		err := req.Bind(&target)

		require.NoError(t, err, "Bind should not fail with empty request")
		assert.Equal(t, "", target.Name, "Name should be empty")
	})

}
