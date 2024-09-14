package nats

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

const (
	natsServer = "nats://localhost:4222"
)

// mockConn is a minimal mock implementation of nats.Conn.
type mockConn struct {
	status nats.Status
}

func (m *mockConn) Status() nats.Status {
	return m.status
}

// mockJetStream is a minimal mock implementation of jetstream.JetStream.
type mockJetStream struct {
	accountInfoErr error
}

func (m *mockJetStream) AccountInfo(_ context.Context) (*jetstream.AccountInfo, error) {
	if m.accountInfoErr != nil {
		return nil, m.accountInfoErr
	}

	return &jetstream.AccountInfo{}, nil
}

// testNATSClient is a test-specific implementation of NATSClient.
type testNATSClient struct {
	NATSClient
	mockConn      *mockConn
	mockJetStream *mockJetStream
}

func (c *testNATSClient) Health() datasource.Health {
	health := datasource.Health{
		Details: make(map[string]interface{}),
	}

	health.Status = datasource.StatusUp
	connectionStatus := c.mockConn.Status()

	switch connectionStatus {
	case nats.CONNECTING:
		health.Status = datasource.StatusUp
		health.Details["connection_status"] = jetstreamConnecting
	case nats.CONNECTED:
		health.Details["connection_status"] = jetstreamConnected
	case nats.CLOSED, nats.DISCONNECTED, nats.RECONNECTING, nats.DRAINING_PUBS, nats.DRAINING_SUBS:
		health.Status = datasource.StatusDown
		health.Details["connection_status"] = jetstreamDisconnected
	default:
		health.Status = datasource.StatusDown
		health.Details["connection_status"] = connectionStatus.String()
	}

	health.Details["host"] = c.config.Server
	health.Details["backend"] = natsBackend
	health.Details["jetstream_enabled"] = c.mockJetStream != nil

	if c.mockJetStream != nil {
		_, err := c.mockJetStream.AccountInfo(context.Background())
		if err != nil {
			health.Details["jetstream_status"] = jetstreamStatusError + ": " + err.Error()
		} else {
			health.Details["jetstream_status"] = jetstreamStatusOK
		}
	}

	return health
}

func TestNATSClient_HealthStatusUP(t *testing.T) {
	client := &testNATSClient{
		NATSClient: NATSClient{
			config: &Config{Server: natsServer},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn:      &mockConn{status: nats.CONNECTED},
		mockJetStream: &mockJetStream{},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, natsServer, health.Details["host"])
	assert.Equal(t, natsBackend, health.Details["backend"])
	assert.Equal(t, jetstreamConnected, health.Details["connection_status"])
	assert.Equal(t, true, health.Details["jetstream_enabled"])
	assert.Equal(t, jetstreamStatusOK, health.Details["jetstream_status"])
}

func TestNATSClient_HealthStatusDown(t *testing.T) {
	client := &testNATSClient{
		NATSClient: NATSClient{
			config: &Config{Server: natsServer},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn: &mockConn{status: nats.CLOSED},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, natsServer, health.Details["host"])
	assert.Equal(t, natsBackend, health.Details["backend"])
	assert.Equal(t, jetstreamDisconnected, health.Details["connection_status"])
	assert.Equal(t, false, health.Details["jetstream_enabled"])
}

func TestNATSClient_HealthJetStreamError(t *testing.T) {
	client := &testNATSClient{
		NATSClient: NATSClient{
			config: &Config{Server: natsServer},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn:      &mockConn{status: nats.CONNECTED},
		mockJetStream: &mockJetStream{accountInfoErr: errJetStream},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, natsServer, health.Details["host"])
	assert.Equal(t, natsBackend, health.Details["backend"])
	assert.Equal(t, jetstreamConnected, health.Details["connection_status"])
	assert.Equal(t, true, health.Details["jetstream_enabled"])
	assert.Equal(t, jetstreamStatusError+": "+errJetStream.Error(), health.Details["jetstream_status"])
}

func TestNATSClient_Health(t *testing.T) {
	testCases := []struct {
		name            string
		setupMocks      func(*MockConnInterface, *MockJetStream)
		expectedStatus  string
		expectedDetails map[string]interface{}
		expectedLogs    []string
	}{
		{
			name: "HealthyConnection",
			setupMocks: func(mockConn *MockConnInterface, mockJS *MockJetStream) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED)
				mockJS.EXPECT().AccountInfo(gomock.Any()).Return(&jetstream.AccountInfo{}, nil)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              natsServer,
				"backend":           natsBackend,
				"connection_status": jetstreamConnected,
				"jetstream_enabled": true,
				"jetstream_status":  jetstreamStatusOK,
			},
			expectedLogs: []string{"NATS health check: Connected"},
		},
		{
			name: "DisconnectedStatus",
			setupMocks: func(mockConn *MockConnInterface, _ *MockJetStream) {
				mockConn.EXPECT().Status().Return(nats.DISCONNECTED)
			},
			expectedStatus: datasource.StatusDown,
			expectedDetails: map[string]interface{}{
				"host":              natsServer,
				"backend":           natsBackend,
				"connection_status": jetstreamDisconnected,
				"jetstream_enabled": true,
			},
			expectedLogs: []string{"NATS health check: Disconnected"},
		},
		{
			name: "JetStreamError",
			setupMocks: func(mockConn *MockConnInterface, mockJS *MockJetStream) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED)
				mockJS.EXPECT().AccountInfo(gomock.Any()).Return(nil, errJetStream)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              natsServer,
				"backend":           natsBackend,
				"connection_status": jetstreamConnected,
				"jetstream_enabled": true,
				"jetstream_status":  jetstreamStatusError + ": " + errJetStream.Error(),
			},
			expectedLogs: []string{"NATS health check: JetStream error"},
		},
		{
			name: "NoJetStream",
			setupMocks: func(mockConn *MockConnInterface, _ *MockJetStream) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              natsServer,
				"backend":           natsBackend,
				"connection_status": jetstreamConnected,
				"jetstream_enabled": false,
			},
			expectedLogs: []string{"NATS health check: JetStream not enabled"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConn := NewMockConnInterface(ctrl)
			mockJS := NewMockJetStream(ctrl)

			tc.setupMocks(mockConn, mockJS)

			client := &NATSClient{
				conn:   mockConn,
				js:     mockJS,
				config: &Config{Server: natsServer},
				logger: logging.NewMockLogger(logging.DEBUG),
			}

			if tc.name == "NoJetStream" {
				client.js = nil
			}

			logs := testutil.StdoutOutputForFunc(func() {
				health := client.Health()

				assert.Equal(t, tc.expectedStatus, health.Status)
				assert.Equal(t, tc.expectedDetails, health.Details)
			})

			for _, expectedLog := range tc.expectedLogs {
				assert.Contains(t, logs, expectedLog)
			}
		})
	}
}
