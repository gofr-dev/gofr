package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"
)

var errDynamoFailure = errors.New("dynamodb error")

type testDeps struct {
	ctx         context.Context
	client      *Client
	mockDB      *MockdynamoDBInterface
	mockLogger  *MockLogger
	mockMetrics *MockMetrics
	finish      func()
}

func setupTest(t *testing.T) testDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockDB := NewMockdynamoDBInterface(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := &Client{
		db:        mockDB,
		configs:   &Configs{Table: "test-table", Region: "us-east-1", PartitionKeyName: "pk"},
		logger:    mockLogger,
		metrics:   mockMetrics,
		connected: true, // Set as connected for tests
	}

	return testDeps{
		ctx:         t.Context(),
		client:      client,
		mockDB:      mockDB,
		mockLogger:  mockLogger,
		mockMetrics: mockMetrics,
		finish:      ctrl.Finish,
	}
}

func Test_ClientSet(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	key := "test-key"
	value := "test-value"

	expectedInput := &dynamodb.PutItemInput{
		TableName: aws.String("test-table"),
		Item: map[string]types.AttributeValue{
			"pk":    &types.AttributeValueMemberS{Value: key},
			"value": &types.AttributeValueMemberS{Value: value},
		},
	}

	mockDB.EXPECT().PutItem(ctx, expectedInput, gomock.Any()).Return(&dynamodb.PutItemOutput{}, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", client.configs.Table,
		"operation", "SET",
	)

	require.NoError(t, client.Set(ctx, key, value))
}

func Test_ClientSetError(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	key := "test-key"
	value := "test-value"
	expectedErr := errDynamoFailure

	mockDB.EXPECT().PutItem(ctx, gomock.Any(), gomock.Any()).Return(nil, expectedErr)
	mockLogger.EXPECT().Errorf("error while setting data for key: %v, error: %v", key, expectedErr)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", "test-table",
		"operation", "SET",
	)

	err := client.Set(ctx, key, value)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func Test_ClientGet(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	key := "test-key"
	expectedValue := "test-value"

	expectedInput := &dynamodb.GetItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	expectedOutput := &dynamodb.GetItemOutput{
		Item: map[string]types.AttributeValue{
			"pk":    &types.AttributeValueMemberS{Value: key},
			"value": &types.AttributeValueMemberS{Value: expectedValue},
		},
	}

	mockDB.EXPECT().GetItem(ctx, expectedInput, gomock.Any()).Return(expectedOutput, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", "test-table",
		"operation", "GET",
	)

	result, err := client.Get(ctx, key)

	require.NoError(t, err)
	assert.Equal(t, expectedValue, result)
}

func Test_ClientGetError(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	key := "test-key"
	expectedErr := errDynamoFailure

	expectedInput := &dynamodb.GetItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	mockDB.EXPECT().GetItem(ctx, expectedInput, gomock.Any()).Return(nil, expectedErr)
	mockLogger.EXPECT().Errorf("error while fetching data for key: %v, error: %v", key, expectedErr)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", "test-table",
		"operation", "GET",
	)

	value, err := client.Get(ctx, key)

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Empty(t, value)
}

func Test_ClientDelete(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	key := "test-key"
	expectedInput := &dynamodb.DeleteItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	mockDB.EXPECT().DeleteItem(ctx, expectedInput, gomock.Any()).Return(&dynamodb.DeleteItemOutput{}, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", client.configs.Table,
		"operation", "DELETE",
	)

	require.NoError(t, client.Delete(ctx, key))
}

