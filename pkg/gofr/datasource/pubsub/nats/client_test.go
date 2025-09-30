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

	ctx := t.Context()
	subject := "test-subject"
	message := []byte("test-message")

	// Set up expected calls
	gomock.InOrder(
		mockConnManager.EXPECT().IsConnected().Return(true),
		mockConnManager.EXPECT().Publish(ctx, subject, message, mockMetrics).Return(nil),
	)

	// Call Publish
	err := client.Publish(ctx, subject, message)
	require.NoError(t, err)
}

func TestNATSClient_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)

	ctx := t.Context()
	subject := "test"
	message := []byte("test-message")

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

	tests := []struct {
		name     string
		client   *Client
		mockCall func(mockConn *MockConnectionManagerInterface)
		expErr   error
	}{
		{name: "nil client", client: nil, mockCall: func(_ *MockConnectionManagerInterface) {}, expErr: errClientNotConnected},
		{name: "nil connManager", client: &Client{connManager: nil}, mockCall: func(_ *MockConnectionManagerInterface) {},
			expErr: errClientNotConnected},
		{name: "not connected to NATS server", client: &Client{connManager: mockConnManager},
			mockCall: func(mockConn *MockConnectionManagerInterface) {
				mockConn.EXPECT().IsConnected().Return(false)
			}, expErr: errClientNotConnected},
		{name: "err in publishing", client: client, mockCall: func(mockConn *MockConnectionManagerInterface) {
			mockConn.EXPECT().IsConnected().Return(true)
			mockConn.EXPECT().Publish(gomock.Any(), subject, message, mockMetrics).Return(errPublishError)
		}, expErr: errPublishError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockCall(mockConnManager)

			err := tt.client.Publish(ctx, subject, message)

			require.Error(t, err)
			assert.Equal(t, tt.expErr, err)
		})
	}
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
			Consumer: "test-consumer",
			MaxWait:  time.Second,
		},
		metrics: mockMetrics,
		logger:  logging.NewMockLogger(logging.DEBUG),
	}

	ctx := t.Context()
	expectedMsg := &pubsub.Message{
		Topic: "test-subject",
		Value: []byte("test message"),
	}

	mockConnManager.EXPECT().IsConnected().Return(true)
	mockConnManager.EXPECT().JetStream().Return(mockJetStream, nil).AnyTimes()

	mockSubManager.EXPECT().
		Subscribe(ctx, "test-subject", mockJetStream, gomock.Any(), gomock.Any(), gomock.Any()).
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

	ctx := t.Context()
	expectedErr := errSubscriptionError

	mockConnManager.EXPECT().IsConnected().Return(true)
	mockConnManager.EXPECT().JetStream().Return(mockJetStream, nil).AnyTimes()

	mockSubManager.EXPECT().
		Subscribe(ctx, "test-subject", mockJetStream, gomock.Any(), gomock.Any(), gomock.Any()).
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

	ctx := t.Context()

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
		Consumer: "test-consumer",
		MaxWait:  5 * time.Second,
	}

	mockLogger := logging.NewMockLogger(logging.DEBUG)

	natsClient := New(config, mockLogger)
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
	mockConnManager := NewMockConnectionManagerInterface(ctrl)

	client := &Client{
		streamManager: mockStreamManager,
		connManager:   mockConnManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := t.Context()
	gomock.InOrder(
		mockConnManager.EXPECT().IsConnected().Return(true),
		mockStreamManager.EXPECT().DeleteStream(ctx, "test-topic").Return(nil),
	)

	err := client.DeleteTopic(ctx, "test-topic")
	require.NoError(t, err)
}

