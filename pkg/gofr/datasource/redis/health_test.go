package redis

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
)

func TestRedis_HealthHandlerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	require.NoError(t, err)

	defer s.Close()

	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(),
		"hostname", gomock.Any(), "type", "ping")
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(),
		"hostname", gomock.Any(), "type", "info")

	client := NewClient(config.NewMockConfig(map[string]string{
		"REDIS_HOST": s.Host(),
		"REDIS_PORT": s.Port(),
	}), logging.NewMockLogger(logging.DEBUG), mockMetric)

	require.NoError(t, err)

	health := client.HealthCheck()

	assert.Equal(t, datasource.Health{
		Status:  "DOWN",
		Details: map[string]interface{}{"error": "section (Stats) is not supported", "host": s.Host() + ":" + s.Port()},
	}, health)
}

func TestRedisHealth_WithoutRedis(t *testing.T) {
	client := Redis{
		Client: nil,
		logger: logging.NewMockLogger(logging.ERROR),
		config: &Config{
			HostName: "localhost",
			Port:     2003,
		},
	}

	health := client.HealthCheck()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "redis not connected", health.Details["error"])
}
