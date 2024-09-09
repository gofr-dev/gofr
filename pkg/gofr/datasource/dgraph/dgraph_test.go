package dgraph

import (
	"testing"

	"go.uber.org/mock/gomock"
)

func TestClient_Connect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetric := NewMockMetrics(ctrl)

	config := Config{Host: "localhost", Port: "9080"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetric)

	// Mock expected logger behavior for logging the connection error
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).Times(1) // Expect Error to be called once when HealthCheck fails
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).Times(2)
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(1)

	// Mock Metric behavior
	mockMetric.EXPECT().NewHistogram(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Perform the connect operation
	err := client.Connect()

	// Validate the error
	if err == nil {
		t.Fatalf("expected connection error, got none")
	}

	// Optionally, you can check if the error message matches what you expect
	expectedError := "dgraph health check failed"
	if err.Error() != expectedError {
		t.Errorf("expected error: %v, got: %v", expectedError, err)
	}
}
