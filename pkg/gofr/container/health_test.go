package container

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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
		expected := getExpectedData(tc.datasourceHealth, tc.appHealth, srv.URL)

		expectedJSONdata, _ := json.Marshal(expected)
		c, mocks := NewMockContainer(t)

		registerMocks(mocks, tc.datasourceHealth)

		c.appName = "test-app"
		c.appVersion = "test"
		c.Services = make(map[string]service.HTTP)
		c.Services["test-service"] = service.NewHTTPService(srv.URL, logger, nil)

		healthData := c.Health(t.Context())

		jsonData, _ := json.Marshal(healthData)

		assert.JSONEq(t, string(expectedJSONdata), string(jsonData), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func registerMocks(mocks *Mocks, health string) {
	mocks.SQL.ExpectHealthCheck().WillReturnHealthCheck(&datasource.Health{
		Status: health,
		Details: map[string]any{
			"host": "localhost:3306/test",
			"stats": sql.DBStats{
				MaxOpenConnections: 0, OpenConnections: 1, InUse: 0, Idle: 1, WaitCount: 0,
				WaitDuration: 0, MaxIdleClosed: 0, MaxIdleTimeClosed: 0, MaxLifetimeClosed: 0,
			},
		},
	})

	mocks.Redis.EXPECT().HealthCheck().Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:6379",
			"error": "redis not connected",
		},
	})

	mocks.Mongo.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:6379",
			"error": "mongo not connected",
		},
	}, nil)

	mocks.Cassandra.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:6379",
			"error": "cassandra not connected",
		},
	}, nil)

	mocks.Clickhouse.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:6379",
			"error": "clickhouse not connected",
		},
	}, nil)

	mocks.Oracle.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:1521",
			"error": "oracle not connected",
		},
	}, nil)

	mocks.KVStore.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:1234",
			"error": "kv-store not connected",
		},
	}, nil)

	mocks.DGraph.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:8000",
			"error": "dgraph not connected",
		},
	}, nil)

	mocks.OpenTSDB.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:8000",
			"error": "opentsdb not connected",
		},
	}, nil)

	mocks.PubSub.EXPECT().Health().Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:pubsub",
			"error": nil,
		},
	}).Times(1)

	mocks.Elasticsearch.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{
		Status: health,
		Details: map[string]any{
			"host":  "localhost:9200",
			"error": "elasticsearch not connected",
		},
	}, nil)
}

func getExpectedData(datasourceHealth, appHealth, srvURL string) map[string]any {
	return map[string]any{
		"kv-store": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:1234",
				"error": "kv-store not connected",
			},
		},
		"redis": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:6379",
				"error": "redis not connected",
			},
		},
		"mongo": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:6379",
				"error": "mongo not connected",
			},
		},
		"clickHouse": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:6379",
				"error": "clickhouse not connected",
			},
		},
		"oracle": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:1521",
				"error": "oracle not connected",
			},
		},
		"cassandra": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:6379",
				"error": "cassandra not connected",
			},
		},

		"sql": &datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host": "localhost:3306/test",
				"stats": sql.DBStats{
					MaxOpenConnections: 0, OpenConnections: 1, InUse: 0, Idle: 1, WaitCount: 0,
					WaitDuration: 0, MaxIdleClosed: 0, MaxIdleTimeClosed: 0, MaxLifetimeClosed: 0,
				},
			},
		},
		"dgraph": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:8000",
				"error": "dgraph not connected",
			},
		},
		"opentsdb": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:8000",
				"error": "opentsdb not connected",
			},
		},
		"elasticsearch": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:9200",
				"error": "elasticsearch not connected",
			},
		},
		"pubsub": datasource.Health{
			Status: datasourceHealth, Details: map[string]any{
				"host":  "localhost:pubsub",
				"error": nil,
			},
		},
		"test-service": &service.Health{
			Status: "UP", Details: map[string]any{
				"host": strings.TrimPrefix(srvURL, "http://"),
			},
		},
		"name":    "test-app",
		"status":  appHealth,
		"version": "test",
	}
}
