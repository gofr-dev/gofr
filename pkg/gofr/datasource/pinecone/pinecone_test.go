package pinecone

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Connect(t *testing.T) {
	client := New(&Config{
		APIKey: "pcsk_2CevjK_CgWuBU92rTb2BWSKky5eazZVy7t1J3BEDL4KuG3USJBRNGgxWeYhUGijjKGLzWF",
	})

	// Test that Connect doesn't panic and sets up the client properly
	client.Connect()

	assert.True(t, client.connected)
	assert.NotNil(t, client.client)
	assert.Equal(t, "pcsk_2CevjK_CgWuBU92rTb2BWSKky5eazZVy7t1J3BEDL4KuG3USJBRNGgxWeYhUGijjKGLzWF", client.config.APIKey)
}

func TestClient_Connect_InvalidAPIKey(t *testing.T) {
	client := New(&Config{
		APIKey: "",
	})

	// Test that Connect handles empty API key gracefully
	client.Connect()

	assert.False(t, client.connected)
	assert.Nil(t, client.client)
}

func TestClient_IsConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "pcsk_2CevjK_CgWuBU92rTb2BWSKky5eazZVy7t1J3BEDL4KuG3USJBRNGgxWeYhUGijjKGLzWF",
	})

	// Initially not connected
	assert.False(t, client.IsConnected())

	// After Connect() should be connected (even with invalid key)
	client.Connect()
	assert.True(t, client.IsConnected())
}

func TestClient_HealthCheck_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect() to test unconnected state
	health, err := client.HealthCheck(context.Background())

	assert.Error(t, err)
	assert.NotNil(t, health)
	assert.Contains(t, err.Error(), "not connected")

	// Check health struct
	h, ok := health.(Health)
	require.True(t, ok)
	assert.Equal(t, "DOWN", h.Status)
	assert.Equal(t, "pinecone client not connected", h.Details["error"])
	assert.Equal(t, "disconnected", h.Details["connection_state"])
	assert.True(t, h.Details["api_key_configured"].(bool))
}

func TestClient_HealthCheck_Connected_InvalidKey(t *testing.T) {
	client := New(&Config{
		APIKey: "invalid-api-key",
	})

	client.Connect()

	// This should fail with invalid credentials, but test that it returns proper structure
	health, err := client.HealthCheck(context.Background())

	// We expect an error since we don't have valid credentials
	assert.Error(t, err)
	assert.NotNil(t, health)

	// Check health struct
	h, ok := health.(Health)
	require.True(t, ok)
	assert.Equal(t, "DOWN", h.Status)
	assert.Equal(t, "error", h.Details["connection_state"])
	assert.Contains(t, h.Details, "error")
	assert.True(t, h.Details["api_key_configured"].(bool))
}

// TestClient_HealthCheck_RealConnection tests with a potentially valid API key
// This test will skip if no valid API key is available
func TestClient_HealthCheck_RealConnection(t *testing.T) {
	// Use the API key from your test - this should work if it's valid
	client := New(&Config{
		APIKey: "pcsk_2CevjK_CgWuBU92rTb2BWSKky5eazZVy7t1J3BEDL4KuG3USJBRNGgxWeYhUGijjKGLzWF",
	})

	client.Connect()

	// Test real-time connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := client.HealthCheck(ctx)

	// This test expects either success or a specific API error
	if err != nil {
		// If there's an error, it should be a proper health response
		h, ok := health.(Health)
		require.True(t, ok)
		assert.Equal(t, "DOWN", h.Status)
		assert.Contains(t, h.Details, "error")

		// Log the error for debugging
		t.Logf("Health check failed (expected if API key is invalid): %v", err)
	} else {
		// If successful, verify the structure
		h, ok := health.(Health)
		require.True(t, ok)
		assert.Equal(t, "UP", h.Status)
		assert.Equal(t, "connected", h.Details["connection_state"])
		assert.Contains(t, h.Details, "index_count")

		t.Logf("Health check successful. Index count: %v", h.Details["index_count"])
	}
}

func TestClient_Ping(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Test ping without connection
	err := client.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// Test ping with connection (will fail due to invalid key, but tests method)
	client.Connect()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Ping(ctx)
	// We expect an error due to invalid API key, but it should be an API error, not a connection error
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "not connected")
}

// TestClient_Ping_RealConnection tests ping with potentially valid API key
func TestClient_Ping_RealConnection(t *testing.T) {
	client := New(&Config{
		APIKey: "pcsk_2CevjK_CgWuBU92rTb2BWSKky5eazZVy7t1J3BEDL4KuG3USJBRNGgxWeYhUGijjKGLzWF",
	})

	client.Connect()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Ping(ctx)

	if err != nil {
		t.Logf("Ping failed (expected if API key is invalid): %v", err)
		// Should not be a connection error
		assert.NotContains(t, err.Error(), "not connected")
	} else {
		t.Log("Ping successful - real connection established")
	}
}

