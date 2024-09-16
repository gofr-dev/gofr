package nats_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	natspubsub "gofr.dev/pkg/gofr/datasource/pubsub/nats"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// TestNewNATSClient tests the NewNATSClient function.
func TestNewNATSClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conf := &natspubsub.Config{
		Server: natspubsub.NatsServer,
		Stream: natspubsub.StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	mockConn := natspubsub.NewMockConnInterface(ctrl)
	mockJS := natspubsub.NewMockJetStream(ctrl)

	mockConn.EXPECT().Status().Return(nats.CONNECTED)
	mockConn.EXPECT().NatsConn().Return(&nats.Conn{})

	metrics := natspubsub.NewMockMetrics(ctrl)

	// Create a mock function for nats.Connect
	//nolint:unparam // mock function
	mockNatsConnect := func(_ string, _ ...nats.Option) (natspubsub.ConnInterface, error) {
		return mockConn, nil
	}

	// Create a mock function for jetstream.New
	//nolint:unparam // mock function
	mockJetStreamNew := func(_ *nats.Conn) (jetstream.JetStream, error) {
		return mockJS, nil
	}

	logs := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.DEBUG)
		client, err := natspubsub.NewNATSClient(conf, logger, metrics, mockNatsConnect, mockJetStreamNew)
		require.NoError(t, err)
		require.NotNil(t, client)

		assert.Equal(t, mockConn, client.Conn)
		assert.Equal(t, mockJS, client.Js)
		assert.Equal(t, conf, client.Config)
	})

	assert.Contains(t, logs, "connecting to NATS server 'nats://localhost:4222'")
	assert.Contains(t, logs, "connected to NATS server 'nats://localhost:4222'")
}

