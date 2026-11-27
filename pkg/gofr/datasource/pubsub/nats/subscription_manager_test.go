package nats

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/logging"
)

func TestNewSubscriptionManager(t *testing.T) {
	sm := newSubscriptionManager()
	assert.NotNil(t, sm)
	assert.NotNil(t, sm.consumers)
}

func TestSubscriptionManager_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "test-consumer",
		Stream: StreamConfig{
			Stream:     "test-stream",
			MaxDeliver: 3,
		},
		MaxWait: time.Second,
	}

	topic := "test.topic"

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)
	mockConsumer.EXPECT().Fetch(1, gomock.Any()).Return(createMockMessageBatch(ctrl), nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", topic)

	msg, err := sm.Subscribe(t.Context(), topic, mockJS, cfg, mockLogger, mockMetrics)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, topic, msg.Topic)
}

// TestSubscriptionManager_Subscribe_FetchesOnlyWhenCalled pins that each Subscribe
// call triggers exactly one fetch and nothing is fetched into a buffer ahead of the
// handler. A prefetch loop would let fetched messages sit past AckWait and the
// broker would redeliver them while the handler is still working through the queue.
func TestSubscriptionManager_Subscribe_FetchesOnlyWhenCalled(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "test-consumer",
		Stream:   StreamConfig{Stream: "test-stream"},
		MaxWait:  time.Second,
	}

	topic := "test.topic"

	fetches := 0

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic).Times(2)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", topic).Times(2)
	mockConsumer.EXPECT().Fetch(1, gomock.Any()).DoAndReturn(func(int, ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
		fetches++

		return createMockMessageBatch(ctrl), nil
	}).Times(2)

	for range 2 {
		msg, err := sm.Subscribe(t.Context(), topic, mockJS, cfg, mockLogger, mockMetrics)
		require.NoError(t, err)
		require.NotNil(t, msg)
	}

	assert.Equal(t, 2, fetches, "each Subscribe call must fetch exactly one message on demand")
}

func TestSubscriptionManager_Subscribe_PrerequisiteError(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager()
	topic := "test.topic"

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)

	msg, err := sm.Subscribe(t.Context(), topic, nil, &Config{Consumer: "test-consumer"}, mockLogger, mockMetrics)
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, errJetStreamNotConfigured, err)
}

func TestSubscriptionManager_Subscribe_Error(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStream(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "test-consumer",
		Stream: StreamConfig{
			Stream: "test-stream",
		},
	}

	topic := "test.topic"

	expectedErr := errConsumerCreationError
	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(nil, expectedErr)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)

	msg, err := sm.Subscribe(t.Context(), topic, mockJS, cfg, mockLogger, mockMetrics)
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, expectedErr, err)
}

func TestSubscriptionManager_Subscribe_FetchError(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "test-consumer",
		Stream:   StreamConfig{Stream: "test-stream"},
		MaxWait:  time.Second,
	}

	topic := "test.topic"

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)
	mockConsumer.EXPECT().Fetch(1, gomock.Any()).Return(nil, errSubscriptionError)

	msg, err := sm.Subscribe(t.Context(), topic, mockJS, cfg, mockLogger, mockMetrics)
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, errSubscriptionError, err)
}

func TestSubscriptionManager_Subscribe_NoMessage(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "test-consumer",
		Stream:   StreamConfig{Stream: "test-stream"},
		MaxWait:  time.Second,
	}

	topic := "test.topic"

	batch := NewMockMessageBatch(ctrl)
	batch.EXPECT().Messages().Return(closedMsgChan())
	batch.EXPECT().Error().Return(nil)

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)
	mockConsumer.EXPECT().Fetch(1, gomock.Any()).Return(batch, nil)

	msg, err := sm.Subscribe(t.Context(), topic, mockJS, cfg, mockLogger, mockMetrics)
	require.NoError(t, err, "an empty fetch means no message right now, not an error")
	assert.Nil(t, msg)
}

func TestSubscriptionManager_Subscribe_BatchError(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "test-consumer",
		Stream:   StreamConfig{Stream: "test-stream"},
		MaxWait:  time.Second,
	}

	topic := "test.topic"

	batch := NewMockMessageBatch(ctrl)
	batch.EXPECT().Messages().Return(closedMsgChan())
	batch.EXPECT().Error().Return(errSubscriptionError)

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", topic)
	mockConsumer.EXPECT().Fetch(1, gomock.Any()).Return(batch, nil)

	msg, err := sm.Subscribe(t.Context(), topic, mockJS, cfg, mockLogger, mockMetrics)
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, errSubscriptionError, err)
}

