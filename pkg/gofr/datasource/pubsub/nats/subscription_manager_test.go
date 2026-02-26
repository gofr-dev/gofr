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
)

func TestNewSubscriptionManager(t *testing.T) {
	sm := newSubscriptionManager(100)
	assert.NotNil(t, sm)
	assert.Equal(t, 100, sm.bufferSize)
	assert.NotNil(t, sm.subscriptions)
	assert.NotNil(t, sm.topicBuffers)
}

func TestSubscriptionManager_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager(1)
	cfg := &Config{
		Consumer: "test-consumer",
		Stream: StreamConfig{
			Stream:     "test-stream",
			MaxDeliver: 3,
		},
		MaxWait: time.Second,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	topic := "test.topic"

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)
	mockConsumer.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(createMockMessageBatch(ctrl), nil).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", topic)

	msg, err := sm.Subscribe(ctx, topic, mockJS, cfg, mockLogger, mockMetrics)
	require.NoError(t, err)
	assert.NotNil(t, msg)
	assert.Equal(t, topic, msg.Topic)
}

func TestSubscriptionManager_Subscribe_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager(1)
	cfg := &Config{
		Consumer: "test-consumer",
		Stream: StreamConfig{
			Stream: "test-stream",
		},
	}

	ctx := t.Context()
	topic := "test.topic"

	expectedErr := errConsumerCreationError
	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(nil, expectedErr)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)

	msg, err := sm.Subscribe(ctx, topic, mockJS, cfg, mockLogger, mockMetrics)
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, expectedErr, err)
}

func TestSubscriptionManager_validateSubscribePrerequisites(t *testing.T) {
	sm := newSubscriptionManager(1)
	mockJS := NewMockJetStream(gomock.NewController(t))
	cfg := &Config{Consumer: "test-consumer"}

	err := sm.validateSubscribePrerequisites(mockJS, cfg)
	require.NoError(t, err)

	err = sm.validateSubscribePrerequisites(nil, cfg)
	assert.Equal(t, errJetStreamNotConfigured, err)

	err = sm.validateSubscribePrerequisites(mockJS, &Config{})
	assert.Equal(t, errConsumerNotProvided, err)
}

func TestSubscriptionManager_getOrCreateBuffer(t *testing.T) {
	sm := newSubscriptionManager(1)
	topic := "test.topic"

	buffer := sm.getOrCreateBuffer(topic)
	assert.NotNil(t, buffer)
	assert.Empty(t, buffer)
	assert.Equal(t, 1, cap(buffer))

	// Check that the same buffer is returned for the same topic
	sameBuffer := sm.getOrCreateBuffer(topic)
	assert.Equal(t, buffer, sameBuffer)
}

func TestSubscriptionManager_createOrUpdateConsumer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)

	sm := newSubscriptionManager(1)
	cfg := &Config{
		Consumer: "test-consumer",
		Stream: StreamConfig{
			Stream:     "test-stream",
			MaxDeliver: 3,
		},
	}

	ctx := t.Context()
	topic := "test.topic"

	mockJS.EXPECT().CreateOrUpdateConsumer(ctx, cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)

	consumer, err := sm.createOrUpdateConsumer(ctx, mockJS, topic, cfg)
	require.NoError(t, err)
	assert.Equal(t, mockConsumer, consumer)
}

func TestSubscriptionManager_consumeMessages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConsumer := NewMockConsumer(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager(1)
	cfg := &Config{MaxWait: time.Second}
	topic := "test.topic"
	buffer := make(chan *pubsub.Message, 1)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockBatch := createMockMessageBatch(ctrl)
	mockConsumer.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(mockBatch, nil).AnyTimes()

	go sm.consumeMessages(ctx, mockConsumer, topic, buffer, cfg, mockLogger)

	select {
	case msg := <-buffer:
		assert.NotNil(t, msg)
		assert.Equal(t, topic, msg.Topic)
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for message")
	}
}

func createMockMessageBatch(ctrl *gomock.Controller) jetstream.MessageBatch {
	mockBatch := NewMockMessageBatch(ctrl)
	mockMsg := NewMockMsg(ctrl)

	mockMsg.EXPECT().Data().Return([]byte("test message")).AnyTimes()
	mockMsg.EXPECT().Headers().Return(nil).AnyTimes()

	msgChan := make(chan jetstream.Msg, 1)
	msgChan <- mockMsg

	close(msgChan)

	mockBatch.EXPECT().Messages().Return(msgChan).AnyTimes()
	mockBatch.EXPECT().Error().Return(nil).AnyTimes()

	return mockBatch
}

func TestSubscriptionManager_Close(t *testing.T) {
	sm := newSubscriptionManager(1)
	topic := "test.topic"

	// Create a subscription and buffer
	ctx, cancel := context.WithCancel(t.Context())
	sm.subscriptions[topic] = &subscription{cancel: cancel}
	sm.topicBuffers[topic] = make(chan *pubsub.Message, 1)

	sm.Close()

	assert.Empty(t, sm.subscriptions)
	assert.Empty(t, sm.topicBuffers)

	// Check that the context was canceled
	if ctx.Err() == nil {
		t.Fatal("Context was not canceled")
	}
}
