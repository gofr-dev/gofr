package nats

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestValidateConfigs(t *testing.T) {
	testCases := []struct {
		name     string
		config   Config
		expected error
	}{
		{
			name: "Valid Config",
			config: Config{
				Server: NatsServer,
				Stream: StreamConfig{
					Stream:   "test-stream",
					Subjects: []string{"test-subject"},
				},
			},
			expected: nil,
		},
		{
			name:     "Empty Server",
			config:   Config{},
			expected: errServerNotProvided,
		},
		{
			name: "Empty Stream Subject",
			config: Config{
				Server: NatsServer,
				Stream: StreamConfig{
					Stream: "test-stream",
					// Subjects is intentionally left empty
				},
			},
			expected: errSubjectsNotProvided,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfigs(&tc.config)
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestNATSClient_Publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	mockConn := NewMockConnInterface(ctrl)

	conf := &Config{
		Server: NatsServer,
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &Client{
		Conn:      mockConn,
		JetStream: mockJS,
		Config:    conf,
		Logger:    mockLogger,
		Metrics:   mockMetrics,
	}

	ctx := context.Background()
	subject := "test-subject"
	message := []byte("test-message")

	// Set up expected calls
	mockMetrics.EXPECT().
		IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", subject)
	mockJS.EXPECT().
		Publish(gomock.Any(), subject, message).
		Return(&jetstream.PubAck{}, nil)
	mockMetrics.EXPECT().
		IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "subject", subject)

	// We don't need to set an expectation for NatsConn() in this test,
	// as we're not using it in the Publish method.

	// Call Publish
	err := client.Publish(ctx, subject, message)
	require.NoError(t, err)
}

func TestNATSClient_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	mockConn := NewMockConnInterface(ctrl)

	config := &Config{
		Server: NatsServer,
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &Client{
		Conn:      mockConn,
		JetStream: nil, // Simulate JetStream being nil
		Metrics:   metrics,
		Config:    config,
	}

	ctx := context.TODO()
	subject := "test"
	message := []byte("test-message")

	metrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", subject)

	logs := testutil.StderrOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		err := client.Publish(ctx, subject, message)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JetStream is not configured")
	})

	assert.Contains(t, logs, "JetStream is not configured")
}

func TestNATSClient_SubscribeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMsgBatch := NewMockMessageBatch(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockMsg := NewMockMsg(ctrl)

	client := &Client{
		JetStream: mockJS,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
		Metrics:       mockMetrics,
		Subscriptions: make(map[string]*subscription),
		topicBuffers:  make(map[string]chan *pubsub.Message),
		bufferSize:    1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", "test-subject")
	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)
	mockConsumer.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(mockMsgBatch, nil).AnyTimes()

	msgChan := make(chan jetstream.Msg, 1)
	msgChan <- mockMsg
	close(msgChan)

	mockMsgBatch.EXPECT().Messages().Return(msgChan).AnyTimes() // Allow multiple calls to Messages()
	mockMsgBatch.EXPECT().Error().Return(nil).AnyTimes()

	mockMsg.EXPECT().Data().Return([]byte("test message"))
	mockMsg.EXPECT().Headers().Return(nil)

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", "test-subject")

	// Call Subscribe
	msg, err := client.Subscribe(ctx, "test-subject")

	require.NoError(t, err)
	assert.NotNil(t, msg)
	assert.Equal(t, "test-subject", msg.Topic)
	assert.Equal(t, []byte("test message"), msg.Value)
}

