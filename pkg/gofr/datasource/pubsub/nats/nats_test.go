package nats

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr"
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
				Server: natsServer,
				Stream: StreamConfig{Subject: "test-stream"},
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
				Server: natsServer,
			},
			expected: errStreamNotProvided,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigs(&tc.config)
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestNATSClient_Publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	ctx := context.TODO()

	logs := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.DEBUG)
		client := &NATSClient{
			js:      mockJS,
			logger:  logger,
			metrics: mockMetrics,
			config:  &Config{Server: natsServer},
		}

		mockJS.EXPECT().Publish(ctx, "test", []byte(`hello`)).Return(&nats.PubAck{}, nil)
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "stream", "test")
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "stream", "test")

		err := client.Publish(ctx, "test", []byte(`hello`))
		require.NoError(t, err)
	})

	assert.Contains(t, logs, "NATS")
	assert.Contains(t, logs, "PUB")
	assert.Contains(t, logs, "test")
	assert.Contains(t, logs, "hello")
}

func TestNATSClient_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	ctx := context.TODO()

	testCases := []struct {
		desc      string
		client    *NATSClient
		stream    string
		msg       []byte
		setupMock func()
		expErr    error
		expLog    string
	}{
		{
			desc: "error JetStream is nil",
			client: &NATSClient{
				js:      nil,
				metrics: mockMetrics,
			},
			stream: "test",
			msg:    []byte("test message"),
			expErr: errPublisherNotConfigured,
			expLog: "can't publish message: publisher not configured or stream is empty",
		},
		{
			desc: "error stream is not provided",
			client: &NATSClient{
				js:      mockJS,
				metrics: mockMetrics,
			},
			expErr: errPublisherNotConfigured,
		},
		{
			desc: "error while publishing message",
			client: &NATSClient{
				js:      mockJS,
				metrics: mockMetrics,
			},
			stream: "test",
			setupMock: func() {
				mockJS.EXPECT().Publish(ctx, "test", gomock.Any()).Return(nil, errPublish)
			},
			expErr: errPublish,
			expLog: "failed to publish message to NATS JetStream",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.setupMock != nil {
				tc.setupMock()
			}

			mockMetrics.EXPECT().
				IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "stream", tc.stream).
				AnyTimes()

			logs := testutil.StderrOutputForFunc(func() {
				logger := logging.NewMockLogger(logging.DEBUG)
				tc.client.logger = logger

				err := tc.client.Publish(ctx, tc.stream, tc.msg)
				assert.Equal(t, tc.expErr, err)
			})

			if tc.expLog != "" {
				assert.Contains(t, logs, tc.expLog)
			}
		})
	}
}

func TestNATSClient_SubscribeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks for jetstream.JetStream and jetstream.Consumer
	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	logger := logging.NewMockLogger(logging.DEBUG)
	client := &NATSClient{
		js:      mockJS,
		logger:  logger,
		metrics: mockMetrics,
		config: &Config{
			Server: natsServer,
			Stream: StreamConfig{
				Stream:  "test-stream",
				Subject: "test-subject",
			},
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
		mu:            &sync.RWMutex{},
		subscriptions: make(map[string]*subscription),
	}

	// Set up the handler function
	messageReceived := make(chan *nats.Msg, 1)

	handler := func(ctx *gofr.Context, msg *nats.Msg) error {
		messageReceived <- msg
		return nil // Indicate successful processing
	}

	ctx := context.TODO()

	// Set up mocks and expectations

	// Mock CreateStream
	mockJS.EXPECT().
		CreateStream(ctx, gomock.Any()).
		Return(nil, nil)

	// Mock CreateOrUpdateConsumer
	mockJS.EXPECT().
		CreateOrUpdateConsumer(ctx, "test-stream", gomock.Any()).
		Return(mockConsumer, nil)

	// Simulate fetching messages
	natsMsg := &nats.Msg{Data: []byte("hello"), Subject: "test-subject"}
	mockJetstreamMsg := NewMockMsg(ctrl)
	mockJetstreamMsg.EXPECT().Data().Return([]byte("hello")).AnyTimes()
	mockJetstreamMsg.EXPECT().Ack().Return(nil).AnyTimes()

	msgsChan := make(chan jetstream.Msg, 1)
	msgsChan <- mockJetstreamMsg
	close(msgsChan)

	mockMsgs := NewMockMsgs(ctrl)
	mockMsgs.EXPECT().Messages().Return(msgsChan).AnyTimes()
	mockMsgs.EXPECT().Error().Return(nil).AnyTimes()

	// Mock Fetch
	mockConsumer.EXPECT().
		Fetch(client.config.BatchSize, gomock.Any()).
		Return(mockMsgs, nil)

	// Since the subscription loop runs indefinitely, we need to stop it after the test
	testCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Start the subscription
	err := client.Subscribe(testCtx, "test-subject", handler)
	require.NoError(t, err)

	// Wait for the message to be received
	select {
	case msg := <-messageReceived:
		require.Equal(t, natsMsg, msg)
	case <-time.After(1 * time.Second):
		t.Fatal("Did not receive message in time")
	}
}

