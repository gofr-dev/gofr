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

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// Publish publishes a message to a Redis channel or stream.
func (ps *PubSub) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := ps.tracer.Start(ctx, "redis-publish")
	defer span.End()

	ps.parent.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	if topic == "" {
		return errEmptyTopicName
	}

	if !ps.isConnected() {
		return errClientNotConnected
	}

	mode := ps.parent.config.PubSubMode
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
		ps.parent.logger.Errorf("failed to publish message to Redis channel '%s': %v", topic, err)
		return err
	}

	ps.logPubSub("PUB", topic, span, string(message), end.Microseconds(), "")
	ps.parent.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

// publishToStream publishes a message to a Redis stream.
func (ps *PubSub) publishToStream(ctx context.Context, topic string, message []byte, span trace.Span) error {
	start := time.Now()

	args := &redis.XAddArgs{
		Stream: topic,
		Values: map[string]any{"payload": message},
	}

	if ps.parent.config.PubSubStreamsConfig != nil && ps.parent.config.PubSubStreamsConfig.MaxLen > 0 {
		args.MaxLen = ps.parent.config.PubSubStreamsConfig.MaxLen
		args.Approx = true
	}

	id, err := ps.client.XAdd(ctx, args).Result()
	end := time.Since(start)

	if err != nil {
		ps.parent.logger.Errorf("failed to publish message to Redis stream '%s': %v", topic, err)
		return err
	}

	ps.logPubSub("PUB", topic, span, string(message), end.Microseconds(), id)
	ps.parent.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

// Subscribe subscribes to a Redis channel or stream and returns a single message.
func (ps *PubSub) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if topic == "" {
		return nil, errEmptyTopicName
	}

	for !ps.isConnected() {
		select {
		case <-ctx.Done():
			return nil, nil
		case <-time.After(defaultRetryTimeout):
			ps.logDebug("Redis not connected, retrying subscribe for topic '%s'", topic)
		}
	}

	spanCtx, span := ps.tracer.Start(ctx, "redis-subscribe")
	defer span.End()

	ps.parent.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_total_count", "topic", topic)

	msgChan := ps.ensureSubscription(ctx, topic)

	msg := ps.waitForMessage(ctx, spanCtx, span, topic, msgChan)

	return msg, nil
}

// ensureSubscription ensures a subscription is started for the topic.
func (ps *PubSub) ensureSubscription(_ context.Context, topic string) chan *pubsub.Message {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	_, exists := ps.subStarted[topic]
	if exists {
		return ps.receiveChan[topic]
	}

	// Initialize channel before starting subscription
	bufferSize := ps.parent.config.PubSubBufferSize
	if bufferSize == 0 {
		bufferSize = defaultPubSubBufferSize // fallback default
	}

	ps.receiveChan[topic] = make(chan *pubsub.Message, bufferSize)
	ps.chanClosed[topic] = false

	// Create cancel context for this subscription
	subCtx, cancel := context.WithCancel(context.Background())
	ps.subCancel[topic] = cancel

	// Create WaitGroup for this subscription
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ps.subWg[topic] = wg

	// Start subscription in goroutine
	go func() {
		defer wg.Done()
		defer cancel()

		mode := ps.parent.config.PubSubMode
		if mode == "" {
			mode = modeStreams
		}

		for {
			if subCtx.Err() != nil {
				return
			}

			if mode == modeStreams {
				ps.subscribeToStream(subCtx, topic)
			} else {
				ps.subscribeToChannel(subCtx, topic)
			}

			if subCtx.Err() == nil {
				ps.logDebug("Subscription stopped for topic '%s', restarting...", topic)
				time.Sleep(defaultRetryTimeout)
			}
		}
	}()

	ps.subStarted[topic] = struct{}{}

	return ps.receiveChan[topic]
}

// waitForMessage waits for a message from the channel.
func (ps *PubSub) waitForMessage(ctx context.Context, spanCtx context.Context, span trace.Span,
	topic string, msgChan chan *pubsub.Message) *pubsub.Message {
	select {
	case msg := <-msgChan:
		ps.parent.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_success_count", "topic", topic)

		if msg != nil {
			ps.logPubSub("SUB", topic, span, string(msg.Value), 0, "")
		}

		return msg
	case <-ctx.Done():
		return nil
	}
}

