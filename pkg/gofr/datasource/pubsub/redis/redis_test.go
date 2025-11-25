package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.opentelemetry.io/otel/trace/noop"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

var (
	//nolint:gochecknoglobals // used for testing purposes only
	testMessage = []byte("test message")
	testTopic   = "test-topic"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "with config",
			cfg:  DefaultConfig(),
		},
		{
			name: "nil config",
			cfg:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(tt.cfg)
			assert.NotNil(t, client)
			assert.NotNil(t, client.cfg)
			assert.NotNil(t, client.receiveChan)
			assert.NotNil(t, client.subStarted)
			assert.NotNil(t, client.subCancel)
			assert.NotNil(t, client.subPubSub)
			assert.NotNil(t, client.subWg)
			assert.NotNil(t, client.chanClosed)
		})
	}
}

func TestUseLogger(t *testing.T) {
	client := New(DefaultConfig())

	// Test with redis.Logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	client.UseLogger(mockLogger)
	assert.NotNil(t, client.logger)

	// Test with pubsub.Logger
	mockPubSubLogger := &mockPubSubLogger{}
	client.UseLogger(mockPubSubLogger)
	assert.NotNil(t, client.logger)
}

func TestUseMetrics(t *testing.T) {
	client := New(DefaultConfig())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)
	assert.Equal(t, mockMetrics, client.metrics)
}

func TestUseTracer(t *testing.T) {
	client := New(DefaultConfig())

	tracer := noop.NewTracerProvider().Tracer("test")
	client.UseTracer(tracer)
	assert.NotNil(t, client.tracer)
}

func TestConnect_Success(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)

	client.Connect()

	// Give some time for connection
	time.Sleep(100 * time.Millisecond)

	assert.NotNil(t, client.pubConn)
	assert.NotNil(t, client.subConn)
	assert.NotNil(t, client.queryConn)
}

func TestConnect_InvalidConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockLogger.EXPECT().Errorf("could not initialize Redis, error: %v", gomock.Any())

	cfg := &Config{
		Addr: "", // Invalid address
		DB:   -1, // Invalid DB
	}

	client := New(cfg)
	client.UseLogger(mockLogger)

	client.Connect()

	assert.Nil(t, client.pubConn)
	assert.Nil(t, client.subConn)
}

func TestPublish_Success(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)

	client.Connect()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", testTopic)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", testTopic)

	err = client.Publish(ctx, testTopic, testMessage)
	require.NoError(t, err)
}

func TestPublish_NoConnection(t *testing.T) {
	client := New(DefaultConfig())

	ctx := context.Background()
	err := client.Publish(ctx, testTopic, testMessage)
	require.Error(t, err)
	assert.Equal(t, errPublisherNotConfigured, err)
}

func TestPublish_EmptyTopic(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	err = client.Publish(ctx, "", testMessage)
	require.Error(t, err)
	assert.Equal(t, errPublisherNotConfigured, err)
}

func TestSubscribe_Success(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)

	client.Connect()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", testTopic).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", testTopic).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", testTopic).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", testTopic).AnyTimes()

	// Publish a message in a goroutine
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = client.Publish(context.Background(), testTopic, testMessage)
	}()

	msg, err := client.Subscribe(ctx, testTopic)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, testTopic, msg.Topic)
	assert.Equal(t, testMessage, msg.Value)
}

func TestSubscribe_NoConnection(t *testing.T) {
	client := New(DefaultConfig())

	ctx := context.Background()
	_, err := client.Subscribe(ctx, testTopic)
	require.Error(t, err)
	assert.Equal(t, errClientNotConnected, err)
}

func TestSubscribe_EmptyTopic(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	_, err = client.Subscribe(ctx, "")
	require.Error(t, err)
	assert.Equal(t, errEmptyTopicName, err)
}

func TestUnsubscribe_Success(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)

	client.Connect()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Subscribe first
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", testTopic).AnyTimes()

	// Start subscription
	go func() {
		_, _ = client.Subscribe(ctx, testTopic)
	}()

	time.Sleep(200 * time.Millisecond)

	// Unsubscribe
	err = client.Unsubscribe(testTopic)
	require.NoError(t, err)

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)
}

