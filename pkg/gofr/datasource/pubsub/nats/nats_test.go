package nats

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
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

	mockJS := NewMockJetStreamContext(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	logs := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.DEBUG)
		client := &NATSClient{
			js:      mockJS,
			logger:  logger,
			metrics: mockMetrics,
			config:  &Config{Server: natsServer},
		}

		ctx := context.TODO()

		mockJS.EXPECT().Publish("test", []byte(`hello`)).Return(&nats.PubAck{}, nil)
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

	mockJS := NewMockJetStreamContext(ctrl)
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
				mockJS.EXPECT().Publish("test", gomock.Any()).Return(nil, errPublish)
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

	mockJS := NewMockJetStreamContext(ctrl)
	mockSub := &nats.Subscription{}
	mockMsg := &nats.Msg{Data: []byte("hello"), Subject: "test"}
	mockMetrics := NewMockMetrics(ctrl)

	logs := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.DEBUG)
		client := &NATSClient{
			js:      mockJS,
			logger:  logger,
			metrics: mockMetrics,
			config: &Config{
				Server:   natsServer,
				Consumer: "test-consumer",
				MaxWait:  time.Second,
			},
			mu: &sync.RWMutex{},
		}

		client.fetchFunc = func(_ *nats.Subscription, _ int, _ ...nats.PullOpt) ([]*nats.Msg, error) {
			return []*nats.Msg{mockMsg}, nil
		}

		ctx := context.TODO()

		mockJS.EXPECT().PullSubscribe("test", "test-consumer", gomock.Any()).Return(mockSub, nil)
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "stream", "test", "consumer", "test-consumer")
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "stream", "test", "consumer", "test-consumer")

		msg, err := client.Subscribe(ctx, "test")
		require.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, []byte("hello"), msg.Value)
		assert.Equal(t, "test", msg.Topic)
	})

	assert.Contains(t, logs, "NATS")
	assert.Contains(t, logs, "SUB")
	assert.Contains(t, logs, "test")
	assert.Contains(t, logs, "hello")
}

func TestNATSClient_SubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStreamContext(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.DEBUG)
		client := &NATSClient{
			js:      mockJS,
			logger:  logger,
			metrics: mockMetrics,
			config: &Config{
				Server:   "nats://localhost:4222",
				Consumer: "test-consumer",
			},
			mu: &sync.RWMutex{},
		}

		ctx := context.TODO()

		mockJS.EXPECT().PullSubscribe("test", "test-consumer", gomock.Any()).Return(nil, errSubscribe)
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "stream", "test", "consumer", "test-consumer")

		msg, err := client.Subscribe(ctx, "test")
		require.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "failed to create or attach consumer")
	})

	assert.Contains(t, logs, "failed to create or attach consumer: subscribe error")
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnection(ctrl)
	mockJS := NewMockJetStreamContext(ctrl)

	client := &NATSClient{
		conn: mockConn,
		js:   mockJS,
		config: &Config{
			Stream: StreamConfig{Subject: "test-stream"},
		},
	}

	mockJS.EXPECT().DeleteStream("test-stream").Return(nil)
	mockConn.EXPECT().Drain().Return(nil)

	err := client.Close()
	require.NoError(t, err)
}

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		name                string
		config              Config
		natsConnectFunc     func(string, ...nats.Option) (*nats.Conn, error)
		jetStreamCreateFunc func(*nats.Conn, ...nats.JSOpt) (JetStreamContext, error)
		expectErr           bool
	}{
		{
			name:   "Empty Server",
			config: Config{},
			natsConnectFunc: func(_ string, _ ...nats.Option) (*nats.Conn, error) {
				return &nats.Conn{}, nil
			},
			jetStreamCreateFunc: func(_ *nats.Conn, _ ...nats.JSOpt) (JetStreamContext, error) {
				return NewMockJetStreamContext(ctrl), nil
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
				return &nats.Conn{}, nil
			},
			jetStreamCreateFunc: func(_ *nats.Conn, _ ...nats.JSOpt) (JetStreamContext, error) {
				return NewMockJetStreamContext(ctrl), nil
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
			jetStreamCreateFunc: func(_ *nats.Conn, _ ...nats.JSOpt) (JetStreamContext, error) {
				return NewMockJetStreamContext(ctrl), nil
			},
			expectErr: true,
		},
		{
			name: "Error in jetStreamCreateFunc",
			config: Config{
				Server: natsServer,
				Stream: StreamConfig{Subject: "test-stream"},
			},
			natsConnectFunc: func(_ string, _ ...nats.Option) (*nats.Conn, error) {
				return &nats.Conn{}, nil
			},
			jetStreamCreateFunc: func(_ *nats.Conn, _ ...nats.JSOpt) (JetStreamContext, error) {
				return nil, errJetStream
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(&tc.config, logging.NewMockLogger(logging.ERROR), NewMockMetrics(ctrl), tc.natsConnectFunc, tc.jetStreamCreateFunc)
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

	mockJS := NewMockJetStreamContext(ctrl)
	client := &NATSClient{js: mockJS}

	ctx := context.Background()
	streamName := "test-stream"

	mockJS.EXPECT().DeleteStream(streamName).Return(nil)

	err := client.DeleteStream(ctx, streamName)
	assert.NoError(t, err)
}

func TestNatsClient_CreateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStreamContext(ctrl)
	client := &NATSClient{js: mockJS}

	ctx := context.Background()
	streamName := "test-stream"

	mockJS.EXPECT().AddStream(gomock.Any()).Return(&nats.StreamInfo{}, nil)

	err := client.CreateStream(ctx, streamName)
	assert.NoError(t, err)
}
