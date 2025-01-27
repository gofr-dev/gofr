package nats

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// setupNATS creates a new NATS client with mocked dependencies for testing.
func setupNATS(t *testing.T) *Client {
	t.Helper()

	cl := New(Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	})

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := NewMockLogger(ctrl)

	mockMetrics.EXPECT().NewHistogram("app_natskv_stats",
		"Response time of NATS KV operations in milliseconds.",
		gomock.Any())

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(),
		"app_natskv_stats",
		gomock.Any(),
		"bucket", cl.configs.Bucket,
		"type", gomock.Any()).AnyTimes()

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	cl.UseLogger(mockLogger)
	cl.UseMetrics(mockMetrics)
	cl.Connect()

	return cl
}

func Test_ClientSet(t *testing.T) {
	cl := setupNATS(t)

	err := cl.Set(context.Background(), "test_key", "test_value")

	require.NoError(t, err)
}

func Test_ClientGet(t *testing.T) {
	cl := setupNATS(t)

	err := cl.Set(context.Background(), "test_key", "test_value")
	require.NoError(t, err)

	val, err := cl.Get(context.Background(), "test_key")

	require.NoError(t, err)
	assert.Equal(t, "test_value", val)
}

func Test_ClientGetError(t *testing.T) {
	cl := setupNATS(t)

	val, err := cl.Get(context.Background(), "nonexistent_key")

	require.Error(t, err)
	assert.Empty(t, val)
	assert.Contains(t, err.Error(), "key not found")
}

func Test_ClientDeleteSuccessError(t *testing.T) {
	cl := setupNATS(t)

	err := cl.Delete(context.Background(), "test_key")

	require.NoError(t, err)
}

func Test_ClientHealthCheck(t *testing.T) {
	cl := setupNATS(t)

	val, err := cl.HealthCheck(context.Background())

	require.NoError(t, err)
	assert.Contains(t, fmt.Sprint(val), "UP")
}