func TestNATSClient_SubscribeTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMsgBatch := NewMockMessageBatch(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := &Client{
		JetStream: mockJS,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer:  "test-consumer",
			MaxWait:   10 * time.Millisecond, // Reduced timeout for faster test
			BatchSize: 1,
		},
		Metrics:       mockMetrics,
		Subscriptions: make(map[string]*subscription),
		topicBuffers:  make(map[string]chan *pubsub.Message),
		bufferSize:    1,
		Logger:        logging.NewMockLogger(logging.DEBUG),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", "test-subject")
	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)
	mockConsumer.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(mockMsgBatch, nil).AnyTimes()
	mockMsgBatch.EXPECT().Messages().Return(make(chan jetstream.Msg)).AnyTimes() // Return an empty channel to simulate timeout
	mockMsgBatch.EXPECT().Error().Return(nil).AnyTimes()

	msg, err := client.Subscribe(ctx, "test-subject")

	require.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestNATSClient_SubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := &Client{
		JetStream: mockJS,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer: "test-consumer",
		},
		Metrics:       mockMetrics,
		Subscriptions: make(map[string]*subscription),
	}

	ctx := context.Background()

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", "test-subject")

	expectedErr := errFailedToCreateConsumer
	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(nil, expectedErr)

	logs := testutil.StderrOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		msg, err := client.Subscribe(ctx, "test-subject")

		require.Error(t, err)
		assert.Nil(t, msg)
		assert.Equal(t, expectedErr, err)
	})

	assert.Contains(t, logs, "failed to create or update consumer")
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	mockConn := NewMockConnInterface(ctrl)

	client := &Client{
		Conn:      mockConn,
		JetStream: mockJS,
		Logger:    mockLogger,
		Metrics:   mockMetrics,
		Config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
		},
		Subscriptions: map[string]*subscription{
			"test-subject": {
				cancel: func() {},
			},
		},
	}

	mockConn.EXPECT().Close()

	err := client.Close()
	require.NoError(t, err)
	assert.Empty(t, client.Subscriptions)
}

func TestNew(t *testing.T) {
	config := &Config{
		Server: NatsServer,
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
	assert.NotNil(t, natsClient.Client.Subscriptions)
	assert.NotNil(t, natsClient.Client.topicBuffers)
	assert.Equal(t, config.BatchSize, natsClient.Client.bufferSize)

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
	assert.Nil(t, natsClient.Client.Conn)
	assert.Nil(t, natsClient.Client.JetStream)
}

func TestNew_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		name        string
		config      *Config
		expectedErr error
	}{
		{
			name: "Invalid Config",
			config: &Config{
				Server: "", // Invalid: empty server
			},
			expectedErr: errServerNotProvided,
		},
		// Add more test cases for other error scenarios
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := New(tc.config)
			assert.NotNil(t, client, "Client should not be nil even with invalid config")
		})
	}
}

func TestNatsClient_DeleteStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	client := &Client{JetStream: mockJS}

	ctx := context.Background()
	streamName := "test-stream"

	mockJS.EXPECT().DeleteStream(ctx, streamName).Return(nil)

	err := client.DeleteStream(ctx, streamName)
	assert.NoError(t, err)
}

func TestNatsClient_CreateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	client := &Client{
		JetStream: mockJS,
		Logger:    mockLogger,
		Config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
		},
	}

	ctx := context.Background()

	mockJS.EXPECT().
		CreateStream(ctx, gomock.Any()).
		Return(nil, nil)

	// setup test config
	client.Config.Stream.Stream = "test-stream"

	logs := testutil.StdoutOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		err := client.CreateStream(ctx, client.Config.Stream)
		require.NoError(t, err)
	})

	assert.Contains(t, logs, "creating stream")
	assert.Contains(t, logs, "test-stream")
}

func TestNATSClient_CreateOrUpdateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	mockStream := NewMockStream(ctrl)

	client := &Client{
		JetStream: mockJS,
		Logger:    mockLogger,
		Metrics:   mockMetrics,
		Config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
		},
	}

	ctx := context.Background()
	cfg := &jetstream.StreamConfig{
		Name:     "test-stream",
		Subjects: []string{"test.subject"},
	}

	// Expect the CreateOrUpdateStream call
	mockJS.EXPECT().
		CreateOrUpdateStream(ctx, *cfg).
		Return(mockStream, nil)

	// Capture log output
	logs := testutil.StdoutOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		stream, err := client.CreateOrUpdateStream(ctx, cfg)

		// Assert the results
		require.NoError(t, err)
		assert.Equal(t, mockStream, stream)
	})

	// Check the logs
	assert.Contains(t, logs, "creating or updating stream test-stream")
}

func TestNATSClient_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		JetStream: mockJS,
		Logger:    mockLogger,
		Config:    &Config{},
	}

	ctx := context.Background()

	mockJS.EXPECT().
		CreateStream(ctx, gomock.Any()).
		Return(nil, nil)

	err := client.CreateTopic(ctx, "test-topic")
	require.NoError(t, err)
}

func TestNATSClient_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		JetStream: mockJS,
		Logger:    mockLogger,
		Config:    &Config{},
	}

	ctx := context.Background()

	mockJS.EXPECT().DeleteStream(ctx, "test-topic").Return(nil)

	err := client.DeleteTopic(ctx, "test-topic")
	require.NoError(t, err)
}