func TestNATSClient_DeleteTopic_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
		connManager:   mockConnManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := t.Context()

	expectedErr := errFailedToDeleteStream

	gomock.InOrder(
		mockConnManager.EXPECT().IsConnected().Return(true),
		mockStreamManager.EXPECT().DeleteStream(ctx, "test-topic").Return(expectedErr),
	)

	err := client.DeleteTopic(ctx, "test-topic")
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
		connManager:   mockConnManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := t.Context()

	mockConnManager.EXPECT().IsConnected().Return(true)
	mockStreamManager.EXPECT().
		CreateStream(ctx, &StreamConfig{
			Stream:   "test-topic",
			Subjects: []string{"test-topic"},
		}).Return(nil)

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
			Consumer: "test-consumer",
		},
		logger:           mockLogger,
		natsConnector:    mockNATSConnector,
		jetStreamCreator: mockJSCreator,
	}

	// Set expectations
	mockNATSConnector.EXPECT().Connect("nats://localhost:4222", gomock.Any()).
		Return(mockConn, nil).Times(2)

	mockJSCreator.EXPECT().New(mockConn).Return(mockJS, nil).Times(2)

	_ = client.Connect()

	time.Sleep(100 * time.Millisecond)

	// Assert that the connection manager was set
	assert.NotNil(t, client.connManager)

	// Check for log output
	out := testutil.StdoutOutputForFunc(func() {
		client.logger = logging.NewMockLogger(logging.DEBUG)
		_ = client.Connect()

		time.Sleep(100 * time.Millisecond)
	})

	// Assert that the expected log message is produced
	assert.Contains(t, out, "connecting to NATS server at nats://localhost:4222\n"+
		"Successfully connected to NATS server at nats://localhost:4222\n")
}

