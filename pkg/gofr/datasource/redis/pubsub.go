package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// Publish publishes a message to a Redis channel or stream.
func (ps *PubSub) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := ps.tracer.Start(ctx, "redis-publish")
	defer span.End()

	ps.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	if topic == "" {
		return errEmptyTopicName
	}

	if !ps.isConnected() {
		return errClientNotConnected
	}

	mode := ps.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	if mode == modeStreams {
		return ps.publishToStream(ctx, topic, message, span)
	}

	return ps.publishToChannel(ctx, topic, message, span)
}

// publishToChannel publishes a message to a Redis PubSub channel.
func (ps *PubSub) publishToChannel(ctx context.Context, topic string, message []byte, span trace.Span) error {
	start := time.Now()
	err := ps.client.Publish(ctx, topic, message).Err()
	end := time.Since(start)

	if err != nil {
		ps.logger.Errorf("failed to publish message to Redis channel '%s': %v", topic, err)
		return err
	}

	addr := fmt.Sprintf("%s:%d", ps.config.HostName, ps.config.Port)
	ps.logger.Debug(&pubsub.Log{
		Mode:          "PUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(message),
		Topic:         topic,
		Host:          addr,
		PubSubBackend: "REDIS",
		Time:          end.Microseconds(),
	})
	ps.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

// publishToStream publishes a message to a Redis stream.
func (ps *PubSub) publishToStream(ctx context.Context, topic string, message []byte, span trace.Span) error {
	args := &redis.XAddArgs{
		Stream: topic,
		Values: map[string]any{"payload": message},
	}

	if ps.config.PubSubStreamsConfig != nil && ps.config.PubSubStreamsConfig.MaxLen > 0 {
		args.MaxLen = ps.config.PubSubStreamsConfig.MaxLen
		args.Approx = true
	}

	start := time.Now()
	_, err := ps.client.XAdd(ctx, args).Result()
	end := time.Since(start)

	if err != nil {
		ps.logger.Errorf("failed to publish message to Redis stream '%s': %v", topic, err)
		return err
	}

	addr := fmt.Sprintf("%s:%d", ps.config.HostName, ps.config.Port)
	ps.logger.Debug(&pubsub.Log{
		Mode:          "PUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(message),
		Topic:         topic,
		Host:          addr,
		PubSubBackend: "REDIS",
		Time:          end.Microseconds(),
	})
	ps.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

// Subscribe subscribes to a Redis channel or stream and returns a single message.
func (ps *PubSub) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if topic == "" {
		return nil, errEmptyTopicName
	}

	// Check connection with shorter retry interval to avoid long blocking
	for !ps.isConnected() {
		select {
		case <-ctx.Done():
			return nil, nil
		case <-time.After(subscribeRetryInterval):
			ps.logger.Debugf("Redis not connected, retrying subscribe for topic '%s'", topic)
		}
	}

	spanCtx, span := ps.tracer.Start(ctx, "redis-subscribe")
	defer span.End()

	// Determine mode and consumer group for metrics
	mode := ps.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	var consumerGroup string
	if mode == modeStreams && ps.config.PubSubStreamsConfig != nil {
		consumerGroup = ps.config.PubSubStreamsConfig.ConsumerGroup
	}

	// Increment subscribe total count with consumer_group label if using streams
	if consumerGroup != "" {
		ps.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_total_count", "topic", topic, "consumer_group", consumerGroup)
	} else {
		ps.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_total_count", "topic", topic)
	}

	start := time.Now()
	msgChan := ps.ensureSubscription(ctx, topic)

	msg := ps.waitForMessage(ctx, spanCtx, span, topic, msgChan, start, consumerGroup)

	return msg, nil
}

// ensureSubscription ensures a subscription is started for the topic.
func (ps *PubSub) ensureSubscription(_ context.Context, topic string) chan *pubsub.Message {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Double-check pattern: verify subscription state after acquiring lock
	_, exists := ps.subStarted[topic]
	if exists {
		return ps.receiveChan[topic]
	}

	// Re-check connection after acquiring lock to avoid race condition
	if !ps.isConnected() {
		ps.logger.Debugf("Redis not connected when starting subscription for topic '%s'", topic)
		// Still create channel and start subscription - it will retry in goroutine
	}

	// Initialize channel before starting subscription
	bufferSize := ps.config.PubSubBufferSize
	if bufferSize == 0 {
		bufferSize = defaultPubSubBufferSize // fallback default
	}

	ps.receiveChan[topic] = make(chan *pubsub.Message, bufferSize)
	ps.chanClosed[topic] = false
	ps.closeOnce[topic] = &sync.Once{}

	// Create cancel context for this subscription
	_, cancel := context.WithCancel(context.Background())
	ps.subCancel[topic] = cancel

	// Create WaitGroup for this subscription
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ps.subWg[topic] = wg

	// Start subscription in goroutine
	go ps.runSubscriptionLoop(topic, wg, cancel)

	ps.subStarted[topic] = struct{}{}

	return ps.receiveChan[topic]
}

// runSubscriptionLoop runs the subscription loop in a goroutine.
func (ps *PubSub) runSubscriptionLoop(topic string, wg *sync.WaitGroup, cancel context.CancelFunc) {
	defer wg.Done()
	defer cancel()

	mode := ps.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	permanentFailure := false

	for {
		if !ps.shouldContinueSubscription(topic) {
			return
		}

		if permanentFailure {
			ps.logger.Errorf("Subscription for topic '%s' stopped due to permanent failure", topic)

			return
		}

		currentCtx := context.Background()
		err := ps.subscribeWithMode(currentCtx, topic, mode)

		if err != nil && ps.isPermanentError(err) {
			permanentFailure = true

			ps.logger.Errorf("Permanent failure detected for topic '%s': %v", topic, err)

			continue
		}

		// If subscription stopped (not due to permanent failure), restart after delay
		ps.logger.Debugf("Subscription stopped for topic '%s', restarting...", topic)
		time.Sleep(defaultRetryTimeout)
	}
}

// shouldContinueSubscription checks if subscription should continue.
func (ps *PubSub) shouldContinueSubscription(topic string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	_, stillSubscribed := ps.subStarted[topic]
	_, hasCancel := ps.subCancel[topic]

	return stillSubscribed && hasCancel
}

// subscribeWithMode subscribes using the appropriate mode.
func (ps *PubSub) subscribeWithMode(ctx context.Context, topic, mode string) error {
	if mode == modeStreams {
		return ps.subscribeToStreamWithError(ctx, topic)
	}

	return ps.subscribeToChannelWithError(ctx, topic)
}

// waitForMessage waits for a message from the channel.
func (ps *PubSub) waitForMessage(ctx context.Context, spanCtx context.Context, span trace.Span,
	topic string, msgChan chan *pubsub.Message, start time.Time, consumerGroup string) *pubsub.Message {
	select {
	case msg := <-msgChan:
		// Increment subscribe success count with consumer_group label if using streams
		if consumerGroup != "" {
			ps.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_success_count", "topic", topic, "consumer_group", consumerGroup)
		} else {
			ps.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_success_count", "topic", topic)
		}

		if msg != nil {
			end := time.Since(start)
			addr := fmt.Sprintf("%s:%d", ps.config.HostName, ps.config.Port)
			ps.logger.Debug(&pubsub.Log{
				Mode:          "SUB",
				CorrelationID: span.SpanContext().TraceID().String(),
				MessageValue:  string(msg.Value),
				Topic:         topic,
				Host:          addr,
				PubSubBackend: "REDIS",
				Time:          end.Microseconds(),
			})
		}

		return msg
	case <-ctx.Done():
		return nil
	}
}

// isPermanentError checks if an error indicates a permanent failure that should not be retried.
func (*PubSub) isPermanentError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for permanent errors: invalid topic name, permission denied, invalid consumer group
	permanentErrors := []string{
		"invalid topic",
		"permission denied",
		"NOAUTH",
		"invalid consumer group",
		"WRONGTYPE", // Wrong key type (e.g., trying to use stream as channel)
	}

	for _, permErr := range permanentErrors {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(permErr)) {
			return true
		}
	}

	return false
}

