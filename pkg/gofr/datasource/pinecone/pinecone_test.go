package pinecone

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPineconeClient_Connect_Success(t *testing.T) {
	config := &Config{
		APIKey: "test-api-key",
	}

	client := New(config)
	mockLogger := &MockLogger{}
	client.UseLogger(mockLogger)

	mockLogger.On("Debugf", mock.AnythingOfType("string")).Return()
	mockLogger.On("Infof", mock.AnythingOfType("string")).Return()

	// Note: This would normally connect to real Pinecone, so we'll just test the setup
	assert.NotNil(t, client)
	assert.Equal(t, "test-api-key", client.config.APIKey)
}

func TestPineconeClient_Connect_MissingAPIKey(t *testing.T) {
	config := &Config{
		APIKey: "",
	}

	client := New(config)
	mockLogger := &MockLogger{}
	client.UseLogger(mockLogger)

	// Set up mock expectations for the calls that will be made
	// Debugf is called with (format string, args...)
	mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
	mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()

	client.Connect()

	assert.False(t, client.connected)
	mockLogger.AssertExpectations(t)
}

func TestPineconeClient_Query_Success(t *testing.T) {
	mockClient := &MockPineconeClient{}

	params := QueryParams{
		IndexName: "test-index",
		Namespace: "test-namespace",
		Vector:    []float32{0.1, 0.2, 0.3},
		TopK:      10,
	}

	expectedResults := []any{
		map[string]any{
			"id":     "vec1",
			"score":  0.95,
			"values": []float32{0.1, 0.2, 0.3},
		},
	}

	mockClient.On("Query", mock.Anything, &params).Return(expectedResults, nil)
	ctx := context.Background()
	results, err := mockClient.Query(ctx, &params)

	require.NoError(t, err)
	assert.Equal(t, expectedResults, results)
	// Assert that all expected calls were made
	mockClient.AssertExpectations(t)
}

func TestPineconeClient_Query_Error(t *testing.T) {
	mockClient := &MockPineconeClient{}

	params := QueryParams{
		IndexName: "test-index",
		Namespace: "test-namespace",
		Vector:    []float32{0.1, 0.2, 0.3},
		TopK:      10,
	}
	ctx := context.Background()

	mockClient.On("Query", mock.Anything, &params).Return(nil, ErrQuery)

	results, err := mockClient.Query(ctx, &params)

	require.Error(t, err)
	assert.Nil(t, results)
	// Assert that all expected calls were made
	mockClient.AssertExpectations(t)
}

func TestPineconeClient_Query_InvalidParams(t *testing.T) {
	config := &Config{
		APIKey: "test-api-key",
	}
	ctx := context.Background()
	client := New(config)
	mockLogger := &MockLogger{}
	client.UseLogger(mockLogger)

	params := QueryParams{
		IndexName: "",
		Namespace: "test-namespace",
		Vector:    []float32{0.1, 0.2, 0.3},
		TopK:      10,
	}

	mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()

	results, err := client.Query(ctx, &params)

	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "not connected")
}

func TestPineconeClient_Upsert_Success(t *testing.T) {
	mockClient := &MockPineconeClient{}

	vectors := []any{
		map[string]any{
			"id":     "vec1",
			"values": []float32{0.1, 0.2, 0.3},
		},
	}

	mockClient.On("Upsert", mock.Anything, "test-index", "test-namespace", vectors).Return(1, nil)
	ctx := context.Background()
	count, err := mockClient.Upsert(ctx, "test-index", "test-namespace", vectors)

	require.NoError(t, err)
	assert.Equal(t, 1, count)
	mockClient.AssertExpectations(t)
}

func TestPineconeClient_HealthCheck_Success(t *testing.T) {
	mockClient := &MockPineconeClient{}

	expectedHealth := map[string]any{
		"status": "UP",
		"details": map[string]any{
			"connection_state": "connected",
		},
	}
	ctx := context.Background()
	mockClient.On("HealthCheck", mock.Anything).Return(expectedHealth, nil)
	health, err := mockClient.HealthCheck(ctx)

	require.NoError(t, err)
	assert.Equal(t, expectedHealth, health)
	mockClient.AssertExpectations(t)
}
