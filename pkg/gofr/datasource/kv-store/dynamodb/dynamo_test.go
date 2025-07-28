package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	ctrl := gomock.NewController(t)
	mockDB := NewMockdynamoDBInterface(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := &Client{
		db:      mockDB,
		configs: &Configs{Table: "test-table", Region: "us-east-1", PartitionKeyName: "pk"},
		logger:  mockLogger,
		metrics: mockMetrics,
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
	attributes := map[string]any{"field1": "value1", "field2": "value2"}

	itemAV, err := attributevalue.MarshalMap(attributes)
	require.NoError(t, err)
	itemAV["pk"] = &types.AttributeValueMemberS{Value: key}

	expectedInput := &dynamodb.PutItemInput{
		TableName: aws.String("test-table"),
		Item:      itemAV,
	}

	mockDB.EXPECT().PutItem(ctx, expectedInput, gomock.Any()).Return(&dynamodb.PutItemOutput{}, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", client.configs.Table,
		"type", "SET",
	)

	require.NoError(t, client.Set(ctx, key, attributes))
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
	attributes := map[string]any{"field1": "value1", "field2": "value2"}
	expectedErr := errDynamoFailure

	mockDB.EXPECT().PutItem(ctx, gomock.Any(), gomock.Any()).Return(nil, expectedErr)
	mockLogger.EXPECT().Errorf("error while setting data for key: %v, error: %v", key, expectedErr)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", "test-table",
		"type", "SET",
	)

	err := client.Set(ctx, key, attributes)
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
	expectedAttributes := map[string]any{"field1": "value1", "field2": "value2"}

	itemAV, err := attributevalue.MarshalMap(expectedAttributes)
	require.NoError(t, err)
	itemAV["pk"] = &types.AttributeValueMemberS{Value: key}

	expectedInput := &dynamodb.GetItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	expectedOutput := &dynamodb.GetItemOutput{
		Item: itemAV,
	}

	mockDB.EXPECT().GetItem(ctx, expectedInput, gomock.Any()).Return(expectedOutput, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_dynamodb_duration_ms",
		gomock.Any(),
		"table", "test-table",
		"type", "GET",
	)

	result, err := client.Get(ctx, key)

	require.NoError(t, err)
	assert.Equal(t, expectedAttributes, result)
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
		"type", "GET",
	)

	value, err := client.Get(ctx, key)

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, value)
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
		"type", "DELETE",
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
		"type", "DELETE",
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

	mockDB.EXPECT().DescribeTable(context.Background(), expectedInput, gomock.Any()).Return(&dynamodb.DescribeTableOutput{}, nil)

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

	expectedErr := errors.New("dynamodb error")
	expectedInput := &dynamodb.DescribeTableInput{
		TableName: aws.String("test-table"),
	}

	mockDB.EXPECT().DescribeTable(context.Background(), expectedInput, gomock.Any()).Return(nil, expectedErr)

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