// subscribeToChannelWithError subscribes to a Redis channel and returns error if permanent failure.
func (ps *PubSub) subscribeToChannelWithError(ctx context.Context, topic string) error {
	return ps.subscribeToChannel(ctx, topic)
}

// subscribeToStreamWithError subscribes to a Redis stream and returns error if permanent failure.
func (ps *PubSub) subscribeToStreamWithError(ctx context.Context, topic string) error {
	return ps.subscribeToStream(ctx, topic)
}

// subscribeToChannel subscribes to a Redis channel and forwards messages to the receive channel.
func (ps *PubSub) subscribeToChannel(ctx context.Context, topic string) error {
	redisPubSub := ps.client.Subscribe(ctx, topic)
	if redisPubSub == nil {
		ps.logger.Errorf("failed to create PubSub connection for topic '%s'", topic)
		return fmt.Errorf("%w: %s", errPubSubConnectionFailedForTopic, topic)
	}

	ps.mu.Lock()
	ps.subPubSub[topic] = redisPubSub
	ps.mu.Unlock()

	defer func() {
		ps.mu.Lock()
		delete(ps.subPubSub, topic)
		ps.mu.Unlock()

		if redisPubSub != nil {
			redisPubSub.Close()
		}
	}()

	ch := redisPubSub.Channel()
	if ch == nil {
		ps.logger.Errorf("failed to get channel from PubSub for topic '%s'", topic)

		return fmt.Errorf("%w: %s", errPubSubChannelFailedForTopic, topic)
	}

	ps.processMessages(ctx, topic, ch)

	return nil
}

