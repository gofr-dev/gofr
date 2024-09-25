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
	// NatsServer is the address of a local Client server. Used for testing.
	NatsServer = "nats://localhost:4222"
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

// testNATSClient is a test-specific implementation of Client.
type testNATSClient struct {
	Client
	mockConn      *mockConn
	mockJetStream *mockJetStream
}

func (c *testNATSClient) Health() datasource.Health {
	h := datasource.Health{
		Details: make(map[string]interface{}),
	}

	h.Status = datasource.StatusUp
	connectionStatus := c.mockConn.Status()

	switch connectionStatus {
	case nats.CONNECTING:
		h.Status = datasource.StatusUp
		h.Details["connection_status"] = jetStreamConnecting
	case nats.CONNECTED:
		h.Details["connection_status"] = jetStreamConnected
	case nats.CLOSED, nats.DISCONNECTED, nats.RECONNECTING, nats.DRAINING_PUBS, nats.DRAINING_SUBS:
		h.Status = datasource.StatusDown
		h.Details["connection_status"] = jetStreamDisconnecting
	default:
		h.Status = datasource.StatusDown
		h.Details["connection_status"] = connectionStatus.String()
	}

	h.Details["host"] = c.Config.Server
	h.Details["backend"] = natsBackend
	h.Details["jetstream_enabled"] = c.mockJetStream != nil

	if c.mockJetStream != nil {
		_, err := c.mockJetStream.AccountInfo(context.Background())
		if err != nil {
			h.Details["jetstream_status"] = jetStreamStatusError + ": " + err.Error()
		} else {
			h.Details["jetstream_status"] = jetStreamStatusOK
		}
	}

	return h
}

func TestNATSClient_HealthStatusUP(t *testing.T) {
	client := &testNATSClient{
		Client: Client{
			Config: &Config{Server: NatsServer},
			Logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn:      &mockConn{status: nats.CONNECTED},
		mockJetStream: &mockJetStream{},
	}

	h := client.Health()

	assert.Equal(t, datasource.StatusUp, h.Status)
	assert.Equal(t, NatsServer, h.Details["host"])
	assert.Equal(t, natsBackend, h.Details["backend"])
	assert.Equal(t, jetStreamConnected, h.Details["connection_status"])
	assert.Equal(t, true, h.Details["jetstream_enabled"])
	assert.Equal(t, jetStreamStatusOK, h.Details["jetstream_status"])
}

func TestNATSClient_HealthStatusDown(t *testing.T) {
	client := &testNATSClient{
		Client: Client{
			Config: &Config{Server: NatsServer},
			Logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn: &mockConn{status: nats.CLOSED},
	}

	h := client.Health()

	assert.Equal(t, datasource.StatusDown, h.Status)
	assert.Equal(t, NatsServer, h.Details["host"])
	assert.Equal(t, natsBackend, h.Details["backend"])
	assert.Equal(t, jetStreamDisconnecting, h.Details["connection_status"])
	assert.Equal(t, false, h.Details["jetstream_enabled"])
}

func TestNATSClient_HealthJetStreamError(t *testing.T) {
	client := &testNATSClient{
		Client: Client{
			Config: &Config{Server: NatsServer},
			Logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn:      &mockConn{status: nats.CONNECTED},
		mockJetStream: &mockJetStream{accountInfoErr: errJetStream},
	}

	h := client.Health()

	assert.Equal(t, datasource.StatusUp, h.Status)
	assert.Equal(t, NatsServer, h.Details["host"])
	assert.Equal(t, natsBackend, h.Details["backend"])
	assert.Equal(t, jetStreamConnected, h.Details["connection_status"])
	assert.Equal(t, true, h.Details["jetstream_enabled"])
	assert.Equal(t, jetStreamStatusError+": "+errJetStream.Error(), h.Details["jetstream_status"])
}

func TestNATSClient_Health(t *testing.T) {
	testCases := defineHealthTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runHealthTestCase(t, tc)
		})
	}
}

func defineHealthTestCases() []healthTestCase {
	return []healthTestCase{
		{
			name: "HealthyConnection",
			setupMocks: func(mockConn *MockConnInterface, mockJS *MockJetStream) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED).Times(2)
				mockJS.EXPECT().AccountInfo(gomock.Any()).Return(&jetstream.AccountInfo{}, nil).Times(2)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              NatsServer,
				"backend":           natsBackend,
				"connection_status": jetStreamConnected,
				"jetstream_enabled": true,
				"jetstream_status":  jetStreamStatusOK,
			},
			expectedLogs: []string{"Client health check: Connected", "Client health check: JetStream enabled"},
		},
		{
			name: "DisconnectedStatus",
			setupMocks: func(mockConn *MockConnInterface, _ *MockJetStream) {
				mockConn.EXPECT().Status().Return(nats.DISCONNECTED).Times(2)
			},
			expectedStatus: datasource.StatusDown,
			expectedDetails: map[string]interface{}{
				"host":              NatsServer,
				"backend":           natsBackend,
				"connection_status": jetStreamDisconnecting,
				"jetstream_enabled": true,
			},
			expectedLogs: []string{"Client health check: Disconnected"},
		},
		{
			name: "JetStreamError",
			setupMocks: func(mockConn *MockConnInterface, mockJS *MockJetStream) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED).Times(2)
				mockJS.EXPECT().AccountInfo(gomock.Any()).Return(nil, errJetStream).Times(2)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              NatsServer,
				"backend":           natsBackend,
				"connection_status": jetStreamConnected,
				"jetstream_enabled": true,
				"jetstream_status":  jetStreamStatusError + ": " + errJetStream.Error(),
			},
			expectedLogs: []string{"Client health check: Connected", "Client health check: JetStream error"},
		},
		{
			name: "NoJetStream",
			setupMocks: func(mockConn *MockConnInterface, _ *MockJetStream) {
				mockConn.EXPECT().Status().Return(nats.CONNECTED).Times(2)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]interface{}{
				"host":              NatsServer,
				"backend":           natsBackend,
				"connection_status": jetStreamConnected,
				"jetstream_enabled": false,
			},
			expectedLogs: []string{"Client health check: Connected", "Client health check: JetStream not enabled"},
		},
	}
}

func runHealthTestCase(t *testing.T, tc healthTestCase) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnInterface(ctrl)
	mockJS := NewMockJetStream(ctrl)

	tc.setupMocks(mockConn, mockJS)

	client := &Client{
		Conn:      mockConn,
		JetStream: mockJS,
		Config:    &Config{Server: NatsServer},
	}

	if tc.name == "NoJetStream" {
		client.JetStream = nil
	}

	var h datasource.Health

	combinedLogs := getCombinedLogs(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		h = client.Health()
	})

	assert.Equal(t, tc.expectedStatus, h.Status)
	assert.Equal(t, tc.expectedDetails, h.Details)

	for _, expectedLog := range tc.expectedLogs {
		assert.Contains(t, combinedLogs, expectedLog, "Expected log message not found: %s", expectedLog)
	}
}

func getCombinedLogs(f func()) string {
	stdoutLogs := testutil.StdoutOutputForFunc(f)
	stderrLogs := testutil.StderrOutputForFunc(f)

	return stdoutLogs + stderrLogs
}

type healthTestCase struct {
	name            string
	setupMocks      func(*MockConnInterface, *MockJetStream)
	expectedStatus  string
	expectedDetails map[string]interface{}
	expectedLogs    []string
}
