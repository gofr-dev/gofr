package arangodb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

var (
	errUserNotFound     = errors.New("user not found")
	errDBNotFound       = errors.New("database not found")
	errDocumentNotFound = errors.New("document not found")
)

func setupDB(t *testing.T) (*Client, *MockArango, *MockUser, *MockLogger, *MockMetrics) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockArango := NewMockArango(ctrl)
	mockUser := NewMockUser(ctrl)

	config := Config{Host: "localhost", Port: 8527, User: "root", Password: "root"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arangodb"))

	client.client = mockArango

	return client, mockArango, mockUser, mockLogger, mockMetrics
}

func Test_NewArangoClient_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	logger.EXPECT().Errorf("failed to verify connection: %v", gomock.Any())
	logger.EXPECT().Debugf(gomock.Any(), gomock.Any())

	client := New(Config{Host: "localhost", Port: 8529, Password: "root", User: "admin"})

	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.Connect()

	require.NotNil(t, client)
}

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		name      string
		config    Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid config",
			config: Config{
				Host:     "localhost",
				Port:     8529,
				User:     "root",
				Password: "password",
			},
			expectErr: false,
		},
		{
			name: "Empty host",
			config: Config{
				Port:     8529,
				User:     "root",
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: host is empty",
		},
		{
			name: "Empty port",
			config: Config{
				Host:     "localhost",
				User:     "root",
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: port is empty",
		},
		{
			name: "Empty user",
			config: Config{
				Host:     "localhost",
				Port:     8529,
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: user is empty",
		},
		{
			name: "Empty password",
			config: Config{
				Host: "localhost",
				Port: 8529,
				User: "root",
			},
			expectErr: true,
			errMsg:    "missing required field in config: password is empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &Client{config: &tc.config}
			err := client.validateConfig()

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
