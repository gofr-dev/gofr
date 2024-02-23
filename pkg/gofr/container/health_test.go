package container

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func TestContainer_Health(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("could not initialize mock database err : %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	logger := testutil.NewMockLogger(testutil.ERRORLOG)

	expected := map[string]interface{}{
		"redis": datasource.Health{
			Status: "DOWN",
			Details: map[string]interface{}{
				"host":  "localhost:6379",
				"error": "redis not connected",
			},
		},
		"sql": &datasource.Health{
			Status: "UP",
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
	}

	c := NewContainer(testutil.NewMockConfig(map[string]string{
		"DB_HOST":     "localhost",
		"DB_DIALECT":  "mysql",
		"DB_USER":     "user",
		"DB_PASSWORD": "password",
		"DB_NAME":     "test",
		"REDIS_HOST":  "localhost",
		"REDIS_PORT":  "6379",
	}))

	c.Services = make(map[string]service.HTTP)
	c.Services["test-service"] = service.NewHTTPService(srv.URL, logger, nil)

	c.DB.DB = mockDB

	mock.ExpectPing()

	healthData := c.Health(context.Background())

	assert.Equal(t, expected, healthData)
}