func TestClient_ValidateAndPrepare(t *testing.T) {
	client := &Client{
		Config: &Config{},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	err := validateAndPrepare(client.Config, client.logger)
	require.Error(t, err)

	client.Config = &Config{
		Server: "nats://localhost:4222",
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	err = validateAndPrepare(client.Config, client.logger)
	assert.NoError(t, err)
}

func TestClient_ValidateAndPrepareError(t *testing.T) {
	client := &Client{
		Config: &Config{},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	err := validateAndPrepare(client.Config, client.logger)
	require.Error(t, err)

	client.Config = &Config{
		Server: "",
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	err = validateAndPrepare(client.Config, client.logger)
	assert.Error(t, err)
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

	client, mocks := setupClientAndMocks(t, ctrl)

	var wg sync.WaitGroup

	wg.Add(2) // Two handlers

	t.Run("First_Subscription", func(t *testing.T) {
		testFirstSubscription(t, client, mocks, &wg)
	})

	t.Run("Second_Subscription", func(t *testing.T) {
		testSecondSubscription(t, client, mocks, &wg)
	})

	waitForHandlersToComplete(t, &wg)

	err := client.Close(t.Context())
	require.NoError(t, err)
}

func setupClientAndMocks(t *testing.T, ctrl *gomock.Controller) (*Client, *testMocks) {
	t.Helper()

	mocks := &testMocks{
		connManager:   NewMockConnectionManagerInterface(ctrl),
		jetStream:     NewMockJetStream(ctrl),
		consumer1:     NewMockConsumer(ctrl),
		messageBatch1: NewMockMessageBatch(ctrl),
		msg1:          NewMockMsg(ctrl),
		consumer2:     NewMockConsumer(ctrl),
		messageBatch2: NewMockMessageBatch(ctrl),
		msg2:          NewMockMsg(ctrl),
		subManager:    NewMockSubscriptionManagerInterface(ctrl),
		messageChan1:  make(chan jetstream.Msg, 1),
		messageChan2:  make(chan jetstream.Msg, 1),
	}

	client := &Client{
		connManager:   mocks.connManager,
		subManager:    mocks.subManager,
		Config:        createTestConfig(),
		logger:        logging.NewMockLogger(logging.DEBUG),
		subscriptions: make(map[string]context.CancelFunc),
	}

	setupCommonExpectations(mocks)

	return client, mocks
}

func createTestConfig() *Config {
	return &Config{
		Consumer: "test-consumer",
		Stream: StreamConfig{
			Stream:     "test-stream",
			MaxDeliver: 3,
		},
		MaxWait: time.Second,
	}
}

func setupCommonExpectations(mocks *testMocks) {
	mocks.connManager.EXPECT().JetStream().Return(mocks.jetStream, nil).AnyTimes()
	mocks.subManager.EXPECT().Close().Times(1)
	mocks.connManager.EXPECT().Close(gomock.Any()).AnyTimes()
}

func testFirstSubscription(t *testing.T, client *Client, mocks *testMocks, wg *sync.WaitGroup) {
	t.Helper()

	setupFirstSubscriptionExpectations(mocks)

	firstHandlerCalled := make(chan bool, 1)
	firstHandler := createFirstHandler(t, firstHandlerCalled, wg)

	err := client.SubscribeWithHandler(t.Context(), "test-subject", firstHandler)
	require.NoError(t, err)

	mocks.messageChan1 <- mocks.msg1

	close(mocks.messageChan1)

	assertHandlerCalled(t, firstHandlerCalled, "First handler")
}

func testSecondSubscription(t *testing.T, client *Client, mocks *testMocks, wg *sync.WaitGroup) {
	t.Helper()

	setupSecondSubscriptionExpectations(mocks)

	errorHandlerCalled := make(chan bool, 1)
	errorHandler := createErrorHandler(t, errorHandlerCalled, wg)

	err := client.SubscribeWithHandler(t.Context(), "test-subject", errorHandler)
	require.NoError(t, err)

	mocks.messageChan2 <- mocks.msg2

	close(mocks.messageChan2)

	assertHandlerCalled(t, errorHandlerCalled, "Error handler")
}

func setupFirstSubscriptionExpectations(mocks *testMocks) {
	mocks.jetStream.EXPECT().
		CreateOrUpdateConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(mocks.consumer1, nil).
		Times(1)

	mocks.consumer1.EXPECT().
		Fetch(gomock.Any(), gomock.Any()).
		Return(mocks.messageBatch1, nil).
		Times(2)

	gomock.InOrder(
		mocks.messageBatch1.EXPECT().
			Messages().
			Return(mocks.messageChan1).
			Times(1),

		mocks.messageBatch1.EXPECT().
			Messages().
			Return(nil).
			Times(1),
	)

	mocks.messageBatch1.EXPECT().
		Error().
		Return(nil).
		Times(1)

	mocks.msg1.EXPECT().
		Ack().
		Return(nil).
		Times(1)
}

func setupSecondSubscriptionExpectations(mocks *testMocks) {
	mocks.jetStream.EXPECT().
		CreateOrUpdateConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(mocks.consumer2, nil).
		Times(1)

	mocks.consumer2.EXPECT().
		Fetch(gomock.Any(), gomock.Any()).
		Return(mocks.messageBatch2, nil).
		Times(2)

	gomock.InOrder(
		mocks.messageBatch2.EXPECT().
			Messages().
			Return(mocks.messageChan2).
			Times(1),

		mocks.messageBatch2.EXPECT().
			Messages().
			Return(nil).
			Times(1),
	)

	mocks.messageBatch2.EXPECT().
		Error().
		Return(nil).
		Times(1)

	mocks.msg2.EXPECT().
		Nak().
		Return(nil).
		Times(1)
}

func createFirstHandler(t *testing.T, handlerCalled chan<- bool, wg *sync.WaitGroup) func(context.Context, jetstream.Msg) error {
	t.Helper()

	return func(context.Context, jetstream.Msg) error {
		t.Log("First handler called")

		handlerCalled <- true

		wg.Done()

		return nil
	}
}

func createErrorHandler(t *testing.T, handlerCalled chan<- bool, wg *sync.WaitGroup) func(context.Context, jetstream.Msg) error {
	t.Helper()

	return func(context.Context, jetstream.Msg) error {
		t.Log("Error handler called")

		handlerCalled <- true

		t.Logf("Error handling message: %v", errHandlerError)
		t.Logf("Error processing message: %v", errHandlerError)

		wg.Done()

		return errHandlerError
	}
}

func assertHandlerCalled(t *testing.T, handlerCalled <-chan bool, handlerName string) {
	t.Helper()

	select {
	case <-handlerCalled:
		t.Logf("%s was called successfully", handlerName)
	case <-time.After(time.Second * 5):
		t.Fatalf("%s was not called within the expected time", handlerName)
	}
}

func waitForHandlersToComplete(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()

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
}

type testMocks struct {
	connManager   *MockConnectionManagerInterface
	jetStream     *MockJetStream
	consumer1     *MockConsumer
	messageBatch1 *MockMessageBatch
	messageChan1  chan jetstream.Msg
	msg1          *MockMsg
	consumer2     *MockConsumer
	messageBatch2 *MockMessageBatch
	messageChan2  chan jetstream.Msg
	msg2          *MockMsg
	subManager    *MockSubscriptionManagerInterface
}

func TestClient_CreateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	client := &Client{
		connManager:   mockConnManager,
		streamManager: mockStreamManager,
	}

	cfg := StreamConfig{
		Stream:   "test-stream",
		Subjects: []string{"test-subject"},
	}

	mockConnManager.EXPECT().IsConnected().Return(true)
	mockStreamManager.EXPECT().CreateStream(gomock.Any(), &cfg).Return(nil)

	err := client.CreateStream(t.Context(), &cfg)
	require.NoError(t, err)
}

func TestClient_DeleteStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	client := &Client{
		connManager:   mockConnManager,
		streamManager: mockStreamManager,
	}

	mockConnManager.EXPECT().IsConnected().Return(true)
	mockStreamManager.EXPECT().DeleteStream(gomock.Any(), "test-stream").Return(nil)

	err := client.DeleteStream(t.Context(), "test-stream")
	require.NoError(t, err)
}

func TestClient_CreateOrUpdateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockStream := NewMockStream(ctrl)
	client := &Client{
		connManager:   mockConnManager,
		streamManager: mockStreamManager,
	}

	cfg := jetstream.StreamConfig{
		Name:     "test-stream",
		Subjects: []string{"test-subject"},
	}

	mockConnManager.EXPECT().IsConnected().Return(true)
	mockStreamManager.EXPECT().CreateOrUpdateStream(gomock.Any(), &cfg).Return(mockStream, nil)

	stream, err := client.CreateOrUpdateStream(t.Context(), &cfg)
	require.NoError(t, err)
	assert.Equal(t, mockStream, stream)
}

func Test_checkClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnectionManagerInterface(ctrl)

	tests := []struct {
		name     string
		client   *Client
		mockCall *gomock.Call
		expErr   error
	}{
		{name: "nil client", client: nil, expErr: errClientNotConnected},
		{name: "nil connManager", client: &Client{connManager: nil}, expErr: errClientNotConnected},
		{name: "not connected to NATS server", client: &Client{connManager: mockConn},
			mockCall: mockConn.EXPECT().IsConnected().Return(false), expErr: errClientNotConnected},
		{name: "valid client", client: &Client{connManager: mockConn},
			mockCall: mockConn.EXPECT().IsConnected().Return(true), expErr: nil},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkClient(tt.client)

			assert.Equalf(t, tt.expErr, err, "Test[%d] failed - %s", i, tt.name)
		})
	}
}

