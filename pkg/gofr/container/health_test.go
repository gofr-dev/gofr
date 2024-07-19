package container

import (
	"context"
	"encoding/json"
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
			"kv-store": datasource.Health{
				Status: tc.datasourceHealth, Details: map[string]interface{}{
					"host":  "localhost:1234",
					"error": "kv-store not connected",
				},
			},
			"redis": datasource.Health{
				Status: tc.datasourceHealth, Details: map[string]interface{}{
					"host":  "localhost:6379",
					"error": "redis not connected",
				},
			},
			"mongo": datasource.Health{
				Status: tc.datasourceHealth, Details: map[string]interface{}{
					"host":  "localhost:6379",
					"error": "mongo not connected",
				},
			},
			"clickHouse": datasource.Health{
				Status: tc.datasourceHealth, Details: map[string]interface{}{
					"host":  "localhost:6379",
					"error": "clickhouse not connected",
				},
			},
			"cassandra": datasource.Health{
				Status: tc.datasourceHealth, Details: map[string]interface{}{
					"host":  "localhost:6379",
					"error": "cassandra not connected",
				},
			},

			"sql": &datasource.Health{
				Status: tc.datasourceHealth, Details: map[string]interface{}{
					"host": "localhost:3306/test",
					"stats": sql.DBStats{
						MaxOpenConnections: 0, OpenConnections: 1, InUse: 0, Idle: 1, WaitCount: 0,
						WaitDuration: 0, MaxIdleClosed: 0, MaxIdleTimeClosed: 0, MaxLifetimeClosed: 0,
					},
				},
			},
			"test-service": &service.Health{
				Status: "UP", Details: map[string]interface{}{
					"host": strings.TrimPrefix(srv.URL, "http://"),
				},
			},
			"name":    "test-app",
			"status":  tc.appHealth,
			"version": "test",
		}

		expectedJSONdata, _ := json.Marshal(expected)

		c, mocks := NewMockContainer(t)

		registerMocks(mocks, tc.datasourceHealth)

		c.appName = "test-app"
		c.appVersion = "test"

		c.Services = make(map[string]service.HTTP)
		c.Services["test-service"] = service.NewHTTPService(srv.URL, logger, nil)

		healthData := c.Health(context.Background())

		jsonData, _ := json.Marshal(healthData)

		assert.Equal(t, string(expectedJSONdata), string(jsonData), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func registerMocks(mocks Mocks, health string) {
	mocks.SQL.EXPECT().HealthCheck().Return(&datasource.Health{
		Status: health,
		Details: map[string]interface{}{
			"host": "localhost:3306/test",
			"stats": sql.DBStats{
				MaxOpenConnections: 0, OpenConnections: 1, InUse: 0, Idle: 1, WaitCount: 0,
				WaitDuration: 0, MaxIdleClosed: 0, MaxIdleTimeClosed: 0, MaxLifetimeClosed: 0,
			},
		},
	})

	mocks.Redis.EXPECT().HealthCheck().Return(datasource.Health{
		Status: health,
		Details: map[string]interface{}{
			"host":  "localhost:6379",
			"error": "redis not connected",
		},
	})

	mocks.Mongo.EXPECT().HealthCheck(context.Background()).Return(datasource.Health{
		Status: health,
		Details: map[string]interface{}{
			"host":  "localhost:6379",
			"error": "mongo not connected",
		},
	}, nil)

	mocks.Cassandra.EXPECT().HealthCheck(context.Background()).Return(datasource.Health{
		Status: health,
		Details: map[string]interface{}{
			"host":  "localhost:6379",
			"error": "cassandra not connected",
		},
	}, nil)

	mocks.Clickhouse.EXPECT().HealthCheck(context.Background()).Return(datasource.Health{
		Status: health,
		Details: map[string]interface{}{
			"host":  "localhost:6379",
			"error": "clickhouse not connected",
		},
	}, nil)

	mocks.KVStore.EXPECT().HealthCheck(context.Background()).Return(datasource.Health{
		Status: health,
		Details: map[string]interface{}{
			"host":  "localhost:1234",
			"error": "kv-store not connected",
		},
	}, nil)
}
