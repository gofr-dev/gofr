package nats

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestValidateConfigs(*testing.T) {
	// This test remains unchanged
}

func TestNATSClient_Publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)

	conf := &Config{
		Server: NATSServer,
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &Client{
		connManager: mockConnManager,
		Config:      conf,
		logger:      mockLogger,
		metrics:     mockMetrics,
	}

	ctx := context.Background()
	subject := "test-subject"
	message := []byte("test-message")

	// Set up expected calls
	mockConnManager.EXPECT().
		Publish(ctx, subject, message, mockMetrics).
		Return(nil)

	// Call Publish
	err := client.Publish(ctx, subject, message)
	require.NoError(t, err)
}

func TestNATSClient_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)

	config := &Config{
		Server: NATSServer,
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &Client{
		connManager: mockConnManager,
		metrics:     mockMetrics,
		Config:      config,
		logger:      logging.NewMockLogger(logging.DEBUG),
	}

	ctx := context.TODO()
	subject := "test"
	message := []byte("test-message")

	expectedErr := errPublishError
	mockConnManager.EXPECT().
		Publish(ctx, subject, message, mockMetrics).
		Return(expectedErr)

	err := client.Publish(ctx, subject, message)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_SubscribeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubManager := NewMockSubscriptionManagerInterface(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockJetStream := NewMockJetStream(ctrl)

	client := &Client{
		connManager: mockConnManager,
		subManager:  mockSubManager,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
		metrics: mockMetrics,
		logger:  logging.NewMockLogger(logging.DEBUG),
	}

	ctx := context.Background()
	expectedMsg := &pubsub.Message{
		Topic: "test-subject",
		Value: []byte("test message"),
	}

	mockConnManager.EXPECT().JetStream().Return(mockJetStream)
	mockSubManager.EXPECT().
		Subscribe(ctx, "test-subject", mockJetStream, client.Config, client.logger, client.metrics).
		Return(expectedMsg, nil)

	msg, err := client.Subscribe(ctx, "test-subject")

	require.NoError(t, err)
	assert.Equal(t, expectedMsg, msg)
}

func TestNATSClient_SubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubManager := NewMockSubscriptionManagerInterface(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockJetStream := NewMockJetStream(ctrl)

	client := &Client{
		connManager: mockConnManager,
		subManager:  mockSubManager,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer: "test-consumer",
		},
		metrics: mockMetrics,
		logger:  logging.NewMockLogger(logging.DEBUG),
	}

	ctx := context.Background()

	expectedErr := errFailedToCreateConsumer

	mockConnManager.EXPECT().JetStream().Return(mockJetStream)

	mockSubManager.EXPECT().
		Subscribe(ctx, "test-subject", mockJetStream, client.Config, client.logger, client.metrics).
		Return(nil, expectedErr)

	msg, err := client.Subscribe(ctx, "test-subject")

	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubManager := NewMockSubscriptionManagerInterface(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)

	client := &Client{
		connManager: mockConnManager,
		subManager:  mockSubManager,
		logger:      logging.NewMockLogger(logging.DEBUG),
		Config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
		},
	}

	ctx := context.Background()

	mockSubManager.EXPECT().Close()
	mockConnManager.EXPECT().Close(ctx)

	err := client.Close(ctx)
	require.NoError(t, err)
}

func TestNew(t *testing.T) {
	config := &Config{
		Server: NATSServer,
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer:  "test-consumer",
		MaxWait:   5 * time.Second,
		BatchSize: 100,
	}

	natsClient := New(config)
	assert.NotNil(t, natsClient)

	// Check PubSubWrapper struct
	assert.NotNil(t, natsClient)
	assert.NotNil(t, natsClient.Client)

	// Check Client struct
	assert.Equal(t, config, natsClient.Client.Config)
	assert.NotNil(t, natsClient.Client.subManager)

	// Check methods
	assert.NotNil(t, natsClient.DeleteTopic)
	assert.NotNil(t, natsClient.CreateTopic)
	assert.NotNil(t, natsClient.Subscribe)
	assert.NotNil(t, natsClient.Publish)
	assert.NotNil(t, natsClient.Close)

	// Check new methods
	assert.NotNil(t, natsClient.UseLogger)
	assert.NotNil(t, natsClient.UseMetrics)
	assert.NotNil(t, natsClient.UseTracer)
	assert.NotNil(t, natsClient.Connect)

	// Check that Connect hasn't been called yet
	assert.Nil(t, natsClient.Client.connManager)
}

func TestNATSClient_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := context.Background()

	mockStreamManager.EXPECT().DeleteStream(ctx, "test-topic").Return(nil)

	err := client.DeleteTopic(ctx, "test-topic")
	require.NoError(t, err)
}

func TestNATSClient_DeleteTopic_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := context.Background()

	expectedErr := errFailedToDeleteStream
	mockStreamManager.EXPECT().DeleteStream(ctx, "test-topic").Return(expectedErr)

	err := client.DeleteTopic(ctx, "test-topic")
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := context.Background()

	mockStreamManager.EXPECT().
		CreateStream(ctx, StreamConfig{
			Stream:   "test-topic",
			Subjects: []string{"test-topic"},
		}).
		Return(nil)

	err := client.CreateTopic(ctx, "test-topic")
	require.NoError(t, err)
}