// subscribeToChannel subscribes to a Redis channel and forwards messages to the receive channel.
func (ps *PubSub) subscribeToChannel(ctx context.Context, topic string) {
	redisPubSub := ps.client.Subscribe(ctx, topic)
	if redisPubSub == nil {
		ps.logError("failed to create PubSub connection for topic '%s'", topic)
		return
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
		ps.logError("failed to get channel from PubSub for topic '%s'", topic)
		return
	}

	ps.processMessages(ctx, topic, ch)
}

// subscribeToStream subscribes to a Redis stream via a consumer group.
func (ps *PubSub) subscribeToStream(ctx context.Context, topic string) {
	if ps.parent.config.PubSubStreamsConfig == nil || ps.parent.config.PubSubStreamsConfig.ConsumerGroup == "" {
		ps.logError("consumer group not configured for stream '%s'", topic)
		return
	}

	group := ps.parent.config.PubSubStreamsConfig.ConsumerGroup

	if !ps.ensureConsumerGroup(ctx, topic, group) {
		return
	}

	consumer := ps.getConsumerName()
	ps.storeStreamConsumer(topic, group, consumer)

	block := ps.parent.config.PubSubStreamsConfig.Block
	if block == 0 {
		block = 5 * time.Second
	}

	// Consume messages
	for {
		select {
		case <-ctx.Done():
			return
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
	ps.parent.logger.Errorf("failed to create consumer group for stream '%s': %v", topic, err)

	return false
}

func (ps *PubSub) consumeStreamMessages(ctx context.Context, topic, group, consumer string, block time.Duration) {
	// Read new messages
	bufferSize := ps.parent.config.PubSubBufferSize
	if bufferSize == 0 {
		bufferSize = defaultPubSubBufferSize // fallback default
	}

	streams, err := ps.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{topic, ">"},
		Count:    int64(bufferSize),
		Block:    block,
		NoAck:    false,
	}).Result()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}

		// Redis timeout
		if errors.Is(err, redis.Nil) {
			return
		}

		ps.logError("failed to read from stream '%s': %v", topic, err)
		time.Sleep(defaultRetryTimeout)

		return
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			ps.handleStreamMessage(ctx, topic, &msg, group)
		}
	}
}