// subscribeToStream subscribes to a Redis stream via a consumer group.
func (ps *PubSub) subscribeToStream(ctx context.Context, topic string) error {
	if ps.config.PubSubStreamsConfig == nil || ps.config.PubSubStreamsConfig.ConsumerGroup == "" {
		ps.logger.Errorf("consumer group not configured for stream '%s'", topic)

		return fmt.Errorf("%w: %s", errConsumerGroupNotConfigured, topic)
	}

	group := ps.config.PubSubStreamsConfig.ConsumerGroup

	if !ps.ensureConsumerGroup(ctx, topic, group) {
		ps.logger.Errorf("failed to ensure consumer group '%s' for stream '%s'", group, topic)

		return fmt.Errorf("%w: group=%s, stream=%s", errFailedToEnsureConsumerGroup, group, topic)
	}

	consumer := ps.getConsumerName()
	ps.storeStreamConsumer(topic, group, consumer)

	block := ps.config.PubSubStreamsConfig.Block
	if block == 0 {
		block = 1 * time.Second // Reduced default for better responsiveness
	}

	// Consume messages
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			ps.consumeStreamMessages(ctx, topic, group, consumer, block)
		}
	}
}

// storeStreamConsumer stores consumer info in the streamConsumers map.
func (ps *PubSub) storeStreamConsumer(topic, group, consumer string) {
	ps.mu.Lock()
	ps.streamConsumers[topic] = &streamConsumer{
		stream:   topic,
		group:    group,
		consumer: consumer,
		cancel:   nil, // handled by subCancel
	}
	ps.mu.Unlock()
}

// ensureConsumerGroup checks if a consumer group exists and creates it if needed.
func (ps *PubSub) ensureConsumerGroup(ctx context.Context, topic, group string) bool {
	groupExists := ps.checkGroupExists(ctx, topic, group)

	if groupExists {
		return true
	}

	return ps.createConsumerGroup(ctx, topic, group)
}

