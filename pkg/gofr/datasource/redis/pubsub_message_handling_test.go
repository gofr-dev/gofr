package redis

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

func TestPubSub_ProcessMessages_ChannelClosed(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	// Stop monitorConnection to avoid race conditions
	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan *redis.Message)
	close(ch)

	// processMessages should return when channel is closed
	done := make(chan struct{})
	go func() {
		client.PubSub.processMessages(ctx, "test-topic", ch)
		close(done)
	}()

	select {
	case <-done:
		// Expected - processMessages should return
	case <-time.After(1 * time.Second):
		t.Fatal("processMessages did not return when channel closed")
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_ProcessMessages_NilMessage(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	// Stop monitorConnection to avoid race conditions
	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan *redis.Message, 1)
	ch <- nil // Send nil message

	go func() {
		// Send a valid message after nil to allow processMessages to continue
		time.Sleep(50 * time.Millisecond)
		ch <- &redis.Message{Channel: "test", Payload: "data"}
		time.Sleep(50 * time.Millisecond)
		cancel() // Cancel to stop processing
	}()

	client.PubSub.processMessages(ctx, "test-topic", ch)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_ProcessMessages_ContextDone(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	// Stop monitorConnection to avoid race conditions
	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *redis.Message)

	done := make(chan struct{})
	go func() {
		client.PubSub.processMessages(ctx, "test-topic", ch)
		close(done)
	}()

	cancel() // Cancel context immediately

	select {
	case <-done:
		// Expected
	case <-time.After(1 * time.Second):
		t.Fatal("processMessages did not return when context cancelled")
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DispatchMessage_ChannelFull(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	// Stop monitorConnection
	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx := context.Background()
	topic := "test-topic"

	// Create a channel with buffer size 1 and fill it
	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	// Fill the channel
	msg1 := pubsub.NewMessage(ctx)
	client.PubSub.receiveChan[topic] <- msg1

	// Try to dispatch another message (should hit default case - channel full)
	msg2 := pubsub.NewMessage(ctx)
	client.PubSub.dispatchMessage(ctx, topic, msg2)

	// Cleanup
	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DispatchMessage_ChannelClosed(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	ctx := context.Background()
	topic := "test-topic"

	// Set up closed channel
	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message)
	client.PubSub.chanClosed[topic] = true
	client.PubSub.mu.Unlock()

	msg := pubsub.NewMessage(ctx)
	client.PubSub.dispatchMessage(ctx, topic, msg)

	// Cleanup
	client.PubSub.mu.Lock()
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DispatchMessage_ChannelNotExists(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	ctx := context.Background()
	topic := "non-existent-topic"

	msg := pubsub.NewMessage(ctx)
	client.PubSub.dispatchMessage(ctx, topic, msg)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DispatchMessage_ContextDone(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	topic := "test-topic"

	// Create channel
	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	// Fill channel to block
	msg1 := pubsub.NewMessage(ctx)
	client.PubSub.receiveChan[topic] <- msg1

	// Cancel context and try to dispatch
	cancel()
	msg2 := pubsub.NewMessage(ctx)
	client.PubSub.dispatchMessage(ctx, topic, msg2)

	// Cleanup
	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HandleStreamMessage_StringPayload(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	msg := &redis.XMessage{
		ID: "123-0",
		Values: map[string]any{
			"payload": "string-payload",
		},
	}

	// Set up receive channel
	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.handleStreamMessage(ctx, topic, msg, group)

	// Verify message was dispatched
	select {
	case received := <-client.PubSub.receiveChan[topic]:
		require.NotNil(t, received)
		assert.Equal(t, topic, received.Topic)
		assert.Equal(t, []byte("string-payload"), received.Value)
		assert.NotNil(t, received.Committer)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message was not dispatched")
	}

	// Cleanup
	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HandleStreamMessage_BytePayload(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	msg := &redis.XMessage{
		ID: "123-0",
		Values: map[string]any{
			"payload": []byte("byte-payload"),
		},
	}

	// Set up receive channel
	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.handleStreamMessage(ctx, topic, msg, group)

	// Verify message was dispatched
	select {
	case received := <-client.PubSub.receiveChan[topic]:
		require.NotNil(t, received)
		assert.Equal(t, topic, received.Topic)
		assert.Equal(t, []byte("byte-payload"), received.Value)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message was not dispatched")
	}

	// Cleanup
	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HandleStreamMessage_MissingPayload(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	msg := &redis.XMessage{
		ID:     "123-0",
		Values: map[string]any{}, // No payload key
	}

	// Set up receive channel
	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.handleStreamMessage(ctx, topic, msg, group)

	// Verify message was dispatched (even without payload)
	select {
	case received := <-client.PubSub.receiveChan[topic]:
		require.NotNil(t, received)
		assert.Equal(t, topic, received.Topic)
		assert.Nil(t, received.Value) // No payload
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message was not dispatched")
	}

	// Cleanup
	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HandleStreamMessage_UnsupportedPayloadType(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	msg := &redis.XMessage{
		ID: "123-0",
		Values: map[string]any{
			"payload": 12345, // Unsupported type (int)
		},
	}

	// Set up receive channel
	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.handleStreamMessage(ctx, topic, msg, group)

	// Verify message was dispatched (payload will be nil for unsupported types)
	select {
	case received := <-client.PubSub.receiveChan[topic]:
		require.NotNil(t, received)
		assert.Equal(t, topic, received.Topic)
		assert.Nil(t, received.Value) // Unsupported type results in nil
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message was not dispatched")
	}

	// Cleanup
	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}