func TestValidateConfigs(t *testing.T) {
	testCases := []struct {
		name     string
		config   natspubsub.Config
		expected error
	}{
		{
			name: "Valid Config",
			config: natspubsub.Config{
				Server: natspubsub.NatsServer,
				Stream: natspubsub.StreamConfig{
					Stream:   "test-stream",
					Subjects: []string{"test-subject"},
				},
			},
			expected: nil,
		},
		{
			name:     "Empty Server",
			config:   natspubsub.Config{},
			expected: natspubsub.ErrServerNotProvided,
		},
		{
			name: "Empty Stream Subject",
			config: natspubsub.Config{
				Server: natspubsub.NatsServer,
				Stream: natspubsub.StreamConfig{
					Stream: "test-stream",
					// Subjects is intentionally left empty
				},
			},
			expected: natspubsub.ErrSubjectsNotProvided,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := natspubsub.ValidateConfigs(&tc.config)
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestNATSClient_Publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := natspubsub.NewMockMetrics(ctrl)
	mockConn := natspubsub.NewMockConnInterface(ctrl)

	conf := &natspubsub.Config{
		Server: "nats://localhost:4222",
		Stream: natspubsub.StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &natspubsub.NATSClient{
		Conn:    mockConn,
		Js:      mockJS,
		Config:  conf,
		Logger:  mockLogger,
		Metrics: mockMetrics,
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

	metrics := natspubsub.NewMockMetrics(ctrl)
	mockConn := natspubsub.NewMockConnInterface(ctrl)

	config := &natspubsub.Config{
		Server: natspubsub.NatsServer,
		Stream: natspubsub.StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &natspubsub.NATSClient{
		Conn:    mockConn,
		Js:      nil, // Simulate JetStream being nil
		Metrics: metrics,
		Config:  config,
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

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockConsumer := natspubsub.NewMockConsumer(ctrl)
	mockMsgBatch := natspubsub.NewMockMessageBatch(ctrl)
	mockMsg := natspubsub.NewMockMsg(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	metrics := natspubsub.NewMockMetrics(ctrl)

	client := &natspubsub.NATSClient{
		Js:      mockJS,
		Logger:  logger,
		Metrics: metrics,
		Config: &natspubsub.Config{
			Stream: natspubsub.StreamConfig{
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

	mockJS := natspubsub.NewMockJetStream(ctrl)
	logger := logging.NewLogger(logging.DEBUG)
	metrics := natspubsub.NewMockMetrics(ctrl)

	client := &natspubsub.NATSClient{
		Js:      mockJS,
		Logger:  logger,
		Metrics: metrics,
		Config: &natspubsub.Config{
			Stream: natspubsub.StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer: "test-consumer",
		},
	}

	ctx := context.Background()

	expectedErr := natspubsub.ErrFailedToCreateStream
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

// natsClient is a local receiver, which is used to test the NATS client.
//
//nolint:unused // used for testing
type natsClient struct {
	*natspubsub.NATSClient
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := natspubsub.NewMockMetrics(ctrl)
	mockConn := natspubsub.NewMockConnInterface(ctrl)

	client := &natspubsub.NATSClient{
		Conn:    mockConn,
		Js:      mockJS,
		Logger:  mockLogger,
		Metrics: mockMetrics,
		Config: &natspubsub.Config{
			Stream: natspubsub.StreamConfig{
				Stream: "test-stream",
			},
		},
		Subscriptions: map[string]*natspubsub.Subscription{
			"test-subject": {
				Cancel: func() {},
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

	mockMetrics := natspubsub.NewMockMetrics(ctrl)

	testCases := []struct {
		name         string
		config       *natspubsub.Config
		expectErr    bool
		expectedLogs []string
		setupMocks   func() (natspubsub.ConnInterface, jetstream.JetStream, error)
	}{
		{
			name: "Successful client creation",
			config: &natspubsub.Config{
				Server: "nats://localhost:4222",
				Stream: natspubsub.StreamConfig{
					Stream:   "test-stream",
					Subjects: []string{"test-subject"},
				},
				Consumer: "test-consumer",
			},
			expectErr: false,
			expectedLogs: []string{
				"connecting to NATS server 'nats://localhost:4222'",
				"connected to NATS server 'nats://localhost:4222'",
			},
			setupMocks: func() (natspubsub.ConnInterface, jetstream.JetStream, error) {
				mockConn := natspubsub.NewMockConnInterface(ctrl)
				mockJS := natspubsub.NewMockJetStream(ctrl)
				mockConn.EXPECT().Status().Return(nats.CONNECTED).AnyTimes()
				mockConn.EXPECT().NatsConn().Return(&nats.Conn{}).AnyTimes()
				return mockConn, mockJS, nil
			},
		},
		// ... other test cases ...
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockConn, mockJS, mockErr := tc.setupMocks()

			natsConnectMock := func(url string, opts ...nats.Option) (natspubsub.ConnInterface, error) {
				if mockErr != nil {
					return nil, mockErr
				}
				return mockConn, nil
			}

			jetstreamNewMock := func(nc *nats.Conn) (jetstream.JetStream, error) {
				if mockJS == nil {
					return nil, errors.New("failed to create JetStream")
				}
				return mockJS, nil
			}

			var client pubsub.Client
			var err error

			stdoutLogs := testutil.StdoutOutputForFunc(func() {
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client, err = natspubsub.New(tc.config, mockLogger, mockMetrics, natsConnectMock, jetstreamNewMock)
			})

			stderrLogs := testutil.StderrOutputForFunc(func() {
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client, err = natspubsub.New(tc.config, mockLogger, mockMetrics, natsConnectMock, jetstreamNewMock)
			})

			allLogs := stdoutLogs + stderrLogs

			// Print captured logs for debugging
			t.Logf("Captured logs:\n%s", allLogs)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// Check if client implements pubsub.Client interface
				_, ok := client.(pubsub.Client)
				assert.True(t, ok, "Returned client does not implement pubsub.Client interface")

				// Additional checks for required methods
				natsClient, ok := client.(*natspubsub.NatsPubSubWrapper)
				assert.True(t, ok, "Returned client is not a NatsPubSubWrapper")
				if ok {
					assert.NotNil(t, natsClient.Client.DeleteStream)
					assert.NotNil(t, natsClient.Client.CreateStream)
					assert.NotNil(t, natsClient.Client.CreateOrUpdateStream)
				}
			}

			// Check for expected logs
			for _, expectedLog := range tc.expectedLogs {
				assert.Contains(t, allLogs, expectedLog, "Expected log not found: %s", expectedLog)
			}
		})
	}
}

func TestNatsClient_DeleteStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	client := &natspubsub.NATSClient{Js: mockJS}

	ctx := context.Background()
	streamName := "test-stream"

	mockJS.EXPECT().DeleteStream(ctx, streamName).Return(nil)

	err := client.DeleteStream(ctx, streamName)
	assert.NoError(t, err)
}

func TestNatsClient_CreateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Js:     mockJS,
		Logger: mockLogger,
		Config: &natspubsub.Config{
			Stream: natspubsub.StreamConfig{
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

	assert.Contains(t, logs, "Creating stream")
	assert.Contains(t, logs, "test-stream")
}

func TestNATSClient_CreateOrUpdateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := natspubsub.NewMockMetrics(ctrl)
	mockStream := natspubsub.NewMockStream(ctrl)

	client := &natspubsub.NATSClient{
		Js:      mockJS,
		Logger:  mockLogger,
		Metrics: mockMetrics,
		Config: &natspubsub.Config{
			Stream: natspubsub.StreamConfig{
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
	assert.Contains(t, logs, "Creating or updating stream test-stream")
}

func TestNATSClient_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Js:     mockJS,
		Logger: mockLogger,
		Config: &natspubsub.Config{},
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

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Js:     mockJS,
		Logger: mockLogger,
		Config: &natspubsub.Config{},
	}

	ctx := context.Background()

	mockJS.EXPECT().DeleteStream(ctx, "test-topic").Return(nil)

	err := client.DeleteTopic(ctx, "test-topic")
	require.NoError(t, err)
}

func TestNATSClient_NakMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := natspubsub.NewMockMsg(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Logger: mockLogger,
	}

	// Successful Nak
	mockMsg.EXPECT().Nak().Return(nil)
	err := client.NakMessage(mockMsg)
	assert.NoError(t, err)

	// Failed Nak
	mockMsg.EXPECT().Nak().Return(assert.AnError)
	err = client.NakMessage(mockMsg)
	assert.Error(t, err)
}

func TestNATSClient_HandleFetchError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
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

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Js:     mockJS,
		Logger: mockLogger,
		Config: &natspubsub.Config{},
	}

	ctx := context.Background()

	expectedErr := errors.New("delete stream error")
	mockJS.EXPECT().DeleteStream(ctx, "test-topic").Return(expectedErr)

	err := client.DeleteTopic(ctx, "test-topic")
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNewNATSClient_ConnectionFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conf := &natspubsub.Config{
		Server: "nats://localhost:4222",
		Stream: natspubsub.StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	mockMetrics := natspubsub.NewMockMetrics(ctrl)
	expectedErr := errors.New("connection failed")

	mockNatsConnect := func(_ string, _ ...nats.Option) (natspubsub.ConnInterface, error) {
		return nil, expectedErr
	}

	mockJetStreamNew := func(_ *nats.Conn) (jetstream.JetStream, error) {
		return nil, nil
	}

	logger := logging.NewMockLogger(logging.DEBUG)
	client, err := natspubsub.NewNATSClient(conf, logger, mockMetrics, mockNatsConnect, mockJetStreamNew)

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_Publish_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := natspubsub.NewMockMetrics(ctrl)
	mockConn := natspubsub.NewMockConnInterface(ctrl)

	client := &natspubsub.NATSClient{
		Conn:    mockConn,
		Js:      mockJS,
		Logger:  mockLogger,
		Metrics: mockMetrics,
		Config:  &natspubsub.Config{},
	}

	ctx := context.Background()
	subject := "test-subject"
	message := []byte("test-message")

	expectedErr := errors.New("publish error")

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", subject)
	mockJS.EXPECT().Publish(gomock.Any(), subject, message).Return(nil, expectedErr)

	err := client.Publish(ctx, subject, message)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNewNATSClient_JetStreamFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conf := &natspubsub.Config{
		Server: "nats://localhost:4222",
		Stream: natspubsub.StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	mockMetrics := natspubsub.NewMockMetrics(ctrl)
	mockConn := natspubsub.NewMockConnInterface(ctrl)
	expectedErr := errors.New("jetstream creation failed")

	mockNatsConnect := func(_ string, _ ...nats.Option) (natspubsub.ConnInterface, error) {
		return mockConn, nil
	}

	mockJetStreamNew := func(_ *nats.Conn) (jetstream.JetStream, error) {
		return nil, expectedErr
	}

	mockConn.EXPECT().Status().Return(nats.CONNECTED)
	mockConn.EXPECT().NatsConn().Return(&nats.Conn{})

	logger := logging.NewMockLogger(logging.DEBUG)
	client, err := natspubsub.NewNATSClient(conf, logger, mockMetrics, mockNatsConnect, mockJetStreamNew)

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Equal(t, expectedErr, err)
}

func TestNew_NewNATSClientFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conf := &natspubsub.Config{
		Server: "nats://localhost:4222",
		Stream: natspubsub.StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	mockMetrics := natspubsub.NewMockMetrics(ctrl)
	expectedErr := errors.New("NATS client creation failed")

	mockNatsConnect := func(_ string, _ ...nats.Option) (natspubsub.ConnInterface, error) {
		return nil, expectedErr
	}

	mockJetStreamNew := func(_ *nats.Conn) (jetstream.JetStream, error) {
		return nil, nil
	}

	logger := logging.NewMockLogger(logging.DEBUG)
	client, err := natspubsub.New(conf, logger, mockMetrics, mockNatsConnect, mockJetStreamNew)

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_SubscribeCreateConsumerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	metrics := natspubsub.NewMockMetrics(ctrl)

	client := &natspubsub.NATSClient{
		Js:      mockJS,
		Logger:  logger,
		Metrics: metrics,
		Config: &natspubsub.Config{
			Stream: natspubsub.StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer: "test-consumer",
		},
	}

	ctx := context.Background()
	expectedErr := errors.New("create consumer error")

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

	mockMsg := natspubsub.NewMockMsg(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Logger: logger,
	}

	ctx := context.Background()

	// Set up expectations
	mockMsg.EXPECT().Nak().Return(nil)

	handlerErr := natspubsub.ErrHandlerError
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
	assert.Contains(t, logs, "Error handling message: handler error")
}

/*
func TestNATSClient_SubscribeProcessMessagesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockConsumer := natspubsub.NewMockConsumer(ctrl)
	mockMsgBatch := natspubsub.NewMockMessageBatch(ctrl)
	mockMsg := natspubsub.NewMockMsg(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	metrics := natspubsub.NewMockMetrics(ctrl)

	client := &natspubsub.NATSClient{
		Js:      mockJS,
		Logger:  logger,
		Metrics: metrics,
		Config: &natspubsub.Config{
			Stream: natspubsub.StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)

	mockConsumer.EXPECT().Fetch(client.Config.BatchSize, gomock.Any()).Return(mockMsgBatch, nil).Times(1)
	mockConsumer.EXPECT().Fetch(client.Config.BatchSize, gomock.Any()).Return(nil, context.Canceled).AnyTimes()

	msgChan := make(chan jetstream.Msg, 1)
	msgChan <- mockMsg
	close(msgChan)

	mockMsg.EXPECT().Data().Return([]byte("test message")).AnyTimes()
	mockMsg.EXPECT().Subject().Return("test-subject").AnyTimes()
	mockMsg.EXPECT().Nak().Return(nil).AnyTimes()

	mockMsgBatch.EXPECT().Messages().Return(msgChan)
	mockMsgBatch.EXPECT().Error().Return(nil).AnyTimes()

	handlerErr := errors.New("handler error")

	// Capture log output
	client.Logger = logging.NewMockLogger(logging.DEBUG)
	logs := testutil.StderrOutputForFunc(func() {
		err := client.Subscribe(ctx, "test-subject", func(_ context.Context, _ jetstream.Msg) error {
			log.Println("handler called", handlerErr)
			return handlerErr
		})
		require.NoError(t, err) // Subscribe itself should not return an error
	})

	// Wait for the goroutine to process the message
	time.Sleep(100 * time.Millisecond)

	// Assert on the captured log output
	assert.Contains(t, logs, "Error handling message: handler error")
}

*/

func TestNATSClient_DeleteStreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Js:     mockJS,
		Logger: mockLogger,
	}

	ctx := context.Background()
	streamName := "test-stream"
	expectedErr := errors.New("delete stream error")

	mockJS.EXPECT().DeleteStream(ctx, streamName).Return(expectedErr)

	err := client.DeleteStream(ctx, streamName)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_CreateStreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Js:     mockJS,
		Logger: mockLogger,
		Config: &natspubsub.Config{
			Stream: natspubsub.StreamConfig{
				Stream: "test-stream",
			},
		},
	}

	ctx := context.Background()
	expectedErr := errors.New("create stream error")

	mockJS.EXPECT().CreateStream(ctx, gomock.Any()).Return(nil, expectedErr)

	err := client.CreateStream(ctx, client.Config.Stream)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_CreateOrUpdateStreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := natspubsub.NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &natspubsub.NATSClient{
		Js:     mockJS,
		Logger: mockLogger,
	}

	ctx := context.Background()
	cfg := &jetstream.StreamConfig{
		Name:     "test-stream",
		Subjects: []string{"test.subject"},
	}
	expectedErr := errors.New("create or update stream error")

	mockJS.EXPECT().CreateOrUpdateStream(ctx, *cfg).Return(nil, expectedErr)

	stream, err := client.CreateOrUpdateStream(ctx, cfg)
	require.Error(t, err)
	assert.Nil(t, stream)
	assert.Equal(t, expectedErr, err)
}
