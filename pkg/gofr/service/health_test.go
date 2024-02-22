package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/testutil"
)

func TestHTTPService_HealthCheck(t *testing.T) {
	service, server, metrics := initialSetup(t, "alive")
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
	service, server, metrics := initialSetup(t, "ready")
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
	service, server, metrics := initialSetup(t, "bad-request")
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

func initialSetup(t *testing.T, urlSuffix string) (HTTP, *httptest.Server, *MockMetrics) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/"+urlSuffix, r.URL.Path)

		_, _ = w.Write([]byte(`{"data":"UP"}`))
		w.WriteHeader(http.StatusOK)
	}))

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.INFOLOG), metrics,
		&HealthConfig{HealthEndpoint: ".well-known/" + urlSuffix})

	return service, server, metrics
}
