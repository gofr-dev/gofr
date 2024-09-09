package nats

import (
	"context"
	"errors"
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
			name:     "Valid Config",
			config:   Config{Server: "nats://localhost:4222"},
			expected: nil,
		},
		{
			name:     "Empty Server",
			config:   Config{},
			expected: errServerNotProvided,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigs(tc.config)
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestNATSClient_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStreamContext(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	client := &natsClient{js: mockJS, metrics: mockMetrics}
	ctx := context.TODO()

	testCases := []struct {
		desc      string
		client    *natsClient
		stream    string
		msg       []byte
		setupMock func()
		expErr    error
		expLog    string
	}{
		{
			desc:   "error JetStream is nil",
			client: &natsClient{metrics: mockMetrics},
			stream: "test",
			expErr: errPublisherNotConfigured,
		},
		{
			desc:   "error stream is not provided",
			client: client,
			expErr: errPublisherNotConfigured,
		},
		{
			desc:   "error while publishing message",
			client: client,
			stream: "test",
			setupMock: func() {
				mockJS.EXPECT().Publish("test", gomock.Any()).Return(nil, errors.New("publish error"))
			},
			expErr: errors.New("publish error"),
			expLog: "failed to publish message to NATS JetStream",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			logger := logging.NewMockLogger(logging.DEBUG)
			tc.client.logger = logger
			if tc.setupMock != nil {
				tc.setupMock()
			}
			mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "stream", tc.stream).AnyTimes()

			logs := testutil.StderrOutputForFunc(func() {
				err := tc.client.Publish(ctx, tc.stream, tc.msg)
				assert.Equal(t, tc.expErr, err)
			})

			assert.Contains(t, logs, tc.expLog)
		})
	}
}

func TestNATSClient_Publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStreamContext(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	client := &natsClient{
		js:      mockJS,
		logger:  logger,
		metrics: mockMetrics,
		config:  Config{Server: "nats://localhost:4222"},
	}

	ctx := context.TODO()
	mockJS.EXPECT().Publish("test", []byte(`hello`)).Return(&nats.PubAck{}, nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "stream", "test")
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "stream", "test")

	logs := testutil.StdoutOutputForFunc(func() {
		err := client.Publish(ctx, "test", []byte(`hello`))
		require.NoError(t, err)
	})

	assert.Contains(t, logs, "NATS")
	assert.Contains(t, logs, "PUB")
	assert.Contains(t, logs, "hello")
	assert.Contains(t, logs, "test")
}

func TestNATSClient_SubscribeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStreamContext(ctrl)
	mockSub := NewMockSubscription(ctrl)
	mockMsg := nats.NewMsg("test")
	mockMsg.Data = []byte("hello")
	mockMetrics := NewMockMetrics(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	client := &natsClient{
		js:      mockJS,
		logger:  logger,
		metrics: mockMetrics,
		config: Config{
			Server:   "nats://localhost:4222",
			Consumer: "test-consumer",
			MaxWait:  time.Second,
		},
	}

	ctx := context.TODO()

	mockJS.EXPECT().PullSubscribe("test", "test-consumer", gomock.Any()).Return(mockSub, nil)
	mockSub.EXPECT().Fetch(1, gomock.Any()).Return([]*nats.Msg{mockMsg}, nil)

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "stream", "test", "consumer", "test-consumer")
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "stream", "test", "consumer", "test-consumer")

	logs := testutil.StdoutOutputForFunc(func() {
		msg, err := client.Subscribe(ctx, "test")
		require.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, []byte("hello"), msg.Value)
		assert.Equal(t, "test", msg.Topic)
	})

	assert.Contains(t, logs, "NATS")
	assert.Contains(t, logs, "SUB")
	assert.Contains(t, logs, "hello")
	assert.Contains(t, logs, "test")
}

func TestNATSClient_SubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStreamContext(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)
	client := &natsClient{
		js:      mockJS,
		logger:  logger,
		metrics: mockMetrics,
		config: Config{
			Server:   "nats://localhost:4222",
			Consumer: "test-consumer",
		},
	}

	ctx := context.TODO()
	mockJS.EXPECT().PullSubscribe("test", "test-consumer", gomock.Any()).Return(nil, errors.New("subscribe error"))
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "stream", "test", "consumer", "test-consumer")

	logs := testutil.StderrOutputForFunc(func() {
		msg, err := client.Subscribe(ctx, "test")
		assert.Error(t, err)
		assert.Nil(t, msg)
	})

	assert.Contains(t, logs, "subscribe error")
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnection(ctrl)
	client := &natsClient{conn: mockConn}

	mockConn.EXPECT().Drain().Return(nil)

	err := client.Close()
	require.NoError(t, err)
}

func TestNew(t *testing.T) {
	testCases := []struct {
		name      string
		config    Config
		expectNil bool
	}{
		{
			name:      "Empty Server",
			config:    Config{},
			expectNil: true,
		},
		{
			name: "Valid Config",
			config: Config{
				Server: "nats://localhost:4222",
			},
			expectNil: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(tc.config, logging.NewMockLogger(logging.ERROR), NewMockMetrics(gomock.NewController(t)))
			if tc.expectNil {
				assert.Nil(t, client)
				assert.Error(t, err)
			} else {
				assert.NotNil(t, client)
				assert.NoError(t, err)
			}
		})
	}
}
