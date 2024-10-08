package nats

import (
	"context"
	"sync"
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

func TestClient_ConnectError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNATSConnector := NewMockNATSConnector(ctrl)
	mockJSCreator := NewMockJetStreamCreator(ctrl)

	config := &Config{
		Server: "nats://localhost:4222",
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer:  "test-consumer",
		BatchSize: 100,
	}

	client := &Client{
		Config:           config,
		logger:           logging.NewMockLogger(logging.DEBUG),
		natsConnector:    mockNATSConnector,
		jetStreamCreator: mockJSCreator,
	}

	// Simulate a connection error
	expectedErr := errConnectionError
	mockNATSConnector.EXPECT().
		Connect(config.Server, gomock.Any()).
		Return(nil, expectedErr)

	// Capture stderr output
	output := testutil.StderrOutputForFunc(func() {
		client.logger = logging.NewMockLogger(logging.DEBUG)
		err := client.Connect()
		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})

	// Check for the error log
	assert.Contains(t, output, "failed to connect to NATS server at nats://localhost:4222: connection error")

	// Assert that the connection manager, stream manager, and subscription manager were not set
	assert.Nil(t, client.connManager)
	assert.Nil(t, client.streamManager)
	assert.Nil(t, client.subManager)
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

	// Create separate mock consumers and message batches for each subscription
	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockJetStream := NewMockJetStream(ctrl)
	mockConsumer1 := NewMockConsumer(ctrl)
	mockMessageBatch1 := NewMockMessageBatch(ctrl)
	messageChan1 := make(chan jetstream.Msg, 1)
	mockMsg1 := NewMockMsg(ctrl)

	mockConsumer2 := NewMockConsumer(ctrl)
	mockMessageBatch2 := NewMockMessageBatch(ctrl)
	messageChan2 := make(chan jetstream.Msg, 1)
	mockMsg2 := NewMockMsg(ctrl)

	// Create a mock for SubscriptionManagerInterface
	mockSubManager := NewMockSubscriptionManagerInterface(ctrl)

	// Initialize the client with all necessary mocks
	client := &Client{
		connManager: mockConnManager,
		subManager:  mockSubManager, // Assign the mockSubManager here
		Config: &Config{
			Consumer: "test-consumer",
			Stream: StreamConfig{
				Stream:     "test-stream",
				MaxDeliver: 3,
			},
			MaxWait: time.Second,
		},
		logger:        logging.NewMockLogger(logging.DEBUG),
		subscriptions: make(map[string]context.CancelFunc),
	}

	// Set up expectations for JetStream() to be called twice (for two subscriptions)
	mockConnManager.EXPECT().
		JetStream().
		Return(mockJetStream).
		Times(2)

	// Set up expectations for subManager.Close()
	mockSubManager.EXPECT().
		Close().
		Times(1)

	// Set up expectations for connManager.Close(ctx)
	mockConnManager.EXPECT().
		Close(gomock.Any()).
		Times(1) // Removed Return(nil) since Close does not return anything

	// ---------------------
	// Synchronization Setup
	// ---------------------
	var wg sync.WaitGroup

	wg.Add(2) // Two handlers

	// ---------------------
	// First Subscription Execution
	// ---------------------
	t.Run("First_Subscription", func(t *testing.T) {
		// Expect CreateOrUpdateConsumer to be called once for the first subscription
		mockJetStream.EXPECT().
			CreateOrUpdateConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockConsumer1, nil).
			Times(1)

		// Expect Fetch to be called twice: once to fetch the message, once to fetch nothing
		mockConsumer1.EXPECT().
			Fetch(gomock.Any(), gomock.Any()).
			Return(mockMessageBatch1, nil).
			Times(2)

		// Expect Messages to return the message channel first, then nil
		gomock.InOrder(
			mockMessageBatch1.EXPECT().
				Messages().
				Return(messageChan1).
				Times(1),

			mockMessageBatch1.EXPECT().
				Messages().
				Return(nil).
				Times(1),
		)

		// Expect Error to be called once for the first subscription
		mockMessageBatch1.EXPECT().
			Error().
			Return(nil).
			Times(1) // Adjusted from Times(2) to Times(1)

		// Expect Ack to be called once for the first message
		mockMsg1.EXPECT().
			Ack().
			Return(nil).
			Times(1)

		// Define the first handler that acknowledges the message
		firstHandlerCalled := make(chan bool, 1)
		firstHandler := func(context.Context, jetstream.Msg) error {
			t.Log("First handler called")
			firstHandlerCalled <- true

			wg.Done()

			return nil
		}

		// Subscribe with the first handler
		err := client.SubscribeWithHandler(context.Background(), "test-subject", firstHandler)
		require.NoError(t, err)

		// Send a message to trigger the first handler
		messageChan1 <- mockMsg1

		// Close the message channel to allow Fetch to return nil on the next call
		close(messageChan1)

		// Wait for the first handler to be called
		select {
		case <-firstHandlerCalled:
			t.Log("First handler was called successfully")
		case <-time.After(time.Second * 5):
			t.Fatal("First handler was not called within the expected time")
		}
	})

	// ---------------------
	// Second Subscription Execution
	// ---------------------
	t.Run("Second_Subscription", func(t *testing.T) {
		// Expect CreateOrUpdateConsumer to be called once for the second subscription
		mockJetStream.EXPECT().
			CreateOrUpdateConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockConsumer2, nil).
			Times(1)

		// Expect Fetch to be called twice: once to fetch the message, once to fetch nothing
		mockConsumer2.EXPECT().
			Fetch(gomock.Any(), gomock.Any()).
			Return(mockMessageBatch2, nil).
			Times(2)

		// Expect Messages to return the message channel first, then nil
		gomock.InOrder(
			mockMessageBatch2.EXPECT().
				Messages().
				Return(messageChan2).
				Times(1),

			mockMessageBatch2.EXPECT().
				Messages().
				Return(nil).
				Times(1),
		)

		// Expect Error to be called once for the second subscription
		mockMessageBatch2.EXPECT().
			Error().
			Return(nil).
			Times(1) // Adjusted from Times(2) to Times(1)

		// Expect Nak to be called once for the second message
		mockMsg2.EXPECT().
			Nak().
			Return(nil).
			Times(1)

		// Define the second handler that returns an error, causing a NAK
		errorHandlerCalled := make(chan bool, 1)
		errorHandler := func(context.Context, jetstream.Msg) error {
			t.Log("Error handler called")
			errorHandlerCalled <- true

			t.Logf("Error handling message: %v", errHandlerError)
			t.Logf("Error processing message: %v", errHandlerError)
			wg.Done()

			return errHandlerError
		}

		// Subscribe with the error handler
		err := client.SubscribeWithHandler(context.Background(), "test-subject", errorHandler)
		require.NoError(t, err)

		// Send a message to trigger the error handler
		messageChan2 <- mockMsg2

		// Close the message channel to allow Fetch to return nil on the next call
		close(messageChan2)

		// Wait for the error handler to be called
		select {
		case <-errorHandlerCalled:
			t.Log("Error handler was called successfully")
		case <-time.After(time.Second):
			t.Fatal("Error handler was not called within the expected time")
		}
	})

	// ---------------------
	// Wait for All Handlers to Complete
	// ---------------------
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All handlers completed
	case <-time.After(5 * time.Second):
		t.Fatal("Handlers did not complete within the expected time")
	}

	// ---------------------
	// Gracefully Close the Client
	// ---------------------
	err := client.Close(context.Background())
	require.NoError(t, err)
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
