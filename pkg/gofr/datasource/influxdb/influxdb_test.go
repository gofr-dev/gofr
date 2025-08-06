package influxdb

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
	"go.opentelemetry.io/otel"
)

func setupDB(t *testing.T) *Client {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockInfluxClient := NewMockInfluxClient(ctrl)

	config := Config{
		URL:      "http://localhost:8086",
		Username: "admin",
		Password: "admin1234",
		Token:    "token",
	}

	client := New(config)

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-influxdb"))

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	// mockMetrics.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any())

	// Replace the client with our mocked version
	client.client = mockInfluxClient
	return client
}

func Test_HelthCheckSuccess(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mockInflux := NewMockInfluxDB(ctrl)
	client := *setupDB(t)
	client.HealthCheck(t.Context())

	// client.HealthCheck(t.Context())

}
