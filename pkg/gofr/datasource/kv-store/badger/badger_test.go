package badger

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupDB(t *testing.T) *Client {
	t.Helper()
	cl := New(Configs{DirPath: t.TempDir()})

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := NewMockLogger(ctrl)

	mockMetrics.EXPECT().NewHistogram("app_badger_stats", "Response time of Badger queries in milliseconds.", gomock.Any())

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_badger_stats", gomock.Any(), "database", cl.configs.DirPath,
		"type", gomock.Any()).AnyTimes()

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	cl.UseLogger(mockLogger)
	cl.UseMetrics(mockMetrics)
	cl.Connect()

	return cl
}

func Test_ClientSet(t *testing.T) {
	cl := setupDB(t)

	err := cl.Set(context.Background(), "lkey", "lvalue")

	require.NoError(t, err)
}

func Test_ClientGet(t *testing.T) {
	cl := setupDB(t)

	err := cl.Set(context.Background(), "lkey", "lvalue")
	require.NoError(t, err)

	val, err := cl.Get(context.Background(), "lkey")

	require.NoError(t, err)
	assert.Equal(t, "lvalue", val)
}

func Test_ClientGetError(t *testing.T) {
	cl := setupDB(t)

	val, err := cl.Get(context.Background(), "lkey")

	require.EqualError(t, err, "Key not found")
	assert.Empty(t, val)
}

func Test_ClientDeleteSuccessError(t *testing.T) {
	cl := setupDB(t)

	err := cl.Delete(context.Background(), "lkey")

	require.NoError(t, err)
}

func Test_ClientHealthCheck(t *testing.T) {
	cl := setupDB(t)

	val, err := cl.HealthCheck(context.Background())

	require.NoError(t, err)
	assert.Contains(t, fmt.Sprint(val), "UP")
}