// getConsumerName returns the configured consumer name or generates one.
func (ps *PubSub) getConsumerName() string {
	if ps.parent.config.PubSubStreamsConfig != nil && ps.parent.config.PubSubStreamsConfig.ConsumerName != "" {
		return ps.parent.config.PubSubStreamsConfig.ConsumerName
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
				ps.logDebug("Redis subscription channel closed for topic '%s'", topic)
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
	m.Committer = newStreamMessage(ps.client, topic, group, msg.ID, ps.parent.logger)

	// Extract payload
	if val, ok := msg.Values["payload"]; ok {
		switch v := val.(type) {
		case string:
			m.Value = []byte(v)
		case []byte:
			m.Value = v
		}
	} else {
		ps.logDebug("received stream message without 'payload' key on topic '%s'", topic)
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

	select {
	case msgChan <- m:
	case <-ctx.Done():
		return
	default:
		ps.logError("message channel full for topic '%s', dropping message", topic)
	}
}

// Health returns the health status of the Redis PubSub connection.
func (ps *PubSub) Health() datasource.Health {
	res := datasource.Health{
		Status: "DOWN",
		Details: map[string]any{
			"backend": "REDIS",
		},
	}

	addr := fmt.Sprintf("%s:%d", ps.parent.config.HostName, ps.parent.config.Port)
	res.Details["addr"] = addr

	mode := ps.parent.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	res.Details["mode"] = mode

	ctx, cancel := context.WithTimeout(context.Background(), defaultRetryTimeout)
	defer cancel()

	if err := ps.client.Ping(ctx).Err(); err != nil {
		ps.parent.logger.Errorf("PubSub health check failed: %v", err)
		return res
	}

	res.Status = "UP"

	return res
}

// CreateTopic is a no-op for Redis PubSub (channels are created on first publish/subscribe).
// For Redis Streams, it creates the stream and consumer group.
func (ps *PubSub) CreateTopic(ctx context.Context, name string) error {
	mode := ps.parent.config.PubSubMode
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
	if ps.parent.config.PubSubStreamsConfig == nil || ps.parent.config.PubSubStreamsConfig.ConsumerGroup == "" {
		return errConsumerGroupNotProvided
	}

	group := ps.parent.config.PubSubStreamsConfig.ConsumerGroup

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

	mode := ps.parent.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	if mode == modeStreams {
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

	ps.mu.Lock()
	ps.chanClosed[topic] = true
	ps.mu.Unlock()

	mode := ps.parent.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	if mode == modeStreams {
		ps.cleanupStreamConsumers(topic)

		return nil
	}

	ps.unsubscribeFromRedis(topic)
	ps.cancelSubscription(topic)
	ps.waitForGoroutine(topic)
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
		ps.logError("failed to unsubscribe from Redis channel '%s': %v", topic, err)
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
		ps.logDebug("timeout waiting for subscription goroutine for topic '%s'", topic)
	}

	ps.mu.Lock()
	delete(ps.subWg, topic)
	ps.mu.Unlock()
}

// cleanupSubscription cleans up subscription resources.
func (ps *PubSub) cleanupSubscription(topic string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ch, ok := ps.receiveChan[topic]; ok && !ps.chanClosed[topic] {
		ps.chanClosed[topic] = true

		close(ch)
		delete(ps.receiveChan, topic)
	}

	delete(ps.subStarted, topic)
	delete(ps.chanClosed, topic)
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

	if ch, ok := ps.receiveChan[topic]; ok && !ps.chanClosed[topic] {
		ps.chanClosed[topic] = true

		close(ch)
		delete(ps.receiveChan, topic)
	}

	delete(ps.subStarted, topic)
	delete(ps.chanClosed, topic)
}

// Query retrieves messages from a Redis channel or stream.
func (ps *PubSub) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if !ps.isConnected() {
		return nil, errClientNotConnected
	}

	if query == "" {
		return nil, errEmptyTopicName
	}

	mode := ps.parent.config.PubSubMode
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

	defer redisPubSub.Close()

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
	timeout = ps.parent.config.PubSubQueryTimeout
	if timeout == 0 {
		timeout = defaultPubSubQueryTimeout // fallback default
	}

	limit = ps.parent.config.PubSubQueryLimit
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

// logDebug logs a debug message.
func (ps *PubSub) logDebug(format string, args ...any) {
	ps.parent.logger.Debugf(format, args...)
}

// logError logs an error message.
func (ps *PubSub) logError(format string, args ...any) {
	ps.parent.logger.Errorf(format, args...)
}

// logInfo logs an info message.
func (ps *PubSub) logInfo(format string, args ...any) {
	ps.parent.logger.Infof(format, args...)
}

// logPubSub logs a PubSub operation.
func (ps *PubSub) logPubSub(mode, topic string, span trace.Span, messageValue string, duration int64, streamID string) {
	traceID := span.SpanContext().TraceID().String()
	addr := fmt.Sprintf("%s:%d", ps.parent.config.HostName, ps.parent.config.Port)

	// Create a simple log entry
	// Note: messageValue, duration, and streamID are available but not used in current simple format
	// They can be used if we enhance the log format in the future
	_ = messageValue
	_ = duration
	_ = streamID

	ps.parent.logger.Debugf("%s %s %s %s", mode, topic, traceID, addr)
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
		close(ch)
		delete(ps.receiveChan, topic)
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
			ps.logDebug("timeout waiting for subscription goroutine for topic '%s'", topic)
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
				ps.logError("Redis connection lost")

				wasConnected = false
			} else if connected && !wasConnected {
				ps.logInfo("Redis connection restored")

				wasConnected = true

				ps.resubscribeAll()
			}
		}
	}
}

// resubscribeAll logs that resubscription is needed (handled by the subscribe loop).
// The actual resubscription happens automatically in the subscribe loop when connection is restored.
func (ps *PubSub) resubscribeAll() {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if len(ps.subStarted) > 0 {
		ps.logInfo("Ensuring all subscriptions are active after reconnection")
	}
}