// checkGroupExists checks if a consumer group exists for the given stream.
func (ps *PubSub) checkGroupExists(ctx context.Context, topic, group string) bool {
	groups, err := ps.client.XInfoGroups(ctx, topic).Result()
	if err != nil {
		// If XInfoGroups failed (e.g., stream doesn't exist), we'll create it with MKSTREAM
		return false
	}

	// Stream exists, check if group is in the list
	for _, g := range groups {
		if g.Name == group {
			return true
		}
	}

	return false
}

// createConsumerGroup creates a consumer group for the given stream.
func (ps *PubSub) createConsumerGroup(ctx context.Context, topic, group string) bool {
	err := ps.client.XGroupCreateMkStream(ctx, topic, group, "$").Err()
	if err == nil {
		return true
	}

	// BUSYGROUP means the group already exists (race condition), which is fine
	if strings.Contains(err.Error(), "BUSYGROUP") {
		return true
	}

	// Log error and return false to indicate failure
	ps.logger.Errorf("failed to create consumer group for stream '%s': %v", topic, err)

	return false
}

func (ps *PubSub) consumeStreamMessages(ctx context.Context, topic, group, consumer string, block time.Duration) {
	available, pendingRead, count := ps.getChannelCapacity(topic)
	if available == 0 {
		return
	}

	// Try to read pending messages first if not already read
	if !pendingRead {
		if ps.readPendingMessages(ctx, topic, group, consumer, count) {
			return // Pending messages were processed
		}
	}

	// Read new messages
	ps.readNewMessages(ctx, topic, group, consumer, count, block)
}

// getChannelCapacity returns available channel capacity, pending read status, and count.
func (ps *PubSub) getChannelCapacity(topic string) (available int, pendingRead bool, count int64) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	msgChan, exists := ps.receiveChan[topic]

	if exists && !ps.chanClosed[topic] {
		available = cap(msgChan) - len(msgChan)
	}

	pendingRead = ps.pendingRead[topic]

	bufferSize := ps.config.PubSubBufferSize
	if bufferSize == 0 {
		bufferSize = defaultPubSubBufferSize
	}

	count = int64(bufferSize)
	if available < bufferSize {
		count = int64(available)
	}

	return available, pendingRead, count
}

// readPendingMessages reads and processes pending messages. Returns true if messages were processed.
func (ps *PubSub) readPendingMessages(ctx context.Context, topic, group, consumer string, count int64) bool {
	streams, err := ps.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{topic, "0"},
		Count:    count,
		Block:    0,
		NoAck:    false,
	}).Result()

	ps.markPendingRead(topic)

	if err != nil && errors.Is(err, redis.Nil) {
		return false // No pending messages, continue to read new
	}

	if err != nil {
		ps.logger.Debugf("error reading pending messages for stream '%s': %v, will try new messages", topic, err)

		return false
	}

	if !ps.hasMessages(streams) {
		return false // Empty result
	}

	// Process pending messages
	ps.processStreamMessages(ctx, topic, streams, group)

	return true
}

// markPendingRead marks pending messages as read.
func (ps *PubSub) markPendingRead(topic string) {
	ps.mu.Lock()
	ps.pendingRead[topic] = true
	ps.mu.Unlock()
}

// hasMessages checks if streams contain any messages.
func (*PubSub) hasMessages(streams []redis.XStream) bool {
	for _, stream := range streams {
		if len(stream.Messages) > 0 {
			return true
		}
	}

	return false
}

// processStreamMessages processes messages from streams.
func (ps *PubSub) processStreamMessages(ctx context.Context, topic string, streams []redis.XStream, group string) {
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			ps.handleStreamMessage(ctx, topic, &msg, group)
		}
	}
}

// readNewMessages reads and processes new messages from the stream.
func (ps *PubSub) readNewMessages(ctx context.Context, topic, group, consumer string, count int64, block time.Duration) {
	streams, err := ps.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{topic, ">"},
		Count:    count,
		Block:    block,
		NoAck:    false,
	}).Result()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, redis.Nil) {
			return
		}

		ps.logger.Errorf("failed to read from stream '%s': %v", topic, err)
		time.Sleep(defaultRetryTimeout)

		return
	}

	ps.processStreamMessages(ctx, topic, streams, group)
}

