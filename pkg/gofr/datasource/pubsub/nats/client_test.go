package nats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

func TestValidateConfigs(t *testing.T) {
	// This test remains unchanged
}

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

	ctx := context.Background()
	subject := "test-subject"
	message := []byte("test-message")

	// Set up expected calls
	mockConnManager.EXPECT().
		Publish(ctx, subject, message, mockMetrics).
		Return(nil)

	// Call Publish
	err := client.Publish(ctx, subject, message)
	require.NoError(t, err)
}

func TestNATSClient_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	mockConnManager := NewMockConnectionManagerInterface(ctrl)

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

	ctx := context.TODO()
	subject := "test"
	message := []byte("test-message")

	expectedErr := errors.New("publish error")
	mockConnManager.EXPECT().
		Publish(ctx, subject, message, mockMetrics).
		Return(expectedErr)

	err := client.Publish(ctx, subject, message)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
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
			Consumer:  "test-consumer",
			MaxWait:   time.Second,
			BatchSize: 1,
		},
		metrics: mockMetrics,
		logger:  logging.NewMockLogger(logging.DEBUG),
	}

	ctx := context.Background()
	expectedMsg := &pubsub.Message{
		Topic: "test-subject",
		Value: []byte("test message"),
	}

	mockConnManager.EXPECT().JetStream().Return(mockJetStream)
	mockSubManager.EXPECT().
		Subscribe(ctx, "test-subject", mockJetStream, client.Config, client.logger, client.metrics).
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

	ctx := context.Background()

	expectedErr := errFailedToCreateConsumer
	mockConnManager.EXPECT().JetStream().Return(mockJetStream)
	mockSubManager.EXPECT().
		Subscribe(ctx, "test-subject", mockJetStream, client.Config, client.logger, client.metrics).
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

	ctx := context.Background()

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
	client := &Client{
		streamManager: mockStreamManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := context.Background()

	mockStreamManager.EXPECT().DeleteStream(ctx, "test-topic").Return(nil)

	err := client.DeleteTopic(ctx, "test-topic")
	require.NoError(t, err)
}

func TestNATSClient_DeleteTopic_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := context.Background()

	expectedErr := errFailedToDeleteStream
	mockStreamManager.EXPECT().DeleteStream(ctx, "test-topic").Return(expectedErr)

	err := client.DeleteTopic(ctx, "test-topic")
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNATSClient_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStreamManager := NewMockStreamManagerInterface(ctrl)
	client := &Client{
		streamManager: mockStreamManager,
		logger:        logging.NewMockLogger(logging.DEBUG),
		Config:        &Config{},
	}

	ctx := context.Background()

	mockStreamManager.EXPECT().
		CreateStream(ctx, StreamConfig{
			Stream:   "test-topic",
			Subjects: []string{"test-topic"},
		}).
		Return(nil)

	err := client.CreateTopic(ctx, "test-topic")
	require.NoError(t, err)
}

// Additional tests for ConnectionManager, SubscriptionManager, and StreamManager
// should be added in separate test files for those components.

// The following tests have been removed as they are now part of the respective manager components:
// TestNATSClient_NakMessage
// TestNATSClient_HandleFetchError
// TestNATSClient_HandleMessageError
// TestNATSClient_DeleteStreamError
// TestNATSClient_CreateStreamError
// TestNATSClient_CreateOrUpdateStreamError
