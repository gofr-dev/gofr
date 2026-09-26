package nats

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
)

// natsMsgMatcher is a gomock matcher that validates nats.Msg fields.
type natsMsgMatcher struct {
	subject string
	data    []byte
}

func (m natsMsgMatcher) Matches(x any) bool {
	msg, ok := x.(*nats.Msg)
	return ok && msg.Subject == m.subject && bytes.Equal(msg.Data, m.data)
}

func (m natsMsgMatcher) String() string {
	return fmt.Sprintf("nats.Msg{Subject: %q, Data: %q}", m.subject, m.data)
}

func TestNewConnectionManager(t *testing.T) {
	cfg := &Config{Server: "nats://localhost:4222"}
	logger := logging.NewMockLogger(logging.DEBUG)
	natsConnector := &MockNATSConnector{}
	jsCreator := &MockJetStreamCreator{}

	cm := NewConnectionManager(cfg, logger, natsConnector, jsCreator)

	assert.NotNil(t, cm)
	assert.Equal(t, cfg, cm.config)
	assert.Equal(t, logger, cm.logger)
	assert.Equal(t, natsConnector, cm.natsConnector)
	assert.Equal(t, jsCreator, cm.jetStreamCreator)
}

func TestConnectionManager_Connect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnInterface(ctrl)
	mockJS := NewMockJetStream(ctrl)
	mockNATSConnector := NewMockNATSConnector(ctrl)
	mockJSCreator := NewMockJetStreamCreator(ctrl)

	cm := NewConnectionManager(
		&Config{Server: "nats://localhost:4222"},
		logging.NewMockLogger(logging.DEBUG),
		mockNATSConnector,
		mockJSCreator,
	)

	mockNATSConnector.EXPECT().Connect(gomock.Any(), gomock.Any()).Return(mockConn, nil)

	// We don't need to expect NATSConn() call anymore, as we're passing mockConn directly to New()
	mockJSCreator.EXPECT().New(mockConn).Return(mockJS, nil)

	err := cm.Connect()

	time.Sleep(100 * time.Millisecond)

	require.NoError(t, err)
	assert.Equal(t, mockConn, cm.conn)
	assert.Equal(t, mockJS, cm.jStream)
}

func TestConnectionManager_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnInterface(ctrl)
	cm := &ConnectionManager{
		conn: mockConn,
	}

	mockConn.EXPECT().Close()

	ctx := t.Context()
	cm.Close(ctx)
}

func TestConnectionManager_Publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockConn := NewMockConnInterface(ctrl)

	cm := &ConnectionManager{
		conn:    mockConn,
		jStream: mockJS,
		logger:  logging.NewMockLogger(logging.DEBUG),
	}

	ctx := t.Context()
	subject := "test.subject"
	message := []byte("test message")

	gomock.InOrder(
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", subject),
		mockConn.EXPECT().Status().Return(nats.CONNECTED),
		mockJS.EXPECT().PublishMsg(gomock.Any(), natsMsgMatcher{subject: subject, data: message}).Return(&jetstream.PubAck{}, nil),
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "subject", subject),
	)

	err := cm.Publish(ctx, subject, message, mockMetrics)
	require.NoError(t, err)
}

func TestConnectionManager_validateJetStream(t *testing.T) {
	cm := &ConnectionManager{
		jStream: NewMockJetStream(gomock.NewController(t)),
		logger:  logging.NewMockLogger(logging.DEBUG),
	}

	err := cm.validateJetStream("test.subject")
	require.NoError(t, err)

	cm.jStream = nil
	err = cm.validateJetStream("test.subject")
	assert.Equal(t, errJetStreamNotConfigured, err)

	cm.jStream = NewMockJetStream(gomock.NewController(t))
	err = cm.validateJetStream("")
	assert.Equal(t, errJetStreamNotConfigured, err)
}

