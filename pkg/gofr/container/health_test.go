package container

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

func TestContainer_Health(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	logger := logging.NewMockLogger(logging.ERROR)

	tests := []struct {
		desc             string
		datasourceHealth string
		appHealth        string
	}{
		{"datasources UP", "UP", "UP"},
		{"datasources DOWN", "DOWN", "DEGRADED"},
	}

	for i, tc := range tests {
		expected := map[string]interface{}{
			"redis": datasource.Health{
				Status: tc.datasourceHealth,
				Details: map[string]interface{}{
					"host":  "localhost:6379",
					"error": "redis not connected",
				},
			},
			"sql": &datasource.Health{
				Status: tc.datasourceHealth,
				Details: map[string]interface{}{
					"host": "localhost:3306/test",
					"stats": sql.DBStats{
						MaxOpenConnections: 0,
						OpenConnections:    1,
						InUse:              0,
						Idle:               1,
						WaitCount:          0,
						WaitDuration:       0,
						MaxIdleClosed:      0,
						MaxIdleTimeClosed:  0,
						MaxLifetimeClosed:  0,
					},
				},
			},
			"test-service": &service.Health{
				Status: "UP",
				Details: map[string]interface{}{
					"host": strings.TrimPrefix(srv.URL, "http://"),
				},
			},
			"name":    "test-app",
			"status":  tc.appHealth,
			"version": "test",
		}

		c, mocks := NewMockContainer(t)

		c.appName = "test-app"
		c.appVersion = "test"

		c.Services = make(map[string]service.HTTP)
		c.Services["test-service"] = service.NewHTTPService(srv.URL, logger, nil)

		mocks.SQL.EXPECT().HealthCheck().Return(&datasource.Health{
			Status: tc.datasourceHealth,
			Details: map[string]interface{}{
				"host": "localhost:3306/test",
				"stats": sql.DBStats{
					MaxOpenConnections: 0,
					OpenConnections:    1,
					InUse:              0,
					Idle:               1,
					WaitCount:          0,
					WaitDuration:       0,
					MaxIdleClosed:      0,
					MaxIdleTimeClosed:  0,
					MaxLifetimeClosed:  0,
				},
			},
		})

		mocks.Redis.EXPECT().HealthCheck().Return(datasource.Health{
			Status: tc.datasourceHealth,
			Details: map[string]interface{}{
				"host":  "localhost:6379",
				"error": "redis not connected",
			},
		})

		healthData := c.Health(context.Background())

		assert.Equal(t, expected, healthData, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