func TestClient_retryConnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNATSConnector := NewMockNATSConnector(ctrl)
	mockJSCreator := NewMockJetStreamCreator(ctrl)
	mockConn := NewMockConnInterface(ctrl)
	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	subs := make(map[string]context.CancelFunc)
	cfg := Config{Server: "nats://localhost:4222",
		Stream:   StreamConfig{Stream: "test_stream", Subjects: []string{"test_subject"}},
		Consumer: "test_consumer",
	}

	tests := []struct {
		name        string
		setupMocks  func(*Client, *MockNATSConnector, *MockJetStreamCreator, *MockConnInterface, *MockJetStream)
		connSuccess bool
	}{
		{
			name: "successful connection on first attempt",
			setupMocks: func(client *Client, mockNATSConnector *MockNATSConnector, mockJSCreator *MockJetStreamCreator,
				mockConn *MockConnInterface, mockJS *MockJetStream) {
				gomock.InOrder(
					mockNATSConnector.EXPECT().Connect(client.Config.Server, gomock.Any()).
						Return(mockConn, nil).MaxTimes(1),
					mockJSCreator.EXPECT().New(mockConn).Return(mockJS, nil).MaxTimes(1),
				)
			},
			connSuccess: true,
		},
		{
			name: "successful connection after retries",
			setupMocks: func(client *Client, mockNATSConnector *MockNATSConnector, mockJSCreator *MockJetStreamCreator,
				mockConn *MockConnInterface, mockJS *MockJetStream) {
				gomock.InOrder(
					// First attempt fails
					mockNATSConnector.EXPECT().Connect(client.Config.Server, gomock.Any()).
						Return(nil, errConnectionError).MaxTimes(1),
					// Second attempt succeeds
					mockNATSConnector.EXPECT().Connect(client.Config.Server, gomock.Any()).
						Return(mockConn, nil).MaxTimes(1),
					mockJSCreator.EXPECT().New(mockConn).
						Return(mockJS, nil),
				)
			},
		},
		{
			name: "JetStream creation fails after successful connection",
			setupMocks: func(client *Client, mockNATSConnector *MockNATSConnector, mockJSCreator *MockJetStreamCreator,
				mockConn *MockConnInterface, mockJS *MockJetStream) {
				gomock.InOrder(
					// Connection succeeds but JetStream creation fails
					mockNATSConnector.EXPECT().Connect(client.Config.Server, gomock.Any()).Return(mockConn, nil),
					mockJSCreator.EXPECT().New(mockConn).Return(nil, errConnectionError),
					mockConn.EXPECT().Close(),
					// Retry succeeds
					mockNATSConnector.EXPECT().Connect(client.Config.Server, gomock.Any()).Return(mockConn, nil),
					mockJSCreator.EXPECT().New(mockConn).Return(mockJS, nil),
				)
			},
		},
	}

	for _, tt := range tests {
		client := &Client{
			Config:           &cfg,
			logger:           logger,
			natsConnector:    mockNATSConnector,
			jetStreamCreator: mockJSCreator,
			subscriptions:    subs,
		}

		tt.setupMocks(client, mockNATSConnector, mockJSCreator, mockConn, mockJS)
		client.retryConnect()
		time.Sleep(500 * time.Millisecond)

		if tt.connSuccess {
			assert.NotNil(t, client.connManager)
			assert.NotNil(t, client.streamManager)
			assert.NotNil(t, client.subManager)
		}
	}
}