func TestConnectionManager_Health(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnInterface(ctrl)
	cm := &ConnectionManager{
		conn: mockConn,
		config: &Config{
			Server: "nats://localhost:4222",
		},
	}

	mockConn.EXPECT().Status().Return(nats.CONNECTED)

	health := cm.Health()
	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["server"])

	mockConn.EXPECT().Status().Return(nats.CLOSED)

	health = cm.Health()
	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["server"])

	cm.conn = nil
	health = cm.Health()
	assert.Equal(t, datasource.StatusDown, health.Status)
}

func TestConnectionManager_JetStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	cm := &ConnectionManager{
		jStream: mockJS,
	}

	js, err := cm.jetStream()
	require.NoError(t, err)
	assert.Equal(t, mockJS, js)
}

func TestConnectionManager_JetStream_Nil(t *testing.T) {
	cm := &ConnectionManager{
		jStream: nil,
	}

	js, err := cm.jetStream()
	require.Error(t, err)
	assert.Nil(t, js)
	assert.EqualError(t, err, "jStream is not configured")
}

func TestNatsConnWrapper_Status(t *testing.T) {
	mockConn := &nats.Conn{}
	wrapper := &natsConnWrapper{mockConn}

	assert.Equal(t, mockConn.Status(), wrapper.Status())
}

func TestNatsConnWrapper_Close(t *testing.T) {
	// Start a NATS server
	ns, url := startNATSServer(t)
	defer ns.Shutdown()

	// Create a real NATS connection
	nc, err := nats.Connect(url)
	require.NoError(t, err, "Failed to connect to NATS")

	// Create the wrapper with the real connection
	wrapper := &natsConnWrapper{conn: nc}

	// Check initial status
	assert.Equal(t, nats.CONNECTED, wrapper.Status(), "Initial status should be CONNECTED")

	// Close the connection
	wrapper.Close()

	// Check final status
	assert.Equal(t, nats.CLOSED, wrapper.Status(), "Final status should be CLOSED")
}

// startNATSServer starts a NATS server and returns the server instance and the client URL.
func startNATSServer(t *testing.T) (s *server.Server, u string) {
	t.Helper()

	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // Random available port
	}

	ns, err := server.NewServer(opts)
	require.NoError(t, err, "Failed to create NATS server")

	go ns.Start()

	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("NATS server not ready for connections")
	}

	u = ns.ClientURL()

	return ns, u
}

func TestNatsConnWrapper_NatsConn(t *testing.T) {
	mockConn := &nats.Conn{}
	wrapper := &natsConnWrapper{mockConn}

	assert.Equal(t, mockConn, wrapper.NATSConn())
}

func TestNewConnectionManager_Defaults(t *testing.T) {
	cm := NewConnectionManager(&Config{Server: NATSServer}, logging.NewMockLogger(logging.DEBUG), nil, nil)

	assert.Equal(t, &defaultConnector{}, cm.natsConnector, "a nil connector must fall back to the default one")
	assert.Equal(t, &DefaultJetStreamCreator{}, cm.jetStreamCreator, "a nil creator must fall back to the default one")
}

func TestNewConnectionManager_NilLoggerPanics(t *testing.T) {
	assert.PanicsWithValue(t, "logger is required", func() {
		NewConnectionManager(&Config{Server: NATSServer}, nil, nil, nil)
	})
}