// getConsumerName returns the configured consumer name or generates one.
func (ps *PubSub) getConsumerName() string {
	if ps.config.PubSubStreamsConfig != nil && ps.config.PubSubStreamsConfig.ConsumerName != "" {
		return ps.config.PubSubStreamsConfig.ConsumerName
	}

	hostname, _ := os.Hostname()
	pid := os.Getpid()

	return fmt.Sprintf("consumer-%s-%d-%d", hostname, pid, time.Now().UnixNano())
}

// processMessages processes messages from the Redis channel.
func (ps *PubSub) processMessages(ctx context.Context, topic string, ch <-chan *redis.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				ps.logger.Debugf("Redis subscription channel closed for topic '%s'", topic)
				return
			}

			if msg == nil {
				continue
			}

			ps.handleMessage(ctx, topic, msg)
		}
	}
}

// handleMessage handles a single message from Redis.
func (ps *PubSub) handleMessage(ctx context.Context, topic string, msg *redis.Message) {
	m := pubsub.NewMessage(ctx)
	m.Topic = topic
	m.Value = []byte(msg.Payload)
	m.Committer = newPubSubMessage(msg)

	ps.dispatchMessage(ctx, topic, m)
}

// handleStreamMessage handles a single message from Redis Stream.
func (ps *PubSub) handleStreamMessage(ctx context.Context, topic string, msg *redis.XMessage, group string) {
	m := pubsub.NewMessage(ctx)
	m.Topic = topic
	m.Committer = newStreamMessage(ps.client, topic, group, msg.ID, ps.logger)

	// Extract payload
	if val, ok := msg.Values["payload"]; ok {
		switch v := val.(type) {
		case string:
			m.Value = []byte(v)
		case []byte:
			m.Value = v
		}
	} else {
		ps.logger.Debugf("received stream message without 'payload' key on topic '%s'", topic)
	}

	ps.dispatchMessage(ctx, topic, m)
}

// dispatchMessage sends the message to the receive channel.
func (ps *PubSub) dispatchMessage(ctx context.Context, topic string, m *pubsub.Message) {
	ps.mu.RLock()
	msgChan, exists := ps.receiveChan[topic]
	closed := ps.chanClosed[topic]
	ps.mu.RUnlock()

	if !exists || closed {
		return
	}

	// Use recover to handle closed channel gracefully
	func() {
		defer func() {
			if r := recover(); r != nil {
				ps.logger.Debugf("channel closed for topic '%s' during dispatch", topic)
			}
		}()

		select {
		case msgChan <- m:
		case <-ctx.Done():
			return
		default:
			// Channel full - drop message
			ps.logger.Errorf("message channel full for topic '%s', dropping message", topic)

			// Reset pendingRead for Streams mode so PEL is checked again
			// This ensures dropped messages (which stay in PEL) are retried
			if m.Committer != nil {
				// Check if this is a stream message by type assertion
				// Only reset for Streams mode, not PubSub mode
				if _, isStreamMessage := m.Committer.(*streamMessage); isStreamMessage {
					// Lock is necessary: map writes are not thread-safe
					// Setting to false is idempotent, so safe to do without check
					ps.mu.Lock()
					ps.pendingRead[topic] = false
					ps.mu.Unlock()
				}
			}
		}
	}()
}

// CreateTopic is a no-op for Redis PubSub (channels are created on first publish/subscribe).
// For Redis Streams, it creates the stream and consumer group.
func (ps *PubSub) CreateTopic(ctx context.Context, name string) error {
	mode := ps.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	if mode == modeStreams {
		return ps.createStreamTopic(ctx, name)
	}

	// Redis channels are created automatically on first publish/subscribe
	return nil
}

