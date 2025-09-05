package main

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
)

// TestDynamoDBAsKVStoreOperations demonstrates DynamoDB as a KVStore using Uber mocks
func TestDynamoDBAsKVStoreOperations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock container with KVStore mock (which DynamoDB implements)
	_, mocks := container.NewMockContainer(t)

	// Set up expectations for KVStore operations
	mocks.KVStore.EXPECT().
		Set(gomock.Any(), "test-key", gomock.Any()).
		Return(nil).
		Times(1)

	mocks.KVStore.EXPECT().
		Get(gomock.Any(), "test-key").
		Return(`{"name":"John Doe","email":"john@example.com","created":1234567890}`, nil).
		Times(1)

	mocks.KVStore.EXPECT().
		Delete(gomock.Any(), "test-key").
		Return(nil).
		Times(1)

	mocks.KVStore.EXPECT().
		HealthCheck(gomock.Any()).
		Return(map[string]any{
			"status": "UP",
			"details": map[string]any{
				"table":  "gofr-test-table",
				"region": "us-east-1",
			},
		}, nil).
		Times(1)

	// Test the KVStore operations directly
	ctx := context.Background()

	// Test Set operation
	err := mocks.KVStore.Set(ctx, "test-key", `{"name":"John Doe","email":"john@example.com"}`)
	if err != nil {
		t.Errorf("Set operation failed: %v", err)
	}

	// Test Get operation
	result, err := mocks.KVStore.Get(ctx, "test-key")
	if err != nil {
		t.Errorf("Get operation failed: %v", err)
	}

	if result == "" {
		t.Error("Get operation returned empty result")
	}

	// Test Delete operation
	err = mocks.KVStore.Delete(ctx, "test-key")
	if err != nil {
		t.Errorf("Delete operation failed: %v", err)
	}

	// Test HealthCheck operation
	health, err := mocks.KVStore.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck operation failed: %v", err)
	}

	if health == nil {
		t.Error("HealthCheck operation returned nil result")
	}

	t.Logf("✅ All DynamoDB KVStore operations completed successfully with Uber mocks")
	t.Logf("📊 Get result: %s", result)
	t.Logf("🏥 Health check result: %+v", health)
}

// TestDynamoDBAsKVStoreErrorHandling demonstrates error handling with Uber mocks
func TestDynamoDBAsKVStoreErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock container with KVStore mock
	_, mocks := container.NewMockContainer(t)

	// Set up expectations for error scenarios
	mocks.KVStore.EXPECT().
		Get(gomock.Any(), "non-existent-key").
		Return("", context.DeadlineExceeded).
		Times(1)

	mocks.KVStore.EXPECT().
		HealthCheck(gomock.Any()).
		Return(nil, context.DeadlineExceeded).
		Times(1)

	ctx := context.Background()

	// Test Get operation with error
	_, err := mocks.KVStore.Get(ctx, "non-existent-key")
	if err == nil {
		t.Error("Expected error for non-existent key, got nil")
	}

	// Test HealthCheck operation with error
	_, err = mocks.KVStore.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected error for health check, got nil")
	}

	t.Logf("✅ Error handling tests completed successfully with Uber mocks")
	t.Logf("❌ Get error: %v", err)
}

// TestDynamoDBAsKVStoreJSONHandling demonstrates JSON serialization/deserialization
func TestDynamoDBAsKVStoreJSONHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock container with KVStore mock
	_, mocks := container.NewMockContainer(t)

	// Test data
	testData := map[string]any{
		"name":      "John Doe",
		"email":     "john@example.com",
		"created":   1234567890,
		"timestamp": "2023-01-01T00:00:00Z",
	}

	// Marshal to JSON string
	jsonData, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// Set up expectations
	mocks.KVStore.EXPECT().
		Set(gomock.Any(), "json-key", string(jsonData)).
		Return(nil).
		Times(1)

	mocks.KVStore.EXPECT().
		Get(gomock.Any(), "json-key").
		Return(string(jsonData), nil).
		Times(1)

	ctx := context.Background()

	// Test Set operation with JSON data
	err = mocks.KVStore.Set(ctx, "json-key", string(jsonData))
	if err != nil {
		t.Errorf("Set operation with JSON failed: %v", err)
	}

	// Test Get operation with JSON data
	result, err := mocks.KVStore.Get(ctx, "json-key")
	if err != nil {
		t.Errorf("Get operation with JSON failed: %v", err)
	}

	// Unmarshal back to map
	var retrievedData map[string]any
	err = json.Unmarshal([]byte(result), &retrievedData)
	if err != nil {
		t.Errorf("Failed to unmarshal retrieved data: %v", err)
	}

	// Verify data integrity
	if retrievedData["name"] != testData["name"] {
		t.Errorf("Expected name %v, got %v", testData["name"], retrievedData["name"])
	}

	t.Logf("✅ JSON handling tests completed successfully with Uber mocks")
	t.Logf("📊 Original data: %+v", testData)
	t.Logf("📊 Retrieved data: %+v", retrievedData)
}
