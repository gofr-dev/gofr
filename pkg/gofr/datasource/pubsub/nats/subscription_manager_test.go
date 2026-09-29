package nats

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
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

func TestSubscriptionManager_createPubSubMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})

	mockMsg := NewMockMsg(ctrl)
	mockMsg.EXPECT().Data().Return([]byte("test message"))
	mockMsg.EXPECT().Headers().Return(nil).Times(2)

	sm := newSubscriptionManager(1)
	ctx := t.Context()
	topic := "test.topic"

	msg := sm.createPubSubMessage(ctx, mockMsg, topic)

	require.NotNil(t, msg)
	assert.Equal(t, topic, msg.Topic)
	assert.Equal(t, []byte("test message"), msg.Value)

	// Verify the message context carries a valid span.
	require.NotNil(t, msg.Context())

	// Verify the committer holds the subscribe span and ends it on Commit.
	committer, ok := msg.Committer.(*natsCommitter)
	require.True(t, ok, "Committer should be a *natsCommitter")
	assert.NotNil(t, committer.span)
	assert.True(t, committer.span.SpanContext().IsValid())
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

func TestSubscriptionManager_Subscribe_EarlyReturns(t *testing.T) {
	topic := "test.topic"
	cfg := &Config{Consumer: "test-consumer", Stream: StreamConfig{Stream: "test-stream"}, MaxWait: time.Second}

	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name       string
		ctx        context.Context
		jetStream  func(js *MockJetStream) jetstream.JetStream
		setupMocks func(js *MockJetStream, cons *MockConsumer)
		expErr     error
	}{
		{
			name:       "jetstream missing",
			ctx:        t.Context(),
			jetStream:  func(*MockJetStream) jetstream.JetStream { return nil },
			setupMocks: func(*MockJetStream, *MockConsumer) {},
			expErr:     errJetStreamNotConfigured,
		},
		{
			name:      "context canceled before a message arrives",
			ctx:       canceled,
			jetStream: func(js *MockJetStream) jetstream.JetStream { return js },
			setupMocks: func(js *MockJetStream, cons *MockConsumer) {
				js.EXPECT().CreateOrUpdateConsumer(gomock.Any(), "test-stream", gomock.Any()).Return(cons, nil)
				cons.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(nil, context.Canceled).AnyTimes()
			},
			expErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			js := NewMockJetStream(ctrl)
			cons := NewMockConsumer(ctrl)
			metrics := NewMockMetrics(ctrl)

			tt.setupMocks(js, cons)
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)

			sm := newSubscriptionManager(1)
			defer sm.Close()

			msg, err := sm.Subscribe(tt.ctx, topic, tt.jetStream(js), cfg, logging.NewMockLogger(logging.DEBUG), metrics)

			assert.Nil(t, msg)
			require.ErrorIs(t, err, tt.expErr)
		})
	}
}

func TestSubscriptionManager_handleFetchError(t *testing.T) {
	topic := "test.topic"

	tests := []struct {
		name      string
		err       error
		expStderr string
	}{
		{
			name:      "deadline exceeded is expected and not logged",
			err:       context.DeadlineExceeded,
			expStderr: "",
		},
		{
			name:      "other errors are logged",
			err:       errSubscriptionError,
			expStderr: "Error fetching messages for topic test.topic: " + errSubscriptionError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := newSubscriptionManager(1)

			var err error

			stderr := testutil.StderrOutputForFunc(func() {
				err = sm.handleFetchError(tt.err, topic, logging.NewMockLogger(logging.DEBUG))
			})

			require.NoError(t, err, "fetch errors are swallowed so the consume loop keeps running")
			assert.Contains(t, stderr, tt.expStderr)
			assert.Equal(t, tt.expStderr == "", stderr == "")
		})
	}
}

func TestSubscriptionManager_processFetchedMessages(t *testing.T) {
	topic := "test.topic"

	tests := []struct {
		name       string
		bufferSize int
		batchErr   error
		expErr     error
		expStdout  string
		expStderr  string
		expQueued  int
	}{
		{
			name:       "message queued",
			bufferSize: 1,
			expErr:     nil,
			expQueued:  1,
		},
		{
			name:       "full buffer drops the message",
			bufferSize: 0,
			expErr:     nil,
			expStdout:  "Message buffer is full for topic test.topic",
			expQueued:  0,
		},
		{
			name:       "batch error is returned",
			bufferSize: 1,
			batchErr:   errSubscriptionError,
			expErr:     errSubscriptionError,
			expStderr:  "Error in message batch for topic test.topic",
			expQueued:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			msg := NewMockMsg(ctrl)
			msg.EXPECT().Data().Return([]byte("payload")).AnyTimes()
			msg.EXPECT().Headers().Return(nil).AnyTimes()

			msgChan := make(chan jetstream.Msg, 1)
			msgChan <- msg

			close(msgChan)

			batch := NewMockMessageBatch(ctrl)
			batch.EXPECT().Messages().Return(msgChan)
			batch.EXPECT().Error().Return(tt.batchErr)

			sm := newSubscriptionManager(tt.bufferSize)
			buffer := make(chan *pubsub.Message, tt.bufferSize)

			var err error

			var stdout string

			stderr := testutil.StderrOutputForFunc(func() {
				stdout = testutil.StdoutOutputForFunc(func() {
					err = sm.processFetchedMessages(t.Context(), batch, topic, buffer, logging.NewMockLogger(logging.DEBUG))
				})
			})

			require.ErrorIs(t, err, tt.expErr)
			assert.Contains(t, stdout, tt.expStdout)
			assert.Contains(t, stderr, tt.expStderr)
			assert.Len(t, buffer, tt.expQueued)
		})
	}
}

// TestSubscriptionManager_consumeMessages_LogsBatchError runs the loop synchronously: the fetch
// cancels the context, so the loop logs the batch error once and then exits on its own.
func TestSubscriptionManager_consumeMessages_LogsBatchError(t *testing.T) {
	ctrl := gomock.NewController(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	batch := NewMockMessageBatch(ctrl)
	batch.EXPECT().Messages().Return(closedMsgChan())
	batch.EXPECT().Error().Return(errSubscriptionError)

	cons := NewMockConsumer(ctrl)
	cons.EXPECT().Fetch(1, gomock.Any()).DoAndReturn(func(int, ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
		cancel()

		return batch, nil
	})

	sm := newSubscriptionManager(1)

	stderr := testutil.StderrOutputForFunc(func() {
		sm.consumeMessages(ctx, cons, "test.topic", make(chan *pubsub.Message, 1), &Config{}, logging.NewMockLogger(logging.DEBUG))
	})

	assert.Contains(t, stderr, "Error fetching messages for topic test.topic: "+errSubscriptionError.Error())
}

// TestSubscriptionManager_fetchAndProcessMessages_FetchError pins that a failed fetch is absorbed
// rather than surfaced, so the consume loop keeps polling.
func TestSubscriptionManager_fetchAndProcessMessages_FetchError(t *testing.T) {
	ctrl := gomock.NewController(t)

	cons := NewMockConsumer(ctrl)
	cons.EXPECT().Fetch(1, gomock.Any()).Return(nil, context.DeadlineExceeded)

	sm := newSubscriptionManager(1)

	err := sm.fetchAndProcessMessages(t.Context(), cons, "test.topic", make(chan *pubsub.Message, 1),
		&Config{MaxWait: time.Millisecond}, logging.NewMockLogger(logging.DEBUG))

	require.NoError(t, err)
}

func closedMsgChan() chan jetstream.Msg {
	ch := make(chan jetstream.Msg)
	close(ch)

	return ch
}
