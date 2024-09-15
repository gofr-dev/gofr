package nats_test

import (
	"context"
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
		assert.Contains(t, err.Error(), "JetStream is not configured or subject is empty")
	})

	assert.Contains(t, logs, "JetStream is not configured or subject is empty")
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

	mockJS.EXPECT().CreateStream(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), client.Config.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)

	mockConsumer.EXPECT().Fetch(client.Config.BatchSize, gomock.Any()).Return(mockMsgBatch, nil).Times(1)
	mockConsumer.EXPECT().Fetch(client.Config.BatchSize, gomock.Any()).Return(nil, context.Canceled).AnyTimes()

	msgChan := make(chan jetstream.Msg, 1)

	mockMsg.EXPECT().Data().Return([]byte("test message")).AnyTimes()
	mockMsg.EXPECT().Subject().Return("test-subject").AnyTimes()

	msgChan <- mockMsg
	close(msgChan)

	mockMsgBatch.EXPECT().Messages().Return(msgChan)
	mockMsgBatch.EXPECT().Error().Return(nil).AnyTimes()

	mockMsg.EXPECT().Ack().Return(nil).AnyTimes()

	receivedMsg := make(chan *pubsub.Message, 1)

	go func() {
		msg, err := client.Subscribe(ctx, "test-subject")
		if err == nil {
			receivedMsg <- msg
		}
	}()

	select {
	case msg := <-receivedMsg:
		assert.Equal(t, "test-subject", msg.Topic)
		assert.Equal(t, []byte("test message"), msg.Value)
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
	mockJS.EXPECT().CreateStream(ctx, gomock.Any()).Return(nil, expectedErr)

	var err error

	logs := testutil.StderrOutputForFunc(func() {
		client.Logger = logging.NewMockLogger(logging.DEBUG)
		_, err = client.Subscribe(ctx, "test-subject")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create stream")
	assert.Contains(t, logs, "failed to create or update stream: failed to create stream")
}

// natsClient is a local receiver, which is used to test the NATS client.
//
//nolint:unused // used for testing
type natsClient struct {
	*natspubsub.NATSClient
}

// Mock method to simulate message handling
/*
func (n *natsClient) handleMessage(msg jetstream.Msg) {
	ctx := &gofr.Context{Context: context.Background()}
	if n.Subscriptions["test-subject"] != nil {
		_ = n.Subscriptions["test-subject"].Handler(ctx, msg)
	}
}
*/

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

	s, err := natspubsub.RunEmbeddedNATSServer()
	require.NoError(t, err)
	defer s.Shutdown()

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := natspubsub.NewMockMetrics(ctrl)

	var testCases []struct {
		name            string
		config          natspubsub.Config
		natsConnectFunc func(string, ...nats.Option) (*nats.Conn, error)
		expectErr       bool
		expectedLog     string
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			tc.config.Server = s.ClientURL() // Use the embedded server's URL

			logs := testutil.StdoutOutputForFunc(func() {
				client, err := natspubsub.New(&tc.config, mockLogger, mockMetrics)
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
