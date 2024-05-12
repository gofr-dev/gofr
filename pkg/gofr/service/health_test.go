package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func TestHTTPService_HealthCheck(t *testing.T) {
	service, server, metrics := initializeTest(t, "alive", http.StatusOK)
	defer server.Close()

	ctx := context.Background()

	metrics.EXPECT().RecordHistogram(ctx, "app_http_service_response", gomock.Any(), "path", server.URL,
		"method", http.MethodGet, "status", fmt.Sprintf("%v", http.StatusOK)).Times(1)

	// when params value is of type []string then last value is sent in request
	resp := service.HealthCheck(ctx)

	assert.Equal(t, &Health{Status: serviceUp, Details: map[string]interface{}{"host": server.URL[7:]}},
		resp, "TEST[%d], Failed.\n%s")
}

func TestHTTPService_HealthCheckCustomURL(t *testing.T) {
	service, server, metrics := initializeTest(t, "ready", http.StatusOK)
	defer server.Close()

	ctx := context.Background()

	metrics.EXPECT().RecordHistogram(ctx, "app_http_service_response", gomock.Any(), "path", server.URL,
		"method", http.MethodGet, "status", fmt.Sprintf("%v", http.StatusOK)).Times(1)

	// when params value is of type []string then last value is sent in request
	resp := service.HealthCheck(ctx)

	assert.Equal(t, &Health{Status: serviceUp, Details: map[string]interface{}{"host": server.URL[7:]}},
		resp, "TEST[%d], Failed.\n%s")
}

func TestHTTPService_HealthCheckErrorResponse(t *testing.T) {
	service := NewHTTPService("http://test", logging.NewMockLogger(logging.INFO), nil)

	ctx := context.Background()

	// when params value is of type []string then last value is sent in request
	resp := service.HealthCheck(ctx)

	body, _ := json.Marshal(&resp)

	assert.Contains(t, string(body), `{"status":"DOWN","details":{"error":"Get \"http://test/.well-known/alive\"`)
}

func TestHTTPService_HealthCheckDifferentStatusCode(t *testing.T) {
	service, server, metrics := initializeTest(t, "bad-request", http.StatusBadRequest)
	defer server.Close()

	ctx := context.Background()

	metrics.EXPECT().RecordHistogram(ctx, "app_http_service_response", gomock.Any(), "path", server.URL,
		"method", http.MethodGet, "status", fmt.Sprintf("%v", http.StatusBadRequest)).AnyTimes()

	// when params value is of type []string then last value is sent in request
	resp := service.HealthCheck(ctx)

	assert.Equal(t, &Health{Status: serviceDown,
		Details: map[string]interface{}{"host": server.URL[7:], "error": "service down"}},
		resp, "TEST[%d], Failed.\n%s")
}

func initializeTest(t *testing.T, urlSuffix string, statusCode int) (HTTP, *httptest.Server, *MockMetrics) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/"+urlSuffix, r.URL.Path)

		if statusCode == http.StatusOK {
			_, _ = w.Write([]byte(`{"data":"UP"}`))
		}

		w.WriteHeader(statusCode)
	}))

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), metrics,
		&HealthConfig{HealthEndpoint: ".well-known/" + urlSuffix})

	return service, server, metrics
}
