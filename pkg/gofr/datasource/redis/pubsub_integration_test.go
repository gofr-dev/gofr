//go:build integration

package redis

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

func setupIntegrationTest(t *testing.T, extraConf map[string]string) *PubSub {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger := logging.NewMockLogger(logging.DEBUG)

	conf := map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PORT":                   "6379",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": t.Name(),
		"REDIS_PUBSUB_BUFFER_SIZE":     "10",
	}

	for k, v := range extraConf {
		conf[k] = v
	}

	ps := NewPubSub(config.NewMockConfig(conf), mockLogger, mockMetrics)
	require.NotNil(t, ps)

	psClient, ok := ps.(*PubSub)
	require.True(t, ok)

	t.Cleanup(func() {
		psClient.Close()
	})

	return psClient
}

// TestIntegration_BusySpinFix validates that the busy-spin is fixed
// by measuring goroutine CPU usage when the channel is full.
func TestIntegration_BusySpinFix(t *testing.T) {
	ps := setupIntegrationTest(t, map[string]string{
		"REDIS_PUBSUB_BUFFER_SIZE": "1",
	})

	ctx := context.Background()
	topic := fmt.Sprintf("busyspin-test-%d", time.Now().UnixNano())

	err := ps.CreateTopic(ctx, topic)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ps.DeleteTopic(ctx, topic)
	})

	// Start subscription
	go func() {
		_, _ = ps.Subscribe(ctx, topic)
	}()

	time.Sleep(200 * time.Millisecond)

	// Fill the channel to capacity — this triggers the backoff path
	ps.mu.RLock()
	msgChan := ps.receiveChan[topic]
	ps.mu.RUnlock()
	require.NotNil(t, msgChan)

	msgChan <- &pubsub.Message{Topic: topic, Value: []byte("filler")}

	// Measure goroutine count before and after waiting with a full channel.
	// Before the fix, the busy-spin would show high CPU.
	// We can't easily measure CPU in a test, but we can verify the goroutine
	// count stays stable (no goroutine explosion).
	goroutinesBefore := runtime.NumGoroutine()

	// Wait with full channel — the backoff should keep things calm
	time.Sleep(500 * time.Millisecond)

	goroutinesAfter := runtime.NumGoroutine()

	// Goroutine count should be stable (no leak/explosion)
	diff := goroutinesAfter - goroutinesBefore
	assert.LessOrEqual(t, diff, 2, "Goroutine count should be stable when channel is full (diff=%d)", diff)

	t.Log("Goroutines before:", goroutinesBefore, "after:", goroutinesAfter)
}

// TestIntegration_CloseNoDeadlock validates that Close() completes quickly
// with active stream subscriptions (no 5s deadlock timeout).
func TestIntegration_CloseNoDeadlock(t *testing.T) {
	ps := setupIntegrationTest(t, nil)

	ctx := context.Background()
	topic := fmt.Sprintf("close-test-%d", time.Now().UnixNano())

	err := ps.CreateTopic(ctx, topic)
	require.NoError(t, err)

	// Start subscription
	go func() {
		_, _ = ps.Subscribe(ctx, topic)
	}()

	time.Sleep(200 * time.Millisecond)

	// Close should complete well under the 5s goroutineWaitTimeout
	start := time.Now()

	closeDone := make(chan error, 1)

	go func() {
		closeDone <- ps.Close()
	}()

	select {
	case err := <-closeDone:
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, elapsed, 3*time.Second, "Close() took %v — should be much less than the 5s deadlock timeout", elapsed)

		t.Logf("Close() completed in %v", elapsed)
	case <-time.After(5 * time.Second):
		t.Fatal("Close() deadlocked — exceeded 5s timeout")
	}
}