func TestClient_GetJetStreamStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jStream := NewMockJetStream(ctrl)

	tests := []struct {
		name     string
		mockCall *gomock.Call
		want     string
		wantErr  error
	}{
		{name: "status OK", want: jetStreamStatusOK,
			mockCall: jStream.EXPECT().AccountInfo(gomock.Any()).Return(nil, nil)},
		{name: "error in jetstream", want: jetStreamStatusError, wantErr: errJetStream,
			mockCall: jStream.EXPECT().AccountInfo(gomock.Any()).Return(nil, errJetStream)},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			got, err := GetJetStreamStatus(ctx, jStream)

			assert.Equalf(t, tt.wantErr, err, "Test[%d] failed- %s", i, tt.name)
			assert.Equalf(t, tt.want, got, "Test[%d] failed- %s", i, tt.name)
		})
	}
}

func TestClient_establishConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNATSConnector := NewMockNATSConnector(ctrl)
	mockJSCreator := NewMockJetStreamCreator(ctrl)
	mockConn := NewMockConnInterface(ctrl)
	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	cfg := Config{
		Server: "nats://localhost:4222",
		Stream: StreamConfig{
			Stream:   "test_stream",
			Subjects: []string{"test_subject"},
		},
		Consumer: "test_consumer",
	}

	tests := []struct {
		name       string
		client     *Client
		setupMocks func(*Client, *MockNATSConnector, *MockJetStreamCreator, *MockConnInterface, *MockJetStream)
		wantErr    error
	}{
		{
			name: "successful connection",
			setupMocks: func(client *Client, mockNATSConnector *MockNATSConnector, _ *MockJetStreamCreator,
				_ *MockConnInterface, mockJS *MockJetStream) {
				gomock.InOrder(
					mockNATSConnector.EXPECT().Connect(client.Config.Server, gomock.Any()).
						Return(mockConn, nil),
					mockJSCreator.EXPECT().New(mockConn).Return(mockJS, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "connection failure",
			setupMocks: func(client *Client, mockNATSConnector *MockNATSConnector, _ *MockJetStreamCreator,
				_ *MockConnInterface, _ *MockJetStream) {
				mockNATSConnector.EXPECT().Connect(client.Config.Server, gomock.Any()).
					Return(nil, errConnectionError)
			},
			wantErr: errConnectionError,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				Config:           &cfg,
				logger:           logger,
				natsConnector:    mockNATSConnector,
				jetStreamCreator: mockJSCreator,
			}

			tt.setupMocks(client, mockNATSConnector, mockJSCreator, mockConn, mockJS)

			err := client.establishConnection()

			assert.Equalf(t, tt.wantErr, err, "Test[%d] failed - %s", i, tt.name)
		})
	}
}

