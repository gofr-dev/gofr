package redis

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"

	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/testutil"
	"testing"
)

func TestRedis_HealthHandlerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	assert.Nil(t, err)

	defer s.Close()

	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "type", "ping")
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "type", "info")

	client := NewClient(testutil.NewMockConfig(map[string]string{
		"REDIS_HOST": s.Host(),
		"REDIS_PORT": s.Port(),
	}), testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric)
	assert.Nil(t, err)

	health := client.HealthCheck()

	assert.Equal(t, datasource.Health{
		Status:  "DOWN",
		Details: map[string]interface{}{"error": "section (Stats) is not supported", "host": s.Host() + ":" + s.Port()},
	}, health)
}