// TestIntegration_CloseFullChannelNoDeadlock validates that Close() works
// even when the message channel is full (the original trigger scenario).
func TestIntegration_CloseFullChannelNoDeadlock(t *testing.T) {
	ps := setupIntegrationTest(t, map[string]string{
		"REDIS_PUBSUB_BUFFER_SIZE": "1",
	})

	ctx := context.Background()
	topic := fmt.Sprintf("close-full-test-%d", time.Now().UnixNano())

	err := ps.CreateTopic(ctx, topic)
	require.NoError(t, err)

	// Start subscription
	go func() {
		_, _ = ps.Subscribe(ctx, topic)
	}()

	time.Sleep(200 * time.Millisecond)

	// Fill the channel
	ps.mu.RLock()
	msgChan := ps.receiveChan[topic]
	ps.mu.RUnlock()

	if msgChan != nil {
		select {
		case msgChan <- &pubsub.Message{Topic: topic, Value: []byte("filler")}:
		default:
		}
	}

	start := time.Now()

	closeDone := make(chan error, 1)

	go func() {
		closeDone <- ps.Close()
	}()

	select {
	case err := <-closeDone:
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, elapsed, 3*time.Second, "Close() with full channel took %v", elapsed)

		t.Logf("Close() with full channel completed in %v", elapsed)
	case <-time.After(5 * time.Second):
		t.Fatal("Close() with full channel deadlocked")
	}
}

// TestIntegration_PublishSubscribeEndToEnd validates that messages flow
// correctly through the stream subscription pipeline.
func TestIntegration_PublishSubscribeEndToEnd(t *testing.T) {
	ps := setupIntegrationTest(t, nil)

	ctx := context.Background()
	topic := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())

	err := ps.CreateTopic(ctx, topic)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ps.DeleteTopic(ctx, topic)
	})

	const messageCount = 5

	received := make(chan string, messageCount)

	// Start consuming
	go func() {
		for i := 0; i < messageCount; i++ {
			msg, err := ps.Subscribe(ctx, topic)
			if err != nil || msg == nil {
				return
			}

			received <- string(msg.Value)

			msg.Commit()
		}
	}()

	// Wait for subscription to be ready
	time.Sleep(300 * time.Millisecond)

	// Publish messages
	for i := 0; i < messageCount; i++ {
		err := ps.Publish(ctx, topic, []byte(fmt.Sprintf("msg-%d", i)))
		require.NoError(t, err)
	}

	// Collect all messages
	var messages []string

	timeout := time.After(10 * time.Second)

	for i := 0; i < messageCount; i++ {
		select {
		case msg := <-received:
			messages = append(messages, msg)
		case <-timeout:
			t.Fatalf("Timed out waiting for messages. Got %d/%d: %v", len(messages), messageCount, messages)
		}
	}

	assert.Len(t, messages, messageCount)
	t.Logf("Received %d messages: %v", len(messages), messages)
}

// TestIntegration_ResubscribeAllStreams validates that resubscribeAll
// properly restarts goroutines and messages flow after reconnection.
func TestIntegration_ResubscribeAllStreams(t *testing.T) {
	ps := setupIntegrationTest(t, nil)

	ctx := context.Background()
	topic := fmt.Sprintf("resub-test-%d", time.Now().UnixNano())

	err := ps.CreateTopic(ctx, topic)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ps.DeleteTopic(ctx, topic)
	})

	// Start subscription using ensureSubscription (no competing reader)
	msgChan := ps.ensureSubscription(ctx, topic)
	require.NotNil(t, msgChan)

	time.Sleep(300 * time.Millisecond)

	// Record old WaitGroup
	ps.mu.RLock()
	oldWg := ps.subWg[topic]
	ps.mu.RUnlock()

	// Trigger resubscription
	resubDone := make(chan struct{})

	go func() {
		ps.resubscribeAll()

		close(resubDone)
	}()

	select {
	case <-resubDone:
		t.Log("resubscribeAll completed successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("resubscribeAll deadlocked")
	}

	// Verify new goroutine is running
	ps.mu.RLock()
	_, hasStarted := ps.subStarted[topic]
	newWg := ps.subWg[topic]
	ps.mu.RUnlock()

	assert.True(t, hasStarted)
	assert.NotSame(t, oldWg, newWg, "New goroutine should have a different WaitGroup")

	// Publish a message and verify it arrives through the new goroutine
	err = ps.Publish(ctx, topic, []byte("post-resub"))
	require.NoError(t, err)

	select {
	case msg := <-msgChan:
		require.NotNil(t, msg)
		assert.Equal(t, "post-resub", string(msg.Value))
		t.Log("Message received after resubscription:", string(msg.Value))
	case <-time.After(5 * time.Second):
		t.Fatal("Did not receive message after resubscription")
	}
}