func TestClient_Connect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockNATSConnector := NewMockNATSConnector(ctrl)
	mockJSCreator := NewMockJetStreamCreator(ctrl)
	mockConn := NewMockConnInterface(ctrl)
	mockJS := NewMockJetStream(ctrl)

	// Set up client with mocks
	client := &Client{
		Config: &Config{
			Server: "nats://localhost:4222",
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer:  "test-consumer",
			BatchSize: 100,
		},
		logger:           mockLogger,
		natsConnector:    mockNATSConnector,
		jetStreamCreator: mockJSCreator,
	}

	// Set expectations
	mockNATSConnector.EXPECT().
		Connect("nats://localhost:4222", gomock.Any()).
		Return(mockConn, nil).
		Times(2)

	mockJSCreator.EXPECT().
		New(mockConn).
		Return(mockJS, nil).
		Times(2)

	// Call the Connect method on the client
	err := client.Connect()
	require.NoError(t, err)

	// Assert that the connection manager was set
	assert.NotNil(t, client.connManager)

	// Assert that the stream manager and subscription manager were created
	assert.NotNil(t, client.streamManager)
	assert.NotNil(t, client.subManager)

	// Check for log output
	out := testutil.StdoutOutputForFunc(func() {
		client.logger = logging.NewMockLogger(logging.DEBUG)
		err := client.Connect()
		require.NoError(t, err)
	})

	// Assert that the expected log message is produced
	assert.Contains(t, out, "connected to NATS server 'nats://localhost:4222'")
}

func TestClient_ValidateAndPrepare(t *testing.T) {
	client := &Client{
		Config: &Config{},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	err := client.validateAndPrepare()
	require.Error(t, err)

	client.Config = &Config{
		Server: "nats://localhost:4222",
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	err = client.validateAndPrepare()
	assert.NoError(t, err)
}

func TestClient_LogSuccessfulConnection(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		Config: &Config{Server: "nats://localhost:4222"},
		logger: mockLogger,
	}

	logs := testutil.StdoutOutputForFunc(func() {
		client.logger = logging.NewMockLogger(logging.DEBUG)
		client.logSuccessfulConnection()
	})

	assert.Contains(t, logs, "connected to NATS server 'nats://localhost:4222'")
}

func TestClient_UseLogger(t *testing.T) {
	client := &Client{}
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	client.UseLogger(mockLogger)
	assert.Equal(t, mockLogger, client.logger)

	client.UseLogger("not a logger")
	assert.Equal(t, mockLogger, client.logger) // Should not change
}

func TestClient_UseTracer(t *testing.T) {
	client := &Client{}
	mockTracer := noop.NewTracerProvider().Tracer("test")

	client.UseTracer(mockTracer)
	assert.Equal(t, mockTracer, client.tracer)

	client.UseTracer("not a tracer")
	assert.Equal(t, mockTracer, client.tracer) // Should not change
}

func TestClient_UseMetrics(t *testing.T) {
	client := &Client{}
	mockMetrics := NewMockMetrics(gomock.NewController(t))

	client.UseMetrics(mockMetrics)
	assert.Equal(t, mockMetrics, client.metrics)

	client.UseMetrics("not metrics")
	assert.Equal(t, mockMetrics, client.metrics) // Should not change
}

func TestClient_SubscribeWithHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockJetStream := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMessageBatch := NewMockMessageBatch(ctrl)

	client := &Client{
		connManager: mockConnManager,
		Config: &Config{
			Consumer: "test-consumer",
			Stream: StreamConfig{
				Stream:     "test-stream",
				MaxDeliver: 3,
			},
			MaxWait: time.Second,
		},
		logger: mockLogger,
	}

	mockConnManager.EXPECT().JetStream().Return(mockJetStream).AnyTimes()
	mockJetStream.EXPECT().CreateOrUpdateConsumer(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockConsumer, nil)
	mockConsumer.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(mockMessageBatch, nil).AnyTimes()
	mockMessageBatch.EXPECT().Messages().Return(make(chan jetstream.Msg)).AnyTimes()
	mockMessageBatch.EXPECT().Error().Return(nil).AnyTimes()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(context.Context, jetstream.Msg) error {
		return nil
	}

	err := client.SubscribeWithHandler(ctx, "test-subject", handler)
	require.NoError(t, err)

	// Wait a bit to allow the goroutine to start
	time.Sleep(100 * time.Millisecond)

	// Test error case
	mockJetStream.EXPECT().CreateOrUpdateConsumer(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errConsumerCreationError)

	err = client.SubscribeWithHandler(ctx, "test-subject", handler)
	require.Error(t, err)
}

func TestClient_CreateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
	}

	cfg := StreamConfig{
		Stream:   "test-stream",
		Subjects: []string{"test-subject"},
	}

	mockStreamManager.EXPECT().CreateStream(gomock.Any(), cfg).Return(nil)

	err := client.CreateStream(context.Background(), cfg)
	require.NoError(t, err)
}

func TestClient_DeleteStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
	}

	mockStreamManager.EXPECT().DeleteStream(gomock.Any(), "test-stream").Return(nil)

	err := client.DeleteStream(context.Background(), "test-stream")
	require.NoError(t, err)
}

func TestClient_CreateOrUpdateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	mockStream := NewMockStream(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
	}

	cfg := jetstream.StreamConfig{
		Name:     "test-stream",
		Subjects: []string{"test-subject"},
	}

	mockStreamManager.EXPECT().CreateOrUpdateStream(gomock.Any(), &cfg).Return(mockStream, nil)

	stream, err := client.CreateOrUpdateStream(context.Background(), &cfg)
	require.NoError(t, err)
	assert.Equal(t, mockStream, stream)
}
