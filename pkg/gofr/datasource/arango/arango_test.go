package arango

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

func setupDB(t *testing.T) (*Client, *MockArango, *MockLogger, *MockMetrics) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := Config{Host: "localhost", Port: 8527, User: "root", Password: "root"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	mockArango := NewMockArango(ctrl)
	client.client = mockArango

	return client, mockArango, mockLogger, mockMetrics
}

func Test_NewArangoClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	logger.EXPECT().Debugf(gomock.Any(), gomock.Any())
	logger.EXPECT().Logf(gomock.Any(), gomock.Any())

	metrics.EXPECT().NewHistogram("app_arango_stats",
		"Response time of ArangoDB operations in milliseconds.", gomock.Any())

	client := New(Config{Host: "localhost", Port: 8529, Password: "root", User: "admin"})

	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.Connect()

	assert.NotNil(t, client)
}

func Test_Arango_CreateUser(t *testing.T) {
	client, mockArango, mockLogger, mockMetrics := setupDB(t)

	mockArango.EXPECT().CreateUser(context.Background(), "test", gomock.Any()).Return(nil, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(context.Background(), "app_arango_stats",
		gomock.Any(), "endpoint", gomock.Any(), gomock.Any(), gomock.Any())

	_, err := client.CreateUser(context.Background(), "test", nil)
	require.NoError(t, err, "Test_Arango_CreateUser: failed to create user")
}

// Test DropUser
func Test_Arango_DropUser(t *testing.T) {
	client, mockArango, mockLogger, mockMetrics := setupDB(t)

	mockArango.EXPECT().DropUser(context.Background(), "test").Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(context.Background(), "app_arango_stats",
		gomock.Any(), "endpoint", gomock.Any(), gomock.Any(), gomock.Any())

	err := client.DropUser(context.Background(), "test")
	require.NoError(t, err, "Test_Arango_DropUser: failed to drop user")
}
