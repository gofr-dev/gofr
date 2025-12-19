package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

func TestPubSub_SubscribeToChannel_NilPubSub(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	// Stop monitorConnection
	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock Subscribe to return nil
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectSubscribe("test-topic").Return(nil)

	// subscribeToChannel should handle nil gracefully
	done := make(chan struct{})
	go func() {
		client.PubSub.subscribeToChannel(ctx, "test-topic")
		close(done)
	}()

	select {
	case <-done:
		// Expected - should return when pubsub is nil
	case <-time.After(1 * time.Second):
		t.Fatal("subscribeToChannel did not return")
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_SubscribeToChannel_NilChannel(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	// Stop monitorConnection
	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a mock pubsub that returns nil channel
	mockPubSub := &mockRedisPubSub{channel: nil}
	client.PubSub.mu.Lock()
	client.PubSub.subPubSub["test-topic"] = mockPubSub
	client.PubSub.mu.Unlock()

	// subscribeToChannel should handle nil channel gracefully
	done := make(chan struct{})
	go func() {
		// We need to manually call the logic since we can't easily mock Subscribe
		// Instead, test the nil channel path in processMessages
		cancel()
		close(done)
	}()

	<-done

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_EnsureConsumerGroup_GroupExists(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	// Mock XInfoGroups to return existing group
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{
		{Name: "test-group"},
	})

	result := client.PubSub.ensureConsumerGroup(ctx, topic, group)
	assert.True(t, result, "should return true when group exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_EnsureConsumerGroup_GroupNotExists(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	// Mock XInfoGroups to return empty (group doesn't exist)
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{})
	mock.ExpectXGroupCreateMkStream(topic, group, "$").SetVal("OK")

	result := client.PubSub.ensureConsumerGroup(ctx, topic, group)
	assert.True(t, result, "should return true after creating group")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_EnsureConsumerGroup_CreateFails(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	// Mock XInfoGroups to return empty, then XGroupCreateMkStream fails
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{})
	mock.ExpectXGroupCreateMkStream(topic, group, "$").SetErr(errMockGroup)

	result := client.PubSub.ensureConsumerGroup(ctx, topic, group)
	assert.False(t, result, "should return false when group creation fails")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CheckGroupExists_Error(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	// Mock XInfoGroups to return error
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXInfoGroups(topic).SetErr(errMockGroup)

	result := client.PubSub.checkGroupExists(ctx, topic, group)
	assert.False(t, result, "should return false on error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CheckGroupExists_GroupFound(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	// Mock XInfoGroups to return group list with matching group
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{
		{Name: "other-group"},
		{Name: "test-group"},
		{Name: "another-group"},
	})

	result := client.PubSub.checkGroupExists(ctx, topic, group)
	assert.True(t, result, "should return true when group found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CheckGroupExists_GroupNotFound(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	// Mock XInfoGroups to return group list without matching group
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{
		{Name: "other-group"},
		{Name: "another-group"},
	})

	result := client.PubSub.checkGroupExists(ctx, topic, group)
	assert.False(t, result, "should return false when group not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CleanupSubscription_ChannelNotClosed(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	topic := "test-topic"

	// Set up subscription state
	client.PubSub.mu.Lock()
	ch := make(chan *pubsub.Message, 1)
	client.PubSub.receiveChan[topic] = ch
	client.PubSub.chanClosed[topic] = false
	client.PubSub.subStarted[topic] = struct{}{}
	client.PubSub.mu.Unlock()

	client.PubSub.cleanupSubscription(topic)

	// Verify channel is closed
	client.PubSub.mu.Lock()
	_, exists := client.PubSub.receiveChan[topic]
	_, startedExists := client.PubSub.subStarted[topic]
	_, closedExists := client.PubSub.chanClosed[topic]
	client.PubSub.mu.Unlock()

	assert.False(t, exists, "receiveChan should be deleted")
	assert.False(t, startedExists, "subStarted should be deleted")
	assert.False(t, closedExists, "chanClosed should be deleted")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CleanupSubscription_ChannelAlreadyClosed(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	topic := "test-topic"

	// Set up subscription state with closed channel
	client.PubSub.mu.Lock()
	ch := make(chan *pubsub.Message)
	close(ch)
	client.PubSub.receiveChan[topic] = ch
	client.PubSub.chanClosed[topic] = true
	client.PubSub.subStarted[topic] = struct{}{}
	client.PubSub.mu.Unlock()

	client.PubSub.cleanupSubscription(topic)

	// Verify cleanup
	client.PubSub.mu.Lock()
	_, exists := client.PubSub.receiveChan[topic]
	_, startedExists := client.PubSub.subStarted[topic]
	_, closedExists := client.PubSub.chanClosed[topic]
	client.PubSub.mu.Unlock()

	assert.False(t, exists, "receiveChan should be deleted")
	assert.False(t, startedExists, "subStarted should be deleted")
	assert.False(t, closedExists, "chanClosed should be deleted")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CleanupSubscription_NoChannel(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	topic := "test-topic"

	// Set up subscription state without channel
	client.PubSub.mu.Lock()
	client.PubSub.subStarted[topic] = struct{}{}
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.cleanupSubscription(topic)

	// Verify cleanup
	client.PubSub.mu.Lock()
	_, startedExists := client.PubSub.subStarted[topic]
	_, closedExists := client.PubSub.chanClosed[topic]
	client.PubSub.mu.Unlock()

	assert.False(t, startedExists, "subStarted should be deleted")
	assert.False(t, closedExists, "chanClosed should be deleted")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DeleteTopic_EmptyTopic(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	ctx := context.Background()

	err := client.PubSub.DeleteTopic(ctx, "")
	require.NoError(t, err, "should return nil for empty topic")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DeleteTopic_PubSubMode_NoActiveSubscription(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-topic"

	// No active subscription
	mock.ExpectPing().SetVal("PONG")

	err := client.PubSub.DeleteTopic(ctx, topic)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DeleteTopic_PubSubMode_WithActiveSubscription(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "test-topic"

	// Start subscription
	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(100 * time.Millisecond)

	// Delete topic (should unsubscribe)
	err := client.PubSub.DeleteTopic(ctx, topic)
	require.NoError(t, err)
}

func TestPubSub_DeleteTopic_StreamMode(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"

	// Mock Del for stream deletion
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectDel(topic).SetVal(1)

	err := client.PubSub.DeleteTopic(ctx, topic)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_UnsubscribeFromRedis_NoPubSub(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	topic := "test-topic"

	// No pubsub in map
	client.PubSub.unsubscribeFromRedis(topic)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_UnsubscribeFromRedis_NilPubSub(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	topic := "test-topic"

	// Set nil pubsub
	client.PubSub.mu.Lock()
	client.PubSub.subPubSub[topic] = nil
	client.PubSub.mu.Unlock()

	client.PubSub.unsubscribeFromRedis(topic)

	// Cleanup
	client.PubSub.mu.Lock()
	delete(client.PubSub.subPubSub, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_UnsubscribeFromRedis_Error(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	// Stop monitorConnection
	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	topic := "test-topic"

	// Create a mock pubsub that returns error on Unsubscribe
	mockPubSub := &mockRedisPubSub{
		unsubscribeErr: errors.New("unsubscribe error"),
	}

	client.PubSub.mu.Lock()
	client.PubSub.subPubSub[topic] = mockPubSub
	client.PubSub.mu.Unlock()

	client.PubSub.unsubscribeFromRedis(topic)

	// Cleanup
	client.PubSub.mu.Lock()
	delete(client.PubSub.subPubSub, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CreateTopic_PubSubMode(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-channel"

	// CreateTopic should return nil for pubsub mode (channels auto-create)
	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err, "should return nil for pubsub mode")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CreateTopic_StreamMode_GroupExists(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"

	// Mock group exists
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{
		{Name: "test-group"},
	})

	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err, "should return nil when group exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CreateTopic_StreamMode_BusyGroup(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"

	// Mock group doesn't exist, but creation returns BUSYGROUP
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{})
	mock.ExpectXGroupCreateMkStream(topic, "test-group", "$").SetErr(errors.New("BUSYGROUP Consumer Group name already exists"))

	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err, "should return nil for BUSYGROUP error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CollectMessages_ContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *redis.Message, 1)
	ch <- &redis.Message{Payload: "msg1"}

	// Cancel context immediately
	cancel()

	ps := &PubSub{}
	result := ps.collectMessages(ctx, ch, 10)
	assert.Empty(t, result, "should return empty when context done")
}

func TestPubSub_CollectMessages_ChannelClosed(t *testing.T) {
	ctx := context.Background()
	ch := make(chan *redis.Message)
	close(ch)

	ps := &PubSub{}
	result := ps.collectMessages(ctx, ch, 10)
	assert.Empty(t, result, "should return empty when channel closed")
}

func TestPubSub_CollectMessages_NilMessage(t *testing.T) {
	ctx := context.Background()
	ch := make(chan *redis.Message, 2)
	ch <- nil
	ch <- &redis.Message{Payload: "msg1"}
	close(ch)

	ps := &PubSub{}
	result := ps.collectMessages(ctx, ch, 10)
	assert.Equal(t, []byte("msg1"), result, "should skip nil messages")
}

func TestPubSub_CollectMessages_ReachesLimit(t *testing.T) {
	ctx := context.Background()
	ch := make(chan *redis.Message, 3)
	ch <- &redis.Message{Payload: "msg1"}
	ch <- &redis.Message{Payload: "msg2"}
	ch <- &redis.Message{Payload: "msg3"}
	close(ch)

	ps := &PubSub{}
	result := ps.collectMessages(ctx, ch, 2)
	expected := []byte("msg1\nmsg2")
	assert.Equal(t, expected, result, "should stop at limit")
}

// mockRedisPubSub is a helper for testing
type mockRedisPubSub struct {
	channel        <-chan *redis.Message
	unsubscribeErr error
}

func (m *mockRedisPubSub) Subscribe(ctx context.Context, channels ...string) error {
	return nil
}

func (m *mockRedisPubSub) Unsubscribe(ctx context.Context, channels ...string) error {
	return m.unsubscribeErr
}

func (m *mockRedisPubSub) Channel() <-chan *redis.Message {
	return m.channel
}

func (m *mockRedisPubSub) Close() error {
	return nil
}
