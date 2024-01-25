package container

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_newContainerSuccessWithLogger(t *testing.T) {
	cfg := config.NewEnvFile("")

	container := NewContainer(cfg)

	assert.NotNil(t, container.Logger, "TEST, Failed.\nlogger initialisation")
}

func Test_newContainerDBIntializationFail(t *testing.T) {
	t.Setenv("REDIS_HOST", "invalid")
	t.Setenv("DB_HOST", "invalid")

	cfg := config.NewEnvFile("")

	container := NewContainer(cfg)

	// container is a pointer and we need to see if db are not initialized, comparing the container object
	// will not suffice the purpose of this test
	assert.Nil(t, container.DB, "TEST, Failed.\ninvalid db connections")
	assert.Nil(t, container.Redis, "TEST, Failed.\ninvalid redis connections")
}

func Test_HealthCheckServiceStatusUP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	container := NewContainer(config.NewEnvFile(""))

	container.Services = make(map[string]service.HTTPService)

	container.Services["test-service"] = service.NewHTTPService(server.URL, testutil.NewMockLogger(testutil.INFOLOG))

	response := container.Health(context.Background())

	assert.Equal(t, response, Health{Status: StatusUp, Services: []ServiceHealth{{Name: "test-service", Status: StatusUp}}})
}

func Test_HealthCheckServiceStatusDegraded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	container := NewContainer(config.NewEnvFile(""))

	container.Services = make(map[string]service.HTTPService)

	container.Services["test-service"] = service.NewHTTPService(server.URL, testutil.NewMockLogger(testutil.INFOLOG))

	response := container.Health(context.Background())

	assert.Equal(t, response, Health{Status: StatusDegraded, Services: []ServiceHealth{{Name: "test-service", Status: StatusDown}}})
}
