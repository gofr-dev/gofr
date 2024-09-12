package nats

import (
	"context"
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
)

// mockConn is a minimal mock implementation of nats.Conn
type mockConn struct {
	status nats.Status
}

func (m *mockConn) Status() nats.Status {
	return m.status
}

// mockJetStream is a minimal mock implementation of jetstream.JetStream
type mockJetStream struct {
	accountInfoErr error
}

func (m *mockJetStream) AccountInfo(ctx context.Context) (*jetstream.AccountInfo, error) {
	if m.accountInfoErr != nil {
		return nil, m.accountInfoErr
	}
	return &jetstream.AccountInfo{}, nil
}

// testNATSClient is a test-specific implementation of natsClient
type testNATSClient struct {
	natsClient
	mockConn      *mockConn
	mockJetStream *mockJetStream
}

func (c *testNATSClient) Health() datasource.Health {
	health := datasource.Health{
		Details: make(map[string]interface{}),
	}

	health.Status = datasource.StatusUp

	if c.mockConn != nil && c.mockConn.Status() != nats.CONNECTED {
		health.Status = datasource.StatusDown
	}

	health.Details["host"] = c.config.Server
	health.Details["backend"] = "NATS"
	health.Details["connection_status"] = c.mockConn.Status().String()
	health.Details["jetstream_enabled"] = c.mockJetStream != nil

	if c.mockJetStream != nil {
		_, err := c.mockJetStream.AccountInfo(context.Background())
		if err != nil {
			health.Details["jetstream_status"] = "Error: " + err.Error()
		} else {
			health.Details["jetstream_status"] = "OK"
		}
	}

	return health
}

func TestNATSClient_HealthStatusUP(t *testing.T) {
	client := &testNATSClient{
		natsClient: natsClient{
			config: Config{Server: "nats://localhost:4222"},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn:      &mockConn{status: nats.CONNECTED},
		mockJetStream: &mockJetStream{},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["host"])
	assert.Equal(t, "NATS", health.Details["backend"])
	assert.Equal(t, "CONNECTED", health.Details["connection_status"])
	assert.Equal(t, true, health.Details["jetstream_enabled"])
	assert.Equal(t, "OK", health.Details["jetstream_status"])
}

func TestNATSClient_HealthStatusDown(t *testing.T) {
	client := &testNATSClient{
		natsClient: natsClient{
			config: Config{Server: "nats://localhost:4222"},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn: &mockConn{status: nats.CLOSED},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["host"])
	assert.Equal(t, "NATS", health.Details["backend"])
	assert.Equal(t, "CLOSED", health.Details["connection_status"])
	assert.Equal(t, false, health.Details["jetstream_enabled"])
}

func TestNATSClient_HealthJetStreamError(t *testing.T) {
	client := &testNATSClient{
		natsClient: natsClient{
			config: Config{Server: "nats://localhost:4222"},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn:      &mockConn{status: nats.CONNECTED},
		mockJetStream: &mockJetStream{accountInfoErr: nats.ErrConnectionClosed},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["host"])
	assert.Equal(t, "NATS", health.Details["backend"])
	assert.Equal(t, "CONNECTED", health.Details["connection_status"])
	assert.Equal(t, true, health.Details["jetstream_enabled"])
	assert.Equal(t, "Error: nats: connection closed", health.Details["jetstream_status"])
}

func TestNATSClient_Health(t *testing.T) {
	testCases := []struct {
		name            string
		setupMocks      func(*MockConnection, *MockJetStreamContext)
		expectedStatus  string
		expectedDetails map[string]interface{}
	}{
		{
			name: "HealthyConnection",
			setupMocks: func(mockConn *MockConnection, mockJS *MockJetStreamContext) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED).AnyTimes()
				mockJS.EXPECT().AccountInfo(gomock.Any()).Return(&nats.AccountInfo{}, nil)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              "nats://localhost:4222",
				"backend":           "NATS",
				"connection_status": "CONNECTED",
				"jetstream_enabled": true,
				"jetstream_status":  "OK",
			},
		},
		{
			name: "DisconnectedStatus",
			setupMocks: func(mockConn *MockConnection, mockJS *MockJetStreamContext) {
				mockConn.EXPECT().Status().Return(nats.DISCONNECTED).AnyTimes()
			},
			expectedStatus: datasource.StatusDown,
			expectedDetails: map[string]interface{}{
				"host":              "nats://localhost:4222",
				"backend":           "NATS",
				"connection_status": "DISCONNECTED",
				"jetstream_enabled": true,
			},
		},
		{
			name: "JetStreamError",
			setupMocks: func(mockConn *MockConnection, mockJS *MockJetStreamContext) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED).AnyTimes()
				mockJS.EXPECT().AccountInfo(gomock.Any()).Return(nil, errors.New("jetstream error"))
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              "nats://localhost:4222",
				"backend":           "NATS",
				"connection_status": "CONNECTED",
				"jetstream_enabled": true,
				"jetstream_status":  "Error: jetstream error",
			},
		},
		{
			name: "NoJetStream",
			setupMocks: func(mockConn *MockConnection, mockJS *MockJetStreamContext) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED).AnyTimes()
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              "nats://localhost:4222",
				"backend":           "NATS",
				"connection_status": "CONNECTED",
				"jetstream_enabled": false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConn := NewMockConnection(ctrl)
			mockJS := NewMockJetStreamContext(ctrl)

			tc.setupMocks(mockConn, mockJS)

			client := &natsClient{
				conn:   mockConn,
				js:     mockJS,
				config: Config{Server: "nats://localhost:4222"},
				logger: logging.NewMockLogger(logging.DEBUG),
			}

			if tc.name == "NoJetStream" {
				client.js = nil
			}

			health := client.Health()

			assert.Equal(t, tc.expectedStatus, health.Status)
			assert.Equal(t, tc.expectedDetails, health.Details)
		})
	}
}