func TestNATSClient_SubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	logger := logging.NewMockLogger(logging.DEBUG)
	client := &NATSClient{
		js:      mockJS,
		logger:  logger,
		metrics: mockMetrics,
		config: &Config{
			Server: natsServer,
			Stream: StreamConfig{
				Stream:  "test-stream",
				Subject: "test-subject",
			},
			Consumer: "test-consumer",
		},
		mu:            &sync.RWMutex{},
		subscriptions: make(map[string]*subscription),
	}

	handler := func(ctx *gofr.Context, msg *nats.Msg) error {
		return nil
	}

	ctx := context.TODO()

	// Mock CreateStream
	mockJS.EXPECT().
		CreateStream(ctx, gomock.Any()).
		Return(nil, nil)

	// Mock CreateOrUpdateConsumer to return an error
	mockJS.EXPECT().
		CreateOrUpdateConsumer(ctx, "test-stream", gomock.Any()).
		Return(nil, errSubscribe)

	err := client.Subscribe(ctx, "test-subject", handler)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create or update consumer")
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockNatsConn(ctrl)
	mockJS := NewMockJetStream(ctrl)
	client := &NATSClient{
		conn: mockConn,
		js:   mockJS,
		config: &Config{
			Stream: StreamConfig{Subject: "test-stream"},
		},
		subscriptions: map[string]*subscription{
			"test-subject": {
				cancel: func() {},
			},
		},
	}

	// setup mock context
	ctx := context.TODO()

	// Expect the stream to be deleted
	mockJS.EXPECT().DeleteStream(ctx, "test-stream").Return(nil)

	// Expect the connection to be closed
	mockConn.EXPECT().Close()

	err := client.Close(ctx)
	require.NoError(t, err)
}

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mockJS := NewMockJetStreamContext(ctrl)
	mockConn := &nats.Conn{}

	testCases := []struct {
		name            string
		config          Config
		natsConnectFunc func(string, ...nats.Option) (*nats.Conn, error)
		expectErr       bool
	}{
		{
			name:   "Empty Server",
			config: Config{},
			natsConnectFunc: func(_ string, _ ...nats.Option) (*nats.Conn, error) {
				return mockConn, nil
			},
			expectErr: true, // We expect an error due to empty server
		},
		{
			name: "Valid Config",
			config: Config{
				Server: natsServer,
				Stream: StreamConfig{Subject: "test-stream"},
			},
			natsConnectFunc: func(_ string, _ ...nats.Option) (*nats.Conn, error) {
				return mockConn, nil
			},
			expectErr: false,
		},
		{
			name: "Error in natsConnectFunc",
			config: Config{
				Server: natsServer,
				Stream: StreamConfig{Subject: "test-stream"},
			},
			natsConnectFunc: func(_ string, _ ...nats.Option) (*nats.Conn, error) {
				return nil, errNATSConnection
			},
			expectErr: true,
		},
		{
			name: "Error in JetStream",
			config: Config{
				Server: natsServer,
				Stream: StreamConfig{Subject: "test-stream"},
			},
			natsConnectFunc: func(_ string, _ ...nats.Option) (*nats.Conn, error) {
				return mockConn, nil
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			logger := logging.NewMockLogger(logging.ERROR)
			client, err := New(&tc.config, logger, NewMockMetrics(ctrl), tc.natsConnectFunc, tc.jetStreamCreate)
			if tc.expectErr {
				assert.Nil(t, client)
				assert.Error(t, err)
			} else {
				assert.NotNil(t, client)
				assert.NoError(t, err)
			}
		})
	}
}

func TestNatsClient_DeleteStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	client := &NATSClient{js: mockJS}

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
	// mockJS := NewMockJetStreamContext(ctrl)
	client := &NATSClient{js: mockJS}

	ctx := context.Background()
	streamName := "test-stream"

	// Expect CreateStream to be called
	mockJS.EXPECT().
		CreateStream(ctx, gomock.Any()).
		Return(nil, nil)

	err := client.CreateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{streamName},
	})
	assert.NoError(t, err)
}
