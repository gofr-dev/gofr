package nats

import (
	"context"
	"errors"
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

func TestNewNATSClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conf := &Config{
		Server: "nats://localhost:4222",
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	mockConn := NewMockConnInterface(ctrl)
	mockJS := NewMockJetStream(ctrl)

	mockConn.EXPECT().Status().Return(nats.CONNECTED)
	mockConn.EXPECT().NatsConn().Return(&nats.Conn{})

	metrics := NewMockMetrics(ctrl)

	// Create a mock function for nats.Connect
	mockNatsConnect := func(serverURL string, opts ...nats.Option) (ConnInterface, error) {
		return mockConn, nil
	}

	// Create a mock function for jetstream.New
	mockJetstreamNew := func(nc *nats.Conn) (jetstream.JetStream, error) {
		return mockJS, nil
	}

	logs := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.DEBUG)
		client, err := NewNATSClient(conf, logger, metrics, mockNatsConnect, mockJetstreamNew)
		require.NoError(t, err)
		require.NotNil(t, client)

		assert.Equal(t, mockConn, client.conn)
		assert.Equal(t, mockJS, client.js)
		assert.Equal(t, conf, client.config)

	})

	assert.Contains(t, logs, "connecting to NATS server 'nats://localhost:4222'")
	assert.Contains(t, logs, "connected to NATS server 'nats://localhost:4222'")

}

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
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	mockConn := NewMockConnInterface(ctrl)

	conf := &Config{
		Server: "nats://localhost:4222",
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &NATSClient{
		conn:    mockConn,
		js:      mockJS,
		config:  conf,
		logger:  mockLogger,
		metrics: mockMetrics,
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
		Server: "nats://localhost:4222",
		Stream: StreamConfig{
			Stream:   "test-stream",
			Subjects: []string{"test-subject"},
		},
		Consumer: "test-consumer",
	}

	client := &NATSClient{
		conn:    mockConn,
		js:      nil, // Simulate JetStream being nil
		metrics: metrics,
		config:  config,
	}

	ctx := context.TODO()
	subject := "test"
	message := []byte("test-message")

	metrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", subject)

	logs := testutil.StderrOutputForFunc(func() {
		client.logger = logging.NewMockLogger(logging.DEBUG)
		err := client.Publish(ctx, subject, message)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JetStream is not configured or subject is empty")
	})

	assert.Contains(t, logs, "JetStream is not configured or subject is empty")
}

func TestNATSClient_SubscribeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMsgBatch := NewMockMessageBatch(ctrl)
	mockMsg := NewMockMsg(ctrl)
	logger := logging.NewLogger(logging.DEBUG)
	metrics := NewMockMetrics(ctrl)

	client := &NATSClient{
		js:      mockJS,
		logger:  logger,
		metrics: metrics,
		config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockJS.EXPECT().CreateStream(ctx, gomock.Any()).Return(nil, nil)
	mockJS.EXPECT().CreateOrUpdateConsumer(ctx, client.config.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)

	// First call to Fetch returns the message batch
	mockConsumer.EXPECT().Fetch(client.config.BatchSize, gomock.Any()).Return(mockMsgBatch, nil).Times(1)
	// Subsequent calls to Fetch return context.Canceled error
	mockConsumer.EXPECT().Fetch(client.config.BatchSize, gomock.Any()).Return(nil, context.Canceled).AnyTimes()

	msgChan := make(chan jetstream.Msg, 1)
	msgChan <- mockMsg
	close(msgChan)
	mockMsgBatch.EXPECT().Messages().Return(msgChan)
	mockMsgBatch.EXPECT().Error().Return(nil)

	mockMsg.EXPECT().Ack().Return(nil).Times(1)

	var wg sync.WaitGroup
	wg.Add(1)

	/*
		handler := func(ctx *gofr.Context, msg jetstream.Msg) error {
			defer wg.Done()
			cancel() // Cancel the context to stop the consuming loop
			return nil
		}
	*/

	_, err := client.Subscribe(ctx, "test-subject")
	require.NoError(t, err)

	wg.Wait()
}

func TestNATSClient_SubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewLogger(logging.DEBUG)
	metrics := NewMockMetrics(ctrl)

	client := &NATSClient{
		js:      mockJS,
		logger:  logger,
		metrics: metrics,
		config: &Config{
			Stream: StreamConfig{
				Stream:   "test-stream",
				Subjects: []string{"test-subject"},
			},
			Consumer: "test-consumer",
		},
	}

	ctx := context.TODO()

	mockJS.EXPECT().CreateStream(ctx, gomock.Any()).Return(nil, errors.New("failed to create stream"))

	/*
		handler := func(ctx *gofr.Context, msg jetstream.Msg) error {
			return nil
		}
	*/

	// err := client.Subscribe(ctx, "test-subject", handler)
	_, err := client.Subscribe(ctx, "test-subject")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create stream")
}

// Mock method to simulate message handling
func (n *NATSClient) handleMessage(msg jetstream.Msg) {
	ctx := &gofr.Context{Context: context.Background()}
	if n.subscriptions["test-subject"] != nil {
		_ = n.subscriptions["test-subject"].handler(ctx, msg)
	}
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	mockConn := NewMockConnInterface(ctrl)

	client := &NATSClient{
		conn:    mockConn,
		js:      mockJS,
		logger:  mockLogger,
		metrics: mockMetrics,
		config: &Config{
			Stream: StreamConfig{
				Stream: "test-stream",
			},
		},
		subscriptions: map[string]*subscription{
			"test-subject": {
				cancel: func() {},
			},
		},
	}

	mockConn.EXPECT().Close()

	err := client.Close()
	require.NoError(t, err)
	assert.Empty(t, client.subscriptions)
}

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, err := RunEmbeddedNATSServer()
	require.NoError(t, err)
	defer s.Shutdown()

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)

	var testCases []struct {
		name            string
		config          Config
		natsConnectFunc func(string, ...nats.Option) (*nats.Conn, error)
		expectErr       bool
		expectedLog     string
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			tc.config.Server = s.ClientURL() // Use the embedded server's URL

			logs := testutil.StdoutOutputForFunc(func() {
				client, err := New(&tc.config, mockLogger, mockMetrics)
				if tc.expectErr {
					assert.Nil(t, client)
					assert.Error(t, err)
				} else {
					assert.NotNil(t, client)
					assert.NoError(t, err)
				}
			})

			assert.Contains(t, logs, tc.expectedLog)
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
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := &NATSClient{
		js:     mockJS,
		logger: mockLogger,
		config: &Config{
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
	client.config.Stream.Stream = "test-stream"

	logs := testutil.StdoutOutputForFunc(func() {
		client.logger = logging.NewMockLogger(logging.DEBUG)
		err := client.CreateStream(ctx, client.config.Stream)
		require.NoError(t, err)
	})

	assert.Contains(t, logs, "Creating stream")
	assert.Contains(t, logs, "test-stream")
}