// createStreamTopic creates a stream topic with consumer group.
func (ps *PubSub) createStreamTopic(ctx context.Context, name string) error {
	if ps.config.PubSubStreamsConfig == nil || ps.config.PubSubStreamsConfig.ConsumerGroup == "" {
		return errConsumerGroupNotProvided
	}

	group := ps.config.PubSubStreamsConfig.ConsumerGroup

	groupExists := ps.checkGroupExists(ctx, name, group)
	if groupExists {
		return nil
	}

	err := ps.client.XGroupCreateMkStream(ctx, name, group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}

	return nil
}

// DeleteTopic unsubscribes all active subscriptions for the given topic/channel.
func (ps *PubSub) DeleteTopic(ctx context.Context, topic string) error {
	if topic == "" {
		return nil
	}

	mode := ps.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	if mode == modeStreams {
		if !ps.isConnected() {
			return errClientNotConnected
		}

		ps.cleanupStreamConsumers(topic)

		return ps.client.Del(ctx, topic).Err()
	}

	// Check if there are any active subscriptions for this topic
	ps.mu.RLock()
	_, hasActiveSub := ps.subStarted[topic]
	ps.mu.RUnlock()

	if !hasActiveSub {
		return nil
	}

	// Unsubscribe from the topic (this will clean up all resources)
	return ps.unsubscribe(topic)
}

// unsubscribe unsubscribes from a Redis channel or stream.
func (ps *PubSub) unsubscribe(topic string) error {
	if topic == "" {
		return errEmptyTopicName
	}

	ps.mu.Lock()
	_, exists := ps.subStarted[topic]
	ps.mu.Unlock()

	if !exists {
		return nil
	}

	mode := ps.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	if mode == modeStreams {
		ps.cleanupStreamConsumers(topic)
		return nil
	}

	// Unsubscribe from Redis first, then set chanClosed flag to avoid race condition
	ps.unsubscribeFromRedis(topic)
	ps.cancelSubscription(topic)
	ps.waitForGoroutine(topic)

	// Set chanClosed after unsubscribe to ensure messages in flight are handled
	ps.mu.Lock()
	ps.chanClosed[topic] = true
	ps.mu.Unlock()

	ps.cleanupSubscription(topic)

	return nil
}

// unsubscribeFromRedis unsubscribes from the Redis channel.
func (ps *PubSub) unsubscribeFromRedis(topic string) {
	ps.mu.RLock()
	pubSub, ok := ps.subPubSub[topic]
	ps.mu.RUnlock()

	if !ok || pubSub == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), unsubscribeOpTimeout)
	defer cancel()

	if err := pubSub.Unsubscribe(ctx, topic); err != nil {
		ps.logger.Errorf("failed to unsubscribe from Redis channel '%s': %v", topic, err)
	}
}

// cancelSubscription cancels the subscription context.
func (ps *PubSub) cancelSubscription(topic string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if cancel, ok := ps.subCancel[topic]; ok {
		cancel()
		delete(ps.subCancel, topic)
	}
}

// waitForGoroutine waits for the subscription goroutine to finish.
func (ps *PubSub) waitForGoroutine(topic string) {
	ps.mu.RLock()
	wg, ok := ps.subWg[topic]
	ps.mu.RUnlock()

	if !ok {
		return
	}

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(goroutineWaitTimeout):
		ps.logger.Debugf("timeout waiting for subscription goroutine for topic '%s'", topic)
	}

	ps.mu.Lock()
	delete(ps.subWg, topic)
	ps.mu.Unlock()
}

// cleanupSubscription cleans up subscription resources.
func (ps *PubSub) cleanupSubscription(topic string) {
	ps.mu.Lock()
	ch, chExists := ps.receiveChan[topic]
	closeOnce, onceExists := ps.closeOnce[topic]
	ps.mu.Unlock()

	if chExists && onceExists {
		ps.chanClosed[topic] = true

		closeOnce.Do(func() {
			close(ch)
		})
	}

	ps.mu.Lock()
	delete(ps.receiveChan, topic)
	delete(ps.closeOnce, topic)
	delete(ps.subStarted, topic)
	delete(ps.chanClosed, topic)
	delete(ps.pendingRead, topic)
	ps.mu.Unlock()
}