func TestSubscriptionManager_validateSubscribePrerequisites(t *testing.T) {
	sm := newSubscriptionManager()
	mockJS := NewMockJetStream(gomock.NewController(t))
	cfg := &Config{Consumer: "test-consumer"}

	err := sm.validateSubscribePrerequisites(mockJS, cfg)
	require.NoError(t, err)

	err = sm.validateSubscribePrerequisites(nil, cfg)
	assert.Equal(t, errJetStreamNotConfigured, err)

	err = sm.validateSubscribePrerequisites(mockJS, &Config{})
	assert.Equal(t, errConsumerNotProvided, err)
}

func TestSubscriptionManager_consumerFor(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "test-consumer",
		Stream:   StreamConfig{Stream: "test-stream"},
	}

	topic := "test.topic"

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)

	cons, err := sm.consumerFor(t.Context(), mockJS, topic, cfg)
	require.NoError(t, err)
	assert.Equal(t, mockConsumer, cons)

	// The second call reuses the cached consumer instead of creating a new one.
	cached, err := sm.consumerFor(t.Context(), mockJS, topic, cfg)
	require.NoError(t, err)
	assert.Equal(t, mockConsumer, cached)
}

func TestSubscriptionManager_createOrUpdateConsumer(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStream(ctrl)
	mockConsumer := NewMockConsumer(ctrl)

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "test-consumer",
		Stream: StreamConfig{
			Stream:     "test-stream",
			MaxDeliver: 3,
		},
	}

	topic := "test.topic"

	mockJS.EXPECT().CreateOrUpdateConsumer(gomock.Any(), cfg.Stream.Stream, gomock.Any()).Return(mockConsumer, nil)

	consumer, err := sm.createOrUpdateConsumer(t.Context(), mockJS, topic, cfg)
	require.NoError(t, err)
	assert.Equal(t, mockConsumer, consumer)
}

func TestSubscriptionManager_createPubSubMessage(t *testing.T) {
	ctrl := gomock.NewController(t)

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

	sm := newSubscriptionManager()
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
	sm := newSubscriptionManager()
	sm.consumers["test.topic"] = NewMockConsumer(gomock.NewController(t))

	sm.Close()

	assert.Empty(t, sm.consumers)
}

// TestSubscriptionManager_Subscribe_DoesNotPrefetch drives a real JetStream server
// and pins the redelivery fix: only the message a Subscribe call asks for is
// fetched, so nothing can sit past AckWait and get redelivered while the handler
// works through a backlog.
func TestSubscriptionManager_Subscribe_DoesNotPrefetch(t *testing.T) {
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1, // Random available port
		JetStream: true,
		StoreDir:  t.TempDir(),
	}

	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()
	defer ns.Shutdown()

	require.True(t, ns.ReadyForConnections(10*time.Second), "NATS server not ready for connections")

	addr := ns.Addr().(*net.TCPAddr)

	nc, err := nats.Connect(fmt.Sprintf("nats://%s:%d", addr.IP.String(), addr.Port))
	require.NoError(t, err)

	defer nc.Close()

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	ctx := t.Context()

	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "prefetch-stream",
		Subjects: []string{"prefetch.>"},
	})
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	metrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	sm := newSubscriptionManager()
	cfg := &Config{
		Consumer: "prefetch-consumer",
		Stream:   StreamConfig{Stream: "prefetch-stream"},
		MaxWait:  500 * time.Millisecond,
	}
	topic := "prefetch.orders"

	// The consumer is created with DeliverNewPolicy, so it only sees what is
	// published after it exists.
	cons, err := sm.consumerFor(ctx, js, topic, cfg)
	require.NoError(t, err)

	const total = 5

	for i := 0; i < total; i++ {
		_, err = js.Publish(ctx, topic, []byte(fmt.Sprintf("msg-%d", i)))
		require.NoError(t, err)
	}

	msg, err := sm.Subscribe(ctx, topic, js, cfg, logging.NewMockLogger(logging.DEBUG), metrics)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, []byte("msg-0"), msg.Value)

	// The old prefetch loop had drained the whole stream into its buffer by this
	// point. Nothing beyond the requested message may be in flight, otherwise
	// its AckWait starts running against the handler.
	time.Sleep(300 * time.Millisecond)

	info, err := cons.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(total-1), info.NumPending, "messages beyond the requested one must not be fetched")
	assert.Equal(t, 1, info.NumAckPending, "only the requested message is delivered")

	// The rest arrives one per Subscribe call, each exactly once.
	seen := make(map[string]int, total)

	for i := 1; i < total; i++ {
		m, err := sm.Subscribe(ctx, topic, js, cfg, logging.NewMockLogger(logging.DEBUG), metrics)
		require.NoError(t, err)
		require.NotNil(t, m)

		seen[string(m.Value)]++
		m.Committer.Commit()
	}

	for i := 1; i < total; i++ {
		assert.Equal(t, 1, seen[fmt.Sprintf("msg-%d", i)], "msg-%d must be delivered exactly once", i)
	}

	msg.Committer.Commit()
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

func closedMsgChan() chan jetstream.Msg {
	ch := make(chan jetstream.Msg)
	close(ch)

	return ch
}
