package nats

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
	mockConn := NewMockconnInterface(ctrl)

	conf := &Config{
		Server: NatsServer,
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &client{
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
	mockConn := NewMockconnInterface(ctrl)

	config := &Config{
		Server: NatsServer,
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &client{
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
	mockMsg := NewMockMsg(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	metrics := NewMockMetrics(ctrl)

	client := &client{
		JetStream: mockJS,
		Logger:    logger,
		Metrics:   metrics,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)

	mockConsumer.EXPECT().Fetch(client.Config.BatchSize, gomock.Any()).Return(mockMsgBatch, nil).Times(1)
	mockConsumer.EXPECT().Fetch(client.Config.BatchSize, gomock.Any()).Return(nil, context.Canceled).AnyTimes()

	msgChan := make(chan jetstream.Msg, 1)

	mockMsg.EXPECT().Data().Return([]byte("test message")).AnyTimes()
	mockMsg.EXPECT().Subject().Return("test-subject").AnyTimes()
	mockMsg.EXPECT().Ack().Return(nil).AnyTimes()

	msgChan <- mockMsg
	close(msgChan)

	mockMsgBatch.EXPECT().Messages().Return(msgChan)
	mockMsgBatch.EXPECT().Error().Return(nil).AnyTimes()

	messageReceived := make(chan bool)

	err := client.Subscribe(ctx, "test-subject", func(_ context.Context, msg jetstream.Msg) error {
		assert.Equal(t, []byte("test message"), msg.Data())
		assert.Equal(t, "test-subject", msg.Subject())
		messageReceived <- true

		return nil
	})

	require.NoError(t, err)

	select {
	case <-messageReceived:
		// Test passed
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for message")
	}
}

func TestNATSClient_SubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewLogger(logging.DEBUG)
	metrics := NewMockMetrics(ctrl)

	client := &client{
		JetStream: mockJS,
		Logger:    logger,
		Metrics:   metrics,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer: "test-consumer",
		},
	}

	ctx := context.Background()

	expectedErr := errFailedToCreateStream
	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(nil, expectedErr)

	var err error

	logs := testutil.StderrOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		err = client.Subscribe(ctx, "test-subject", func(_ context.Context, _ jetstream.Msg) error {
			return nil // This shouldn't be called in this error case
		})
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create stream")
	assert.Contains(t, logs, "failed to create or update consumer: failed to create stream")
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	mockConn := NewMockconnInterface(ctrl)

	client := &client{
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		Server: NatsServer,
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	logs := testutil.StdoutOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		client, err := New(config, mockLogger, mockMetrics)

		require.NoError(t, err)
		assert.NotNil(t, client)

		natsClient, ok := client.(*PubSubWrapper)
		assert.True(t, ok, "Returned client is not a NatsPubSubWrapper")

		if ok {
			assert.NotNil(t, natsClient.Client)
			assert.NotNil(t, natsClient.Client.DeleteStream)
			assert.NotNil(t, natsClient.Client.CreateStream)
			assert.NotNil(t, natsClient.Client.CreateOrUpdateStream)
		}
	})

	assert.Contains(t, logs, fmt.Sprintf("connecting to client server '%s'", NatsServer))
	assert.Contains(t, logs, fmt.Sprintf("connected to client server '%s'", NatsServer))
}

func TestNew_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

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
			mockLogger := logging.NewMockLogger(logging.DEBUG)
			client, err := New(tc.config, mockLogger, mockMetrics)

			require.Error(t, err)
			assert.Nil(t, client)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestNatsClient_DeleteStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	client := &client{JetStream: mockJS}

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

	client := &client{
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

	client := &client{
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
	client := &client{
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
	client := &client{
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
	client := &client{
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
	client := &client{
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
	client := &client{
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
	mockConn := NewMockconnInterface(ctrl)

	client := &client{
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
	logger := logging.NewMockLogger(logging.DEBUG)
	metrics := NewMockMetrics(ctrl)

	client := &client{
		JetStream: mockJS,
		Logger:    logger,
		Metrics:   metrics,
		Config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer: "test-consumer",
		},
	}

	ctx := context.Background()
	expectedErr := errFailedToCreateConsumer

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(nil, expectedErr)

	err := client.Subscribe(ctx, "test-subject", func(_ context.Context, _ jetstream.Msg) error {
		return nil
	})

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_HandleMessageError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	client := &client{
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
	client := &client{
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
	client := &client{
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
	client := &client{
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