// cleanupStreamConsumers cleans up stream consumer resources.
func (ps *PubSub) cleanupStreamConsumers(topic string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if c, ok := ps.streamConsumers[topic]; ok {
		if c.cancel != nil {
			c.cancel()
		}

		delete(ps.streamConsumers, topic)
	}

	if ch, ok := ps.receiveChan[topic]; ok {
		if closeOnce, onceExists := ps.closeOnce[topic]; onceExists {
			ps.chanClosed[topic] = true

			closeOnce.Do(func() {
				close(ch)
			})
		}

		delete(ps.receiveChan, topic)
		delete(ps.closeOnce, topic)
	}

	delete(ps.subStarted, topic)
	delete(ps.chanClosed, topic)
	delete(ps.pendingRead, topic)
}

// Query retrieves messages from a Redis channel or stream.
func (ps *PubSub) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if !ps.isConnected() {
		return nil, errClientNotConnected
	}

	if query == "" {
		return nil, errEmptyTopicName
	}

	mode := ps.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	if mode == modeStreams {
		return ps.queryStream(ctx, query, args...)
	}

	return ps.queryChannel(ctx, query, args...)
}

// queryChannel retrieves messages from a Redis PubSub channel.
func (ps *PubSub) queryChannel(ctx context.Context, query string, args ...any) ([]byte, error) {
	timeout, limit := ps.parseQueryArgs(args...)

	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	redisPubSub := ps.client.Subscribe(queryCtx, query)
	if redisPubSub == nil {
		return nil, errPubSubConnectionFailed
	}

	defer func() {
		if redisPubSub != nil {
			// Explicitly unsubscribe before closing to clean up Redis subscription
			unsubCtx, unsubCancel := context.WithTimeout(context.Background(), unsubscribeOpTimeout)

			_ = redisPubSub.Unsubscribe(unsubCtx, query)

			unsubCancel()
			redisPubSub.Close()
		}
	}()

	ch := redisPubSub.Channel()
	if ch == nil {
		return nil, errPubSubChannelFailed
	}

	return ps.collectMessages(queryCtx, ch, limit), nil
}

// queryStream retrieves messages from a Redis stream.
func (ps *PubSub) queryStream(ctx context.Context, stream string, args ...any) ([]byte, error) {
	timeout, limit := ps.parseQueryArgs(args...)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use XRANGE to get messages from the stream
	vals, err := ps.client.XRangeN(ctx, stream, "-", "+", int64(limit)).Result()
	if err != nil {
		return nil, err
	}

	var result []byte
	for _, msg := range vals {
		var payload []byte

		if val, ok := msg.Values["payload"]; ok {
			switch v := val.(type) {
			case string:
				payload = []byte(v)
			case []byte:
				payload = v
			}
		}

		if len(payload) > 0 {
			if len(result) > 0 {
				result = append(result, '\n')
			}

			result = append(result, payload...)
		}
	}

	return result, nil
}

// collectMessages collects messages from the channel up to the limit.
func (*PubSub) collectMessages(ctx context.Context, ch <-chan *redis.Message, limit int) []byte {
	var result []byte

	collected := 0

	for collected < limit {
		// Check context first before attempting to receive from channel
		select {
		case <-ctx.Done():
			return result
		default:
		}

		// Now try to receive from channel, but also check context
		select {
		case <-ctx.Done():
			return result
		case msg, ok := <-ch:
			if !ok {
				return result
			}

			if msg != nil {
				if len(result) > 0 {
					result = append(result, '\n')
				}

				result = append(result, []byte(msg.Payload)...)
				collected++
			}
		}
	}

	return result
}