func Test_ClientDeleteError(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	key := "test-key"
	expectedErr := errDynamoFailure

	expectedInput := &dynamodb.DeleteItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	mockDB.EXPECT().DeleteItem(ctx, expectedInput, gomock.Any()).Return(nil, expectedErr)
	mockLogger.EXPECT().Errorf("error while deleting data for key: %v, error: %v", key, expectedErr)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", "test-table",
		"operation", "DELETE",
	)

	err := client.Delete(ctx, key)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func Test_ClientHealthCheckSuccess(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	finish := deps.finish

	defer finish()

	expectedInput := &dynamodb.DescribeTableInput{
		TableName: aws.String("test-table"),
	}

	mockDB.EXPECT().DescribeTable(t.Context(), expectedInput, gomock.Any()).Return(&dynamodb.DescribeTableOutput{}, nil)

	res, err := client.HealthCheck(ctx)

	require.NoError(t, err)

	h, ok := res.(*Health)

	require.True(t, ok)
	assert.Equal(t, "UP", h.Status)
	assert.Equal(t, map[string]any{
		"table":  "test-table",
		"region": "us-east-1",
	}, h.Details)
}

func Test_ClientHealthCheckFailure(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	finish := deps.finish

	defer finish()

	expectedErr := errDynamoFailure
	expectedInput := &dynamodb.DescribeTableInput{
		TableName: aws.String("test-table"),
	}

	mockDB.EXPECT().DescribeTable(t.Context(), expectedInput, gomock.Any()).Return(nil, expectedErr)

	res, err := client.HealthCheck(ctx)

	require.Error(t, err)
	assert.Equal(t, errStatusDown, err)

	h, ok := res.(*Health)

	require.True(t, ok)
	assert.Equal(t, "DOWN", h.Status)
	assert.Equal(t, map[string]any{
		"table":  "test-table",
		"region": "us-east-1",
	}, h.Details)
}

func Test_ToJSON(t *testing.T) {
	testData := map[string]any{
		"name":  "John Doe",
		"age":   30,
		"email": "john@example.com",
	}

	jsonStr, err := ToJSON(testData)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonStr)
	assert.Contains(t, jsonStr, "John Doe")
	assert.Contains(t, jsonStr, "john@example.com")
}

func Test_ToJSONError(t *testing.T) {
	// Create a value that cannot be marshaled to JSON
	invalidData := make(chan int)

	jsonStr, err := ToJSON(invalidData)
	require.Error(t, err)
	assert.Empty(t, jsonStr)
	assert.Contains(t, err.Error(), "failed to marshal value to JSON")
}

func Test_FromJSON(t *testing.T) {
	jsonStr := `{"name":"John Doe","age":30,"email":"john@example.com"}`
	var result map[string]any

	err := FromJSON(jsonStr, &result)
	require.NoError(t, err)
	assert.Equal(t, "John Doe", result["name"])
	assert.Equal(t, float64(30), result["age"])
	assert.Equal(t, "john@example.com", result["email"])
}