// TestClient_ConnectionTimeout tests connection timeout behavior
func TestClient_ConnectionTimeout(t *testing.T) {
	client := New(&Config{
		APIKey: "invalid-api-key",
	})

	client.Connect()

	// Test with very short timeout to test timeout behavior
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.ListIndexes(ctx)
	assert.Error(t, err)
	// Should be a timeout or API error, not a connection error
	assert.NotContains(t, err.Error(), "not connected")
}

func TestClient_Upsert(t *testing.T) {
	client := New(&Config{
		APIKey: "invalid-api-key",
	})

	client.Connect()

	vectors := []any{
		Vector{
			ID:     "vec1",
			Values: []float32{0.1, 0.2, 0.3},
			Metadata: map[string]any{
				"category": "test",
			},
		},
	}

	// This will fail due to invalid credentials, but tests the method signature
	count, err := client.Upsert(context.Background(), "test-index", "test-namespace", vectors)

	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestClient_Upsert_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	vectors := []any{
		Vector{
			ID:     "vec1",
			Values: []float32{0.1, 0.2, 0.3},
		},
	}

	count, err := client.Upsert(context.Background(), "test-index", "test-namespace", vectors)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
	assert.Equal(t, 0, count)
}

func TestClient_Query(t *testing.T) {
	client := New(&Config{
		APIKey: "invalid-api-key",
	})

	client.Connect()

	vector := []float32{0.1, 0.2, 0.3}

	results, err := client.Query(context.Background(), "test-index", "test-namespace", vector, 10, true, true, nil)

	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestClient_Query_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	vector := []float32{0.1, 0.2, 0.3}

	results, err := client.Query(context.Background(), "test-index", "test-namespace", vector, 10, true, true, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
	assert.Nil(t, results)
}

func TestClient_Fetch(t *testing.T) {
	client := New(&Config{
		APIKey: "invalid-api-key",
	})

	client.Connect()

	ids := []string{"vec1", "vec2"}

	vectors, err := client.Fetch(context.Background(), "test-index", "test-namespace", ids)

	assert.Error(t, err)
	assert.Nil(t, vectors)
}

func TestClient_Fetch_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	ids := []string{"vec1", "vec2"}

	vectors, err := client.Fetch(context.Background(), "test-index", "test-namespace", ids)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
	assert.Nil(t, vectors)
}

func TestClient_Delete(t *testing.T) {
	client := New(&Config{
		APIKey: "invalid-api-key",
	})

	client.Connect()

	ids := []string{"vec1", "vec2"}

	err := client.Delete(context.Background(), "test-index", "test-namespace", ids)

	assert.Error(t, err)
}

func TestClient_Delete_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	ids := []string{"vec1", "vec2"}

	err := client.Delete(context.Background(), "test-index", "test-namespace", ids)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_DeleteAll(t *testing.T) {
	client := New(&Config{
		APIKey: "invalid-api-key",
	})

	client.Connect()

	err := client.DeleteAll(context.Background(), "test-index", "test-namespace")

	assert.Error(t, err)
}

func TestClient_DeleteAll_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	err := client.DeleteAll(context.Background(), "test-index", "test-namespace")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_ListIndexes_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	indexes, err := client.ListIndexes(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
	assert.Nil(t, indexes)
}

func TestClient_DescribeIndex_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	result, err := client.DescribeIndex(context.Background(), "test-index")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
	assert.Nil(t, result)
}

func TestClient_CreateIndex_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	err := client.CreateIndex(context.Background(), "test-index", 1536, "cosine", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_CreateIndex_UnsupportedMetric(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	client.Connect()

	err := client.CreateIndex(context.Background(), "test-index", 1536, "unsupported", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported metric")
}

func TestClient_DeleteIndex_NotConnected(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Don't call Connect()
	err := client.DeleteIndex(context.Background(), "test-index")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_UseLogger(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Test with nil logger (should not panic)
	client.UseLogger(nil)
	assert.Nil(t, client.logger)

	// Test with invalid type (should not panic)
	client.UseLogger("not a logger")
	assert.Nil(t, client.logger)
}

func TestClient_UseMetrics(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Test with nil metrics (should not panic)
	client.UseMetrics(nil)
	assert.Nil(t, client.metrics)

	// Test with invalid type (should not panic)
	client.UseMetrics("not metrics")
	assert.Nil(t, client.metrics)
}

func TestClient_UseTracer(t *testing.T) {
	client := New(&Config{
		APIKey: "test-api-key",
	})

	// Test with nil tracer (should not panic)
	client.UseTracer(nil)
	assert.Nil(t, client.tracer)

	// Test with invalid type (should not panic)
	client.UseTracer("not a tracer")
	assert.Nil(t, client.tracer)
}
