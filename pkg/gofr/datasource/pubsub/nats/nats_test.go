package nats

import (
	"context"
	"fmt"
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
	// Start embedded NATS server
	s, err := RunEmbeddedNATSServer()
	require.NoError(t, err)
	defer s.Shutdown()

	conf := &Config{
		Server: s.ClientURL(),
		Stream: StreamConfig{
			Stream:  "test-stream",
			Subject: "test-subject",
		},
		Consumer: "test-consumer",
	}

	// setup mock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnInterface(ctrl)
	mockJS := NewMockJetStream(ctrl)

	mockConn.EXPECT().JetStream(gomock.Any()).Return(mockJS, nil)

	logger := logging.NewMockLogger(logging.DEBUG)
	metrics := NewMockMetrics(ctrl)

	natsConnect := func(serverURL string, opts ...nats.Option) (*nats.Conn, error) {
		return nats.Connect(serverURL, opts...)
	}

	client, err := New(conf, logger, metrics, natsConnect)
	require.NoError(t, err)
	require.NotNil(t, client)
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

	// Mock jetstream.JetStream
	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)

	// Create an embedded NATS server
	s, err := RunEmbeddedNATSServer()
	require.NoError(t, err)
	defer s.Shutdown()

	// Connect to the embedded NATS server
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)

	conf := &Config{
		Server: s.ClientURL(),
		Stream: StreamConfig{
			Stream:  "test-stream",
			Subject: "test-subject",
		},
		Consumer: "test-consumer",
	}

	client := &NATSClient{
		conn:    nc,
		js:      mockJS,
		config:  conf,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	// Set up expected calls
	mockMetrics.EXPECT().
		IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "subject", "test-subject")
	mockJS.EXPECT().
		Publish(gomock.Any(), "test-subject", []byte("test-message")).
		Return(&jetstream.PubAck{}, nil)
	mockMetrics.EXPECT().
		IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "subject", "test-subject")

	// Call Publish
	err = client.Publish(context.Background(), "test-subject", []byte("test-message"))
	require.NoError(t, err)
}

func TestNATSClient_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := logging.NewMockLogger(logging.DEBUG)
	metrics := NewMockMetrics(ctrl)
	client := &NATSClient{
		js:      nil, // Simulate JetStream being nil
		logger:  logger,
		metrics: metrics,
	}

	ctx := context.TODO()

	metrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", "test")

	err := client.Publish(ctx, "test", []byte("test-message"))
	require.Error(t, err)
}

func TestNATSClient_SubscribeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	mockJS := NewMockJetStream(ctrl)
	mockStream := NewMockStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMsgBatch := NewMockMessageBatch(ctrl)
	mockMsg := NewMockMsg(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	// Create the client directly
	client := &NATSClient{
		conn:   nil,
		js:     mockJS,
		mu:     &sync.RWMutex{},
		logger: mockLogger,
		config: &Config{
			Server: "nats://localhost:4222",
			Stream: StreamConfig{
				Stream:  "test-stream",
				Subject: "test-subject",
			},
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
		metrics:       mockMetrics,
		subscriptions: make(map[string]*subscription),
	}

	// Set up the handler function
	messageReceived := make(chan jetstream.Msg, 1)
	handler := func(ctx *gofr.Context, msg jetstream.Msg) error {
		messageReceived <- msg
		return nil
	}

	ctx := context.Background()

	// Set up mocks and expectations
	// Mock Stream retrieval
	mockJS.EXPECT().
		Stream(ctx, "test-stream").
		Return(nil, jetstream.ErrStreamNotFound)

	mockJS.EXPECT().
		CreateStream(ctx, gomock.Any()).
		Return(mockStream, nil)

	mockStream.EXPECT().
		CreateOrUpdateConsumer(ctx, gomock.Any()).
		Return(mockConsumer, nil)

	// Mock Fetch
	mockConsumer.EXPECT().
		Fetch(client.config.BatchSize, jetstream.FetchMaxWait(client.config.MaxWait)).
		Return(mockMsgBatch, nil)

	// Mock message batch
	msgChan := make(chan jetstream.Msg, 1)
	msgChan <- mockMsg
	close(msgChan)
	mockMsgBatch.EXPECT().Messages().Return(msgChan)
	mockMsgBatch.EXPECT().Error().Return(nil)

	// Mock message
	mockMsg.EXPECT().Data().Return([]byte("hello")).AnyTimes()
	mockMsg.EXPECT().Subject().Return("test-subject").AnyTimes()
	mockMsg.EXPECT().Headers().Return(nats.Header{}).AnyTimes()
	mockMsg.EXPECT().Ack().Return(nil).AnyTimes()

	// Call Subscribe
	err := client.Subscribe(ctx, "test-subject", handler)
	require.NoError(t, err)

	// Wait for the message to be received
	select {
	case msg := <-messageReceived:
		assert.Equal(t, []byte("hello"), msg.Data())
		assert.Equal(t, "test-subject", msg.Subject())
	case <-time.After(1 * time.Second):
		t.Fatal("Did not receive message in time")
	}
}

// Mock method to simulate message handling
func (n *NATSClient) handleMessage(msg jetstream.Msg) {
	ctx := &gofr.Context{Context: context.Background()}
	if n.subscriptions["test-subject"] != nil {
		_ = n.subscriptions["test-subject"].handler(ctx, msg)
	}
}
func TestNATSClient_SubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockStream := NewMockStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	client := &NATSClient{
		conn:   nil,
		js:     mockJS,
		mu:     &sync.RWMutex{},
		logger: mockLogger,
		config: &Config{
			Server: "nats://localhost:4222",
			Stream: StreamConfig{
				Stream:  "test-stream",
				Subject: "test-subject",
			},
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
		metrics:       mockMetrics,
		subscriptions: make(map[string]*subscription),
	}

	ctx := context.TODO()

	// Set up mocks and expectations
	mockJS.EXPECT().
		Stream(ctx, "test-stream").
		Return(mockStream, nil)

	mockStream.EXPECT().
		CreateOrUpdateConsumer(ctx, gomock.Any()).
		Return(nil, fmt.Errorf("consumer creation error"))

	// Call Subscribe
	err := client.Subscribe(ctx, "test-subject", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "consumer creation error")
}

func TestNATSClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)

	client := &NATSClient{
		conn:    nil,
		js:      mockJS,
		logger:  mockLogger,
		metrics: mockMetrics,
		config: &Config{
			Stream: StreamConfig{
				Stream:  "test-stream",
				Subject: "test-subject",
			},
		},
		subscriptions: map[string]*subscription{
			"test-subject": {
				cancel: func() {},
			},
		},
	}

	ctx := context.TODO()

	mockJS.EXPECT().DeleteStream(ctx, "test-stream").Return(nil)

	err := client.Close(ctx)
	require.NoError(t, err)
}

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, err := RunEmbeddedNATSServer()
	require.NoError(t, err)
	defer s.Shutdown()

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)
	// mockJS := NewMockJetStream(ctrl)

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
				client, err := New(&tc.config, mockLogger, mockMetrics, tc.natsConnectFunc)
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
	mockLogger := logging.NewMockLogger(logging.INFO)
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
		err := client.CreateStream(ctx, client.config.Stream)
		require.NoError(t, err)
	})

	assert.Contains(t, logs, "Creating stream")
	assert.Contains(t, logs, "test-stream")
}