func Test_FromJSONError(t *testing.T) {
	invalidJSON := `{"name":"John Doe","age":30,"email":}`
	var result map[string]any

	err := FromJSON(invalidJSON, &result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal JSON")
}

func Test_ClientNotConnected(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	finish := deps.finish

	defer finish()

	// Set connected to false
	client.connected = false

	// Test Set when not connected
	err := client.Set(ctx, "key", "value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client not connected")

	// Test Get when not connected
	_, err = client.Get(ctx, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client not connected")

	// Test Delete when not connected
	err = client.Delete(ctx, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client not connected")

	// Test HealthCheck when not connected
	health, err := client.HealthCheck(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status down")
	
	h, ok := health.(*Health)
	require.True(t, ok)
	assert.Equal(t, "DOWN", h.Status)
	assert.Contains(t, h.Details["error"], "client not connected")
}

func Test_ClientGetItemNotFound(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	key := "non-existent-key"

	expectedInput := &dynamodb.GetItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	// Return empty item (key not found)
	expectedOutput := &dynamodb.GetItemOutput{
		Item: map[string]types.AttributeValue{},
	}

	mockDB.EXPECT().GetItem(ctx, expectedInput, gomock.Any()).Return(expectedOutput, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", "test-table",
		"operation", "GET",
	)

	result, err := client.Get(ctx, key)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
	assert.Empty(t, result)
}

func Test_ClientConnect(t *testing.T) {
	deps := setupTest(t)
	client := deps.client
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	// Set up expectations for Connect
	mockLogger.EXPECT().Debugf("connecting to DynamoDB table %v in region %v", "test-table", "us-east-1")
	mockMetrics.EXPECT().NewHistogram("app_dynamodb_duration_ms", "Response time of DynamoDB queries in milliseconds.", gomock.Any())
	mockLogger.EXPECT().Infof("connected to DynamoDB table %v in region %v", "test-table", "us-east-1")

	// Call Connect
	client.Connect()

	// Verify client is connected
	assert.True(t, client.connected)
}

func Test_ClientUseLogger(t *testing.T) {
	deps := setupTest(t)
	client := deps.client
	finish := deps.finish

	defer finish()

	mockLogger := NewMockLogger(gomock.NewController(t))
	client.UseLogger(mockLogger)

	assert.Equal(t, mockLogger, client.logger)
}

func Test_ClientUseMetrics(t *testing.T) {
	deps := setupTest(t)
	client := deps.client
	finish := deps.finish

	defer finish()

	mockMetrics := NewMockMetrics(gomock.NewController(t))
	client.UseMetrics(mockMetrics)

	assert.Equal(t, mockMetrics, client.metrics)
}

func Test_ClientUseTracer(t *testing.T) {
	deps := setupTest(t)
	client := deps.client
	finish := deps.finish

	defer finish()

	mockTracer := trace.NewNoopTracerProvider().Tracer("test")
	client.UseTracer(mockTracer)

	assert.Equal(t, mockTracer, client.tracer)
}

func Test_New(t *testing.T) {
	configs := Configs{
		Table:            "test-table",
		Region:           "us-east-1",
		Endpoint:         "http://localhost:8000",
		PartitionKeyName: "custom-pk",
	}

	client := New(configs)

	assert.Equal(t, "test-table", client.configs.Table)
	assert.Equal(t, "us-east-1", client.configs.Region)
	assert.Equal(t, "http://localhost:8000", client.configs.Endpoint)
	assert.Equal(t, "custom-pk", client.configs.PartitionKeyName)
	assert.False(t, client.connected)
}

func Test_NewWithDefaultPartitionKey(t *testing.T) {
	configs := Configs{
		Table:  "test-table",
		Region: "us-east-1",
	}

	client := New(configs)

	assert.Equal(t, "pk", client.configs.PartitionKeyName) // Should default to "pk"
}

func Test_ClientSetWithTracer(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	// Add tracer
	mockTracer := trace.NewNoopTracerProvider().Tracer("test")
	client.UseTracer(mockTracer)

	key := "test-key"
	value := "test-value"

	expectedInput := &dynamodb.PutItemInput{
		TableName: aws.String("test-table"),
		Item: map[string]types.AttributeValue{
			"pk":    &types.AttributeValueMemberS{Value: key},
			"value": &types.AttributeValueMemberS{Value: value},
		},
	}

	mockDB.EXPECT().PutItem(ctx, expectedInput, gomock.Any()).Return(&dynamodb.PutItemOutput{}, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", client.configs.Table,
		"operation", "SET",
	)

	require.NoError(t, client.Set(ctx, key, value))
}

func Test_ClientGetWithTracer(t *testing.T) {
	deps := setupTest(t)
	ctx := deps.ctx
	client := deps.client
	mockDB := deps.mockDB
	mockLogger := deps.mockLogger
	mockMetrics := deps.mockMetrics
	finish := deps.finish

	defer finish()

	// Add tracer
	mockTracer := trace.NewNoopTracerProvider().Tracer("test")
	client.UseTracer(mockTracer)

	key := "test-key"
	expectedValue := "test-value"

	expectedInput := &dynamodb.GetItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	expectedOutput := &dynamodb.GetItemOutput{
		Item: map[string]types.AttributeValue{
			"pk":    &types.AttributeValueMemberS{Value: key},
			"value": &types.AttributeValueMemberS{Value: expectedValue},
		},
	}

	mockDB.EXPECT().GetItem(ctx, expectedInput, gomock.Any()).Return(expectedOutput, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", "test-table",
		"operation", "GET",
	)

	result, err := client.Get(ctx, key)

	require.NoError(t, err)
	assert.Equal(t, expectedValue, result)
}