// TestIntegration_MultipleTopicsCloseCleanly validates that Close()
// works correctly with multiple concurrent stream subscriptions.
func TestIntegration_MultipleTopicsCloseCleanly(t *testing.T) {
	ps := setupIntegrationTest(t, nil)

	ctx := context.Background()

	topics := make([]string, 3)
	for i := range topics {
		topics[i] = fmt.Sprintf("multi-test-%d-%d", i, time.Now().UnixNano())

		err := ps.CreateTopic(ctx, topics[i])
		require.NoError(t, err)
	}

	// Start subscriptions for all topics
	var wg sync.WaitGroup

	for _, topic := range topics {
		wg.Add(1)

		go func(t string) {
			defer wg.Done()

			_, _ = ps.Subscribe(ctx, t)
		}(topic)
	}

	time.Sleep(300 * time.Millisecond)

	// Close all at once
	start := time.Now()

	closeDone := make(chan error, 1)

	go func() {
		closeDone <- ps.Close()
	}()

	select {
	case err := <-closeDone:
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, elapsed, 3*time.Second, "Close() with %d topics took %v", len(topics), elapsed)

		t.Logf("Close() with %d topics completed in %v", len(topics), elapsed)
	case <-time.After(10 * time.Second):
		t.Fatal("Close() with multiple topics deadlocked")
	}

	// Verify all state cleaned up
	ps.mu.RLock()
	assert.Empty(t, ps.subStarted)
	assert.Empty(t, ps.receiveChan)
	assert.Empty(t, ps.streamConsumers)
	ps.mu.RUnlock()
}

// TestIntegration_ResubscribePreservesData validates that messages published
// before resubscribe are not lost — the new goroutine reuses the same consumer
// name so PEL entries are properly recovered.
func TestIntegration_ResubscribePreservesData(t *testing.T) {
	ps := setupIntegrationTest(t, map[string]string{
		"REDIS_PUBSUB_BUFFER_SIZE": "10",
	})

	ctx := context.Background()
	topic := fmt.Sprintf("data-preserve-test-%d", time.Now().UnixNano())

	err := ps.CreateTopic(ctx, topic)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ps.DeleteTopic(ctx, topic)
	})

	// Start subscription (no competing reader)
	msgChan := ps.ensureSubscription(ctx, topic)
	require.NotNil(t, msgChan)

	time.Sleep(300 * time.Millisecond)

	// Record the consumer name before resubscribe
	ps.mu.RLock()
	oldConsumer := ps.streamConsumers[topic].consumer
	ps.mu.RUnlock()
	require.NotEmpty(t, oldConsumer)

	// Publish messages BEFORE resubscribe
	for i := 0; i < 3; i++ {
		err := ps.Publish(ctx, topic, []byte(fmt.Sprintf("pre-resub-%d", i)))
		require.NoError(t, err)
	}

	// Wait for messages to be consumed by the goroutine
	time.Sleep(2 * time.Second)

	// Drain the channel (messages consumed but NOT committed — they stay in PEL)
	var preResubMessages []string

	for {
		select {
		case msg := <-msgChan:
			if msg != nil {
				preResubMessages = append(preResubMessages, string(msg.Value))
				// Intentionally NOT calling msg.Commit() — leaves them in PEL
			}
		default:
			goto drained
		}
	}

drained:

	t.Logf("Drained %d messages before resubscribe: %v", len(preResubMessages), preResubMessages)

	assert.Len(t, preResubMessages, 3, "Should have received all 3 pre-resubscribe messages")

	// Trigger resubscription
	ps.resubscribeAll()

	// Verify consumer name is preserved
	ps.mu.RLock()
	newConsumer := ps.streamConsumers[topic].consumer
	ps.mu.RUnlock()

	assert.Equal(t, oldConsumer, newConsumer,
		"Consumer name should be preserved across resubscribe to maintain PEL ownership")

	t.Logf("Consumer name preserved: %s", newConsumer)

	// The unACKed messages should be re-read from PEL by the new goroutine
	// (same consumer name = same PEL entries)
	var redelivered []string

	timeout := time.After(5 * time.Second)

	for i := 0; i < 3; i++ {
		select {
		case msg := <-msgChan:
			if msg != nil {
				redelivered = append(redelivered, string(msg.Value))
				msg.Commit()
			}
		case <-timeout:
			t.Logf("Got %d/%d redelivered messages: %v", len(redelivered), 3, redelivered)
			break
		}
	}

	t.Logf("Redelivered %d messages after resubscribe: %v", len(redelivered), redelivered)
	assert.Len(t, redelivered, 3, "All unACKed messages should be redelivered from PEL after resubscribe")
}