func TestNATSClient_NakMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		Logger: mockLogger,
	}

	// Successful Nak
	mockMsg.EXPECT().Nak().Return(nil)
	err := client.NakMessage(mockMsg)
	require.NoError(t, err)

	// Failed Nak
	mockMsg.EXPECT().Nak().Return(assert.AnError)
	err = client.NakMessage(mockMsg)
	assert.Error(t, err)
}

func TestNATSClient_HandleFetchError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		Logger: mockLogger,
	}

	stdoutLogs := testutil.StdoutOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		client.HandleFetchError(assert.AnError)
	})

	stderrLogs := testutil.StderrOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		client.HandleFetchError(assert.AnError)
	})

	allLogs := stdoutLogs + stderrLogs

	assert.Contains(t, allLogs, "failed to fetch messages: assert.AnError", "Expected log not found")
}

func TestNATSClient_DeleteTopic_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		JetStream: mockJS,
		Logger:    mockLogger,
		Config:    &Config{},
	}

	ctx := context.Background()

	expectedErr := errFailedToDeleteStream
	mockJS.EXPECT().DeleteStream(ctx, "test-topic").Return(expectedErr)

	err := client.DeleteTopic(ctx, "test-topic")
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_Publish_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	mockConn := NewMockConnInterface(ctrl)

	client := &Client{
		Conn:      mockConn,
		JetStream: mockJS,
		Logger:    mockLogger,
		Metrics:   mockMetrics,
		Config:    &Config{},
	}

	ctx := context.Background()
	subject := "test-subject"
	message := []byte("test-message")

	expectedErr := errPublishError

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", subject)
	mockJS.EXPECT().Publish(gomock.Any(), subject, message).Return(nil, expectedErr)

	err := client.Publish(ctx, subject, message)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_SubscribeCreateConsumerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := &Client{
		JetStream: mockJS,
		Metrics:   mockMetrics,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer: "test-consumer",
		},
		Subscriptions: make(map[string]*subscription),
		messageBuffer: make(chan *pubsub.Message, 1),
	}

	ctx := context.Background()
	expectedErr := errFailedToCreateConsumer

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", "test-subject")
	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(nil, expectedErr)

	logs := testutil.StderrOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		msg, err := client.Subscribe(ctx, "test-subject")

		require.Error(t, err)
		assert.Nil(t, msg)
		assert.Equal(t, expectedErr, err)
	})

	assert.Contains(t, logs, "failed to create or update consumer")
}

func TestNATSClient_HandleMessageError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		Logger: logger,
	}

	ctx := context.Background()

	// Set up expectations
	mockMsg.EXPECT().Nak().Return(nil)

	handlerErr := errHandlerError
	handler := func(_ context.Context, _ jetstream.Msg) error {
		return handlerErr
	}

	// Capture log output
	logs := testutil.StderrOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		err := client.HandleMessage(ctx, mockMsg, handler)
		assert.NoError(t, err)
	})

	// Assert on the captured log output
	assert.Contains(t, logs, "error handling message: handler error")
}

func TestNATSClient_DeleteStreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		JetStream: mockJS,
		Logger:    mockLogger,
	}

	ctx := context.Background()
	streamName := "test-stream"
	expectedErr := errFailedToDeleteStream

	mockJS.EXPECT().DeleteStream(ctx, streamName).Return(expectedErr)

	err := client.DeleteStream(ctx, streamName)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_CreateStreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		JetStream: mockJS,
		Logger:    mockLogger,
		Config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
		},
	}

	ctx := context.Background()
	expectedErr := errFailedToCreateStream

	mockJS.EXPECT().CreateStream(ctx, gomock.Any()).Return(nil, expectedErr)

	err := client.CreateStream(ctx, client.Config.Stream)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_CreateOrUpdateStreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &Client{
		JetStream: mockJS,
		Logger:    mockLogger,
	}

	ctx := context.Background()
	cfg := &jetstream.StreamConfig{
		Name:     "test-stream",
		Subjects: []string{"test.subject"},
	}
	expectedErr := errFailedCreateOrUpdateStream

	mockJS.EXPECT().CreateOrUpdateStream(ctx, *cfg).Return(nil, expectedErr)

	stream, err := client.CreateOrUpdateStream(ctx, cfg)
	require.Error(t, err)
	assert.Nil(t, stream)
	assert.Equal(t, expectedErr, err)
}