func TestUnsubscribe_NoSubscription(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	// Unsubscribe from non-existent subscription
	err = client.Unsubscribe("non-existent-topic")
	require.NoError(t, err) // Should not error, just return
}

func TestUnsubscribe_NoConnection(t *testing.T) {
	client := New(DefaultConfig())

	err := client.Unsubscribe(testTopic)
	require.Error(t, err)
	assert.Equal(t, errClientNotConnected, err)
}

func TestUnsubscribe_EmptyTopic(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	err = client.Unsubscribe("")
	require.Error(t, err)
	assert.Equal(t, errEmptyTopicName, err)
}

func TestQuery_Success(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()

	// Publish messages
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = client.Publish(ctx, testTopic, []byte("message1"))
		_ = client.Publish(ctx, testTopic, []byte("message2"))
	}()

	// Query with timeout and limit
	result, err := client.Query(ctx, testTopic, 2*time.Second, 2)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestQuery_NoConnection(t *testing.T) {
	client := New(DefaultConfig())

	ctx := context.Background()
	_, err := client.Query(ctx, testTopic)
	require.Error(t, err)
	assert.Equal(t, errClientNotConnected, err)
}

func TestQuery_EmptyTopic(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	_, err = client.Query(ctx, "")
	require.Error(t, err)
	assert.Equal(t, errEmptyTopicName, err)
}

func TestHealth_Up(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	health := client.Health()
	assert.Equal(t, "UP", health.Status)
	assert.Equal(t, "REDIS", health.Details["backend"])
	assert.Equal(t, s.Addr(), health.Details["addr"])
}

func TestHealth_Down(t *testing.T) {
	client := New(DefaultConfig())

	health := client.Health()
	assert.Equal(t, "DOWN", health.Status)
	assert.Equal(t, "REDIS", health.Details["backend"])
}

func TestCreateTopic(t *testing.T) {
	client := New(DefaultConfig())

	ctx := context.Background()
	err := client.CreateTopic(ctx, testTopic)
	require.NoError(t, err) // Should be no-op for Redis
}

func TestDeleteTopic(t *testing.T) {
	client := New(DefaultConfig())

	ctx := context.Background()
	err := client.DeleteTopic(ctx, testTopic)
	require.NoError(t, err) // Should be no-op for Redis
}

func TestClose(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	// Subscribe to create some state
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", testTopic).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", testTopic).AnyTimes()

	go func() {
		_, _ = client.Subscribe(ctx, testTopic)
	}()

	time.Sleep(200 * time.Millisecond)

	// Close
	err = client.Close()
	require.NoError(t, err)

	// Verify connections are closed
	time.Sleep(100 * time.Millisecond)
}

func TestClose_NoConnections(t *testing.T) {
	client := New(DefaultConfig())

	err := client.Close()
	require.NoError(t, err)
}

func TestRestartSubscriptions(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)

	client.Connect()
	time.Sleep(100 * time.Millisecond)

	// Manually set up subscription state
	client.mu.Lock()
	client.subStarted[testTopic] = struct{}{}
	client.receiveChan[testTopic] = make(chan *pubsub.Message, 10)
	_, cancel := context.WithCancel(context.Background())
	client.subCancel[testTopic] = cancel
	client.mu.Unlock()

	// Restart subscriptions
	client.restartSubscriptions()

	// Verify state is cleared
	client.mu.RLock()
	_, exists := client.subStarted[testTopic]
	client.mu.RUnlock()

	assert.False(t, exists)
}

// mockPubSubLogger is a mock implementation of pubsub.Logger
type mockPubSubLogger struct{}

func (m *mockPubSubLogger) Debugf(format string, args ...any) {}
func (m *mockPubSubLogger) Debug(args ...any)                  {}
func (m *mockPubSubLogger) Logf(format string, args ...any)   {}
func (m *mockPubSubLogger) Log(args ...any)                   {}
func (m *mockPubSubLogger) Errorf(format string, args ...any) {}
func (m *mockPubSubLogger) Error(args ...any)                  {}