func TestClient_Query_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockJetStream := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMessageBatch := NewMockMessageBatch(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	client := &Client{
		connManager: mockConnManager,
		logger:      mockLogger,
		Config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
			MaxWait: 5 * time.Second,
		},
	}

	query := "test-subject"
	expectedMessages := []byte("message1\nmessage2")

	// Mock expectations
	mockConnManager.EXPECT().IsConnected().Return(true)
	mockConnManager.EXPECT().JetStream().Return(mockJetStream, nil)
	mockJetStream.EXPECT().CreateOrUpdateConsumer(gomock.Any(),
		"test-stream", gomock.Any()).Return(mockConsumer, nil)
	mockConsumer.EXPECT().CachedInfo().AnyTimes().Return(&jetstream.ConsumerInfo{
		Config: jetstream.ConsumerConfig{FilterSubject: "test-subject"}})
	mockConsumer.EXPECT().Info(gomock.Any()).Return(&jetstream.ConsumerInfo{Name: "test-stream"}, nil)
	mockJetStream.EXPECT().DeleteConsumer(gomock.Any(), "test-stream", gomock.Any()).Return(nil)
	mockConsumer.EXPECT().Fetch(2, gomock.Any()).Return(mockMessageBatch, nil).Times(1)

	// Mock message channel
	messageChannel := make(chan jetstream.Msg, 2)
	messageChannel <- newMockMessage("message1")

	messageChannel <- newMockMessage("message2")

	close(messageChannel)

	mockMessageBatch.EXPECT().Messages().Return(messageChannel)

	// Call the Query method
	result, err := client.Query(t.Context(), query, 2*time.Second, 2)

	// Assertions
	require.NoError(t, err)
	require.Equal(t, expectedMessages, result)
}

// Helper function to create a mock message.
func newMockMessage(data string) jetstream.Msg {
	ctrl := gomock.NewController(nil)
	mockMsg := NewMockMsg(ctrl)
	mockMsg.EXPECT().Data().Return([]byte(data)).AnyTimes()
	mockMsg.EXPECT().Ack().Return(nil).AnyTimes()

	return mockMsg
}

func TestClient_Query_EmptyQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockConnManager.EXPECT().IsConnected().Return(true).AnyTimes()

	client := &Client{
		connManager: mockConnManager,
		logger:      logging.NewMockLogger(logging.DEBUG),
	}

	query := ""

	result, err := client.Query(t.Context(), query)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, errEmptySubject, err)
}

func TestClient_Query_ClientNotConnected(t *testing.T) {
	client := &Client{
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	query := "test-subject"

	result, err := client.Query(t.Context(), query)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, errClientNotConnected, err)
}

func TestClient_Query_JetStreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	client := &Client{
		connManager: mockConnManager,
		logger:      mockLogger,
	}

	query := "test-subject"

	mockConnManager.EXPECT().IsConnected().Return(true)
	mockConnManager.EXPECT().JetStream().Return(nil, errJetStream)

	result, err := client.Query(t.Context(), query)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), errJetStream.Error())
}

func TestClient_Query_ConsumerCreationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockJetStream := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	client := &Client{
		connManager: mockConnManager,
		logger:      mockLogger,
		Config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
		},
	}

	query := "test-subject"

	mockConnManager.EXPECT().IsConnected().Return(true)
	mockConnManager.EXPECT().JetStream().Return(mockJetStream, nil)
	mockJetStream.EXPECT().CreateOrUpdateConsumer(gomock.Any(), "test-stream", gomock.Any()).
		Return(nil, errConsumerCreationError)

	result, err := client.Query(t.Context(), query)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), errConsumerCreationError.Error())
}

func TestClient_Query_MessageFetchError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockJetStream := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMessageBatch := NewMockMessageBatch(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	client := &Client{
		connManager: mockConnManager,
		logger:      mockLogger,
		Config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
			MaxWait: 5 * time.Second,
		},
	}

	query := "test-subject"

	// Mock expectations
	mockConnManager.EXPECT().IsConnected().Return(true)
	mockConnManager.EXPECT().JetStream().Return(mockJetStream, nil)
	mockJetStream.EXPECT().CreateOrUpdateConsumer(gomock.Any(),
		"test-stream", gomock.Any()).Return(mockConsumer, nil)
	mockConsumer.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(mockMessageBatch, errHandlerError)
	mockJetStream.EXPECT().DeleteConsumer(gomock.Any(), "test-stream", gomock.Any()).Return(nil)
	mockConsumer.EXPECT().Info(gomock.Any()).Return(&jetstream.ConsumerInfo{Name: "test-stream"}, nil)

	// Call the Query method
	result, err := client.Query(t.Context(), query)

	// Assertions
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), errHandlerError.Error())
}
