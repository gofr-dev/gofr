package nats

import (
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
		mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", subject),
		mockConn.EXPECT().Status().Return(nats.CONNECTED),
		mockJS.EXPECT().Publish(ctx, subject, message).Return(&jetstream.PubAck{}, nil),
		mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_publish_success_count", "subject", subject),
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