// parseQueryArgs parses query arguments (timeout, limit).
// It uses config defaults if not provided, but allows override via args.
func (ps *PubSub) parseQueryArgs(args ...any) (timeout time.Duration, limit int) {
	// Get defaults from config
	timeout = ps.config.PubSubQueryTimeout
	if timeout == 0 {
		timeout = defaultPubSubQueryTimeout // fallback default
	}

	limit = ps.config.PubSubQueryLimit
	if limit == 0 {
		limit = defaultPubSubQueryLimit // fallback default
	}

	// Override with provided args
	if len(args) > 0 {
		if t, ok := args[0].(time.Duration); ok {
			timeout = t
		}
	}

	if len(args) > 1 {
		if l, ok := args[1].(int); ok {
			limit = l
		}
	}

	return timeout, limit
}

// Helper methods

// isConnected checks if the Redis client is connected.
func (ps *PubSub) isConnected() bool {
	ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
	defer cancel()

	return ps.client.Ping(ctx).Err() == nil
}

// Close closes all active subscriptions and cleans up resources.
func (ps *PubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Cancel all subscriptions
	for topic, cancel := range ps.subCancel {
		cancel()
		delete(ps.subCancel, topic)
	}

	// Close all PubSub connections
	for topic, pubSub := range ps.subPubSub {
		if pubSub != nil {
			pubSub.Close()
		}

		delete(ps.subPubSub, topic)
	}

	// Wait for all goroutines
	ps.waitForAllGoroutines()

	// Close all channels
	for topic, ch := range ps.receiveChan {
		if closeOnce, ok := ps.closeOnce[topic]; ok {
			closeOnce.Do(func() {
				close(ch)
			})
		}

		delete(ps.receiveChan, topic)
		delete(ps.closeOnce, topic)
	}

	// Clean up stream consumers
	for topic, consumer := range ps.streamConsumers {
		if consumer.cancel != nil {
			consumer.cancel()
		}

		delete(ps.streamConsumers, topic)
	}

	// Clear all maps
	ps.subStarted = make(map[string]struct{})
	ps.chanClosed = make(map[string]bool)
	ps.closeOnce = make(map[string]*sync.Once)

	if ps.cancel != nil {
		ps.cancel() // Stop monitorConnection
	}

	return nil
}

func (ps *PubSub) waitForAllGoroutines() {
	for topic, wg := range ps.subWg {
		done := make(chan struct{})

		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(goroutineWaitTimeout):
			ps.logger.Debugf("timeout waiting for subscription goroutine for topic '%s'", topic)
		}

		delete(ps.subWg, topic)
	}
}

// monitorConnection periodically checks the connection status and triggers resubscription if connection is restored.
func (ps *PubSub) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(defaultRetryTimeout)
	defer ticker.Stop()

	wasConnected := ps.isConnected()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			connected := ps.isConnected()

			if !connected && wasConnected {
				ps.logger.Errorf("Redis connection lost")

				wasConnected = false
			} else if connected && !wasConnected {
				ps.logger.Infof("Redis connection restored")

				wasConnected = true

				ps.resubscribeAll()
			}
		}
	}
}

// resubscribeAll triggers resubscription by canceling existing subscription contexts.
// Subscription goroutines will detect cancellation and restart, reconnecting to Redis.
func (ps *PubSub) resubscribeAll() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if len(ps.subStarted) == 0 {
		return
	}

	ps.logger.Infof("Triggering resubscription for %d topics after reconnection", len(ps.subStarted))

	// Cancel all subscription contexts to trigger restart
	// This will cause subscription goroutines to restart and reconnect
	// Note: We don't remove from subStarted - the goroutines will restart automatically
	for topic, cancel := range ps.subCancel {
		if cancel != nil {
			// Cancel the old context
			cancel()

			// Create new context for restart
			_, newCancel := context.WithCancel(context.Background())
			ps.subCancel[topic] = newCancel

			// Reset pendingRead so pending messages are read again
			ps.pendingRead[topic] = false
		}
	}
}