func TestConnectionManager_Connect_Scenarios(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *Config
		setupMocks func(connector *MockNATSConnector, creator *MockJetStreamCreator, conn *MockConnInterface, js *MockJetStream)
		expErr     error
		expConn    bool
	}{
		{
			name: "connector fails",
			cfg:  &Config{Server: NATSServer},
			setupMocks: func(connector *MockNATSConnector, _ *MockJetStreamCreator, _ *MockConnInterface, _ *MockJetStream) {
				connector.EXPECT().Connect(NATSServer, gomock.Any()).Return(nil, errConnectionError)
			},
			expErr:  errConnectionError,
			expConn: false,
		},
		{
			name: "jetstream creation fails and closes the connection",
			cfg:  &Config{Server: NATSServer},
			setupMocks: func(connector *MockNATSConnector, creator *MockJetStreamCreator, conn *MockConnInterface, _ *MockJetStream) {
				connector.EXPECT().Connect(NATSServer, gomock.Any()).Return(conn, nil)
				creator.EXPECT().New(conn).Return(nil, errJetStreamCreationFailed)
				conn.EXPECT().Close()
			},
			expErr:  errJetStreamCreationFailed,
			expConn: false,
		},
		{
			// The name option plus the credentials option: exactly two options reach the connector.
			name: "credentials file adds a connect option",
			cfg:  &Config{Server: NATSServer, CredsFile: "/path/to/creds"},
			setupMocks: func(connector *MockNATSConnector, creator *MockJetStreamCreator, conn *MockConnInterface, js *MockJetStream) {
				connector.EXPECT().Connect(NATSServer, gomock.Any(), gomock.Any()).Return(conn, nil)
				creator.EXPECT().New(conn).Return(js, nil)
			},
			expErr:  nil,
			expConn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			connector := NewMockNATSConnector(ctrl)
			creator := NewMockJetStreamCreator(ctrl)
			conn := NewMockConnInterface(ctrl)
			js := NewMockJetStream(ctrl)

			tt.setupMocks(connector, creator, conn, js)

			cm := NewConnectionManager(tt.cfg, logging.NewMockLogger(logging.DEBUG), connector, creator)

			err := cm.Connect()

			require.ErrorIs(t, err, tt.expErr)
			assert.Equal(t, tt.expConn, cm.conn != nil)
		})
	}
}

func TestConnectionManager_Publish_Errors(t *testing.T) {
	subject := "test.subject"
	message := []byte("test message")

	tests := []struct {
		name       string
		subject    string
		manager    func(conn *MockConnInterface, js *MockJetStream) *ConnectionManager
		setupMocks func(metrics *MockMetrics, conn *MockConnInterface, js *MockJetStream)
		expErr     error
	}{
		{
			name:    "not connected",
			subject: subject,
			manager: func(*MockConnInterface, *MockJetStream) *ConnectionManager { return &ConnectionManager{} },
			setupMocks: func(metrics *MockMetrics, _ *MockConnInterface, _ *MockJetStream) {
				metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", subject)
			},
			expErr: errClientNotConnected,
		},
		{
			name:    "connection not in connected state",
			subject: subject,
			manager: func(conn *MockConnInterface, _ *MockJetStream) *ConnectionManager {
				return &ConnectionManager{conn: conn}
			},
			setupMocks: func(metrics *MockMetrics, conn *MockConnInterface, _ *MockJetStream) {
				metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", subject)
				conn.EXPECT().Status().Return(nats.RECONNECTING)
			},
			expErr: errClientNotConnected,
		},
		{
			name:    "jetstream not configured",
			subject: subject,
			manager: func(conn *MockConnInterface, _ *MockJetStream) *ConnectionManager {
				return &ConnectionManager{conn: conn}
			},
			setupMocks: func(metrics *MockMetrics, conn *MockConnInterface, _ *MockJetStream) {
				metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", subject)
				conn.EXPECT().Status().Return(nats.CONNECTED)
			},
			expErr: errJetStreamNotConfigured,
		},
		{
			name:    "publish fails",
			subject: subject,
			manager: func(conn *MockConnInterface, js *MockJetStream) *ConnectionManager {
				return &ConnectionManager{conn: conn, jStream: js}
			},
			setupMocks: func(metrics *MockMetrics, conn *MockConnInterface, js *MockJetStream) {
				metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", subject)
				conn.EXPECT().Status().Return(nats.CONNECTED)
				js.EXPECT().PublishMsg(gomock.Any(), natsMsgMatcher{subject: subject, data: message}).Return(nil, errPublishError)
			},
			expErr: errPublishError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			metrics := NewMockMetrics(ctrl)
			conn := NewMockConnInterface(ctrl)
			js := NewMockJetStream(ctrl)

			tt.setupMocks(metrics, conn, js)

			cm := tt.manager(conn, js)
			cm.logger = logging.NewMockLogger(logging.DEBUG)

			err := cm.Publish(t.Context(), tt.subject, message, metrics)

			require.ErrorIs(t, err, tt.expErr)
		})
	}
}
