// Package redis provides a client for interacting with Redis Pub/Sub.
// This package facilitates interaction with Redis Pub/Sub, allowing publishing
// and subscribing to channels, and handling messages.
package redis

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// Client represents a Redis PubSub client.
type Client struct {
	// Separate connections for pub/sub operations
	// pubConn is used for publishing (non-blocking)
	// subConn is used for subscribing (blocking)
	// queryConn is used for Query operations (separate to avoid conflicts)
	pubConn   *redis.Client
	subConn   *redis.Client
	queryConn *redis.Client

	// Channel-based message buffering (similar to Google PubSub pattern)
	receiveChan map[string]chan *pubsub.Message
	subStarted  map[string]struct{}
	subCancel   map[string]context.CancelFunc
	subPubSub   map[string]*redis.PubSub   // Track active PubSub connections for unsubscribe
	subWg       map[string]*sync.WaitGroup // Track goroutines for proper cleanup
	chanClosed  map[string]bool            // Track closed channels to prevent writes
	mu          sync.RWMutex

	cfg     *Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

// sanitizeRedisAddr removes credentials from a Redis address for safe logging.
// It handles Redis URI format (redis://user:password@host:port) and plain host:port format.
func sanitizeRedisAddr(addr string) string {
	if !strings.Contains(addr, "@") {
		return addr
	}

	lastAt := strings.LastIndex(addr, "@")
	if lastAt < 0 || lastAt >= len(addr)-1 {
		return addr
	}

	hostPart := addr[lastAt+1:]

	return sanitizeRedisAddrWithScheme(addr, hostPart)
}

// sanitizeRedisAddrWithScheme adds the scheme back to the sanitized address.
func sanitizeRedisAddrWithScheme(original, hostPart string) string {
	if strings.HasPrefix(original, "redis://") {
		return "redis://" + hostPart
	}

	if strings.HasPrefix(original, "rediss://") {
		return "rediss://" + hostPart
	}

	return hostPart
}

// New creates a new Redis PubSub client.
func New(cfg *Config) *Client {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &Client{
		cfg:         cfg,
		receiveChan: make(map[string]chan *pubsub.Message),
		subStarted:  make(map[string]struct{}),
		subCancel:   make(map[string]context.CancelFunc),
		subPubSub:   make(map[string]*redis.PubSub),
		subWg:       make(map[string]*sync.WaitGroup),
		chanClosed:  make(map[string]bool),
	}
}

// NewClient creates a Redis PubSub client from config.Config (similar to Redis DB).
// This function reads configuration from environment variables and auto-connects.
// It's used for automatic initialization in container.Create().
// If no Redis address is configured, it returns nil (same behavior as Redis DB).
func NewClient(c config.Config, logger pubsub.Logger, metrics Metrics) *Client {
	cfg := getRedisPubSubConfig(c)

	// If no Redis address configured, return nil (same as Redis DB behavior)
	if cfg.Addr == "" {
		return nil
	}

	client := New(cfg)
	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.Connect() // Auto-connect

	return client
}

// UseLogger sets the logger for the Redis client.
func (r *Client) UseLogger(logger any) {
	if l, ok := logger.(pubsub.Logger); ok {
		// Adapt pubsub.Logger to our Logger interface
		r.logger = &loggerAdapter{Logger: l}

		return
	}

	if l, ok := logger.(Logger); ok {
		r.logger = l
	}
}

// UseMetrics sets the metrics for the Redis client.
func (r *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		r.metrics = m
	}
}

// UseTracer sets the tracer for the Redis client.
func (r *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		r.tracer = t
	}
}

// Connect establishes connections to Redis for publishing and subscribing.
func (r *Client) Connect() {
	if err := validateConfigs(r.cfg); err != nil {
		r.logError("could not initialize Redis, error: %v", err)

		return
	}

	r.logDebug("connecting to Redis at '%s'", sanitizeRedisAddr(r.cfg.Addr))

	options, err := createRedisOptions(r.cfg)
	if err != nil {
		r.logError("failed to create Redis options: %v", err)

		return
	}

	r.createConnections(options)

	if err := r.testConnections(); err != nil {
		r.logError("failed to connect to Redis at '%s', error: %v", sanitizeRedisAddr(r.cfg.Addr), err)

		go r.retryConnect()

		return
	}

	r.testQueryConnection()

	r.logInfo("connected to Redis at '%s'", sanitizeRedisAddr(r.cfg.Addr))
}

// createConnections creates all Redis connections.
func (r *Client) createConnections(options *redis.Options) {
	r.pubConn = redis.NewClient(options)

	subOptions := *options
	r.subConn = redis.NewClient(&subOptions)

	queryOptions := *options
	r.queryConn = redis.NewClient(&queryOptions)
}

// testQueryConnection tests the query connection.
func (r *Client) testQueryConnection() {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.DialTimeout)
	defer cancel()

	if err := r.queryConn.Ping(ctx).Err(); err != nil {
		r.logError("failed to connect query connection to Redis at '%s', error: %v", sanitizeRedisAddr(r.cfg.Addr), err)
	}
}

// logError logs an error if logger is available.
func (r *Client) logError(format string, args ...any) {
	if r.logger != nil {
		r.logger.Errorf(format, args...)
	}
}

// logDebug logs a debug message if logger is available.
func (r *Client) logDebug(format string, args ...any) {
	if r.logger != nil {
		r.logger.Debugf(format, args...)
	}
}

// logInfo logs an info message if logger is available.
func (r *Client) logInfo(format string, args ...any) {
	if r.logger != nil {
		r.logger.Logf(format, args...)
	}
}

// testConnections tests both publisher and subscriber connections.
func (r *Client) testConnections() error {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.DialTimeout)
	defer cancel()

	if r.pubConn == nil {
		return errClientNotConnected
	}

	if err := r.pubConn.Ping(ctx).Err(); err != nil {
		return err
	}

	if r.subConn == nil {
		return errClientNotConnected
	}

	if err := r.subConn.Ping(ctx).Err(); err != nil {
		return err
	}

	return nil
}

// retryConnect handles the retry mechanism for connecting to Redis.
func (r *Client) retryConnect() {
	for {
		time.Sleep(defaultRetryTimeout)

		options, err := createRedisOptions(r.cfg)
		if err != nil {
			r.logError("failed to create Redis options during retry: %v", err)

			continue
		}

		r.recreateConnections(options)

		if !r.testAllConnections() {
			continue
		}

		r.logInfo("reconnected to Redis at '%s'", sanitizeRedisAddr(r.cfg.Addr))

		// Restart subscriptions if they were active
		r.restartSubscriptions()

		return
	}
}

// recreateConnections closes existing connections and creates new ones.
func (r *Client) recreateConnections(options *redis.Options) {
	r.closeConnection(&r.pubConn)
	r.closeConnection(&r.subConn)
	r.closeConnection(&r.queryConn)

	r.pubConn = redis.NewClient(options)
	subOptions := *options
	r.subConn = redis.NewClient(&subOptions)
	queryOptions := *options
	r.queryConn = redis.NewClient(&queryOptions)
}

// closeConnection safely closes a connection if it's not nil.
func (*Client) closeConnection(conn **redis.Client) {
	if *conn != nil {
		_ = (*conn).Close()
	}
}

// testAllConnections tests all three connections and returns true if all succeed.
func (r *Client) testAllConnections() bool {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.DialTimeout)
	defer cancel()

	pubErr := r.pubConn.Ping(ctx).Err()
	subErr := r.subConn.Ping(ctx).Err()
	queryErr := r.queryConn.Ping(ctx).Err()

	if pubErr != nil || subErr != nil || queryErr != nil {
		r.logError("could not connect to Redis at '%s', pub error: %v, sub error: %v, query error: %v",
			sanitizeRedisAddr(r.cfg.Addr), pubErr, subErr, queryErr)

		return false
	}

	return true
}

// restartSubscriptions restarts all active subscriptions after reconnection.
func (r *Client) restartSubscriptions() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear subscription state to allow re-subscription
	for topic := range r.subStarted {
		delete(r.subStarted, topic)

		if cancel, ok := r.subCancel[topic]; ok {
			cancel()
			delete(r.subCancel, topic)
		}

		// Close PubSub connections
		if pubSub, ok := r.subPubSub[topic]; ok {
			_ = pubSub.Unsubscribe(context.Background(), topic)
			pubSub.Close()
			delete(r.subPubSub, topic)
		}

		// Wait for goroutine to finish
		if wg, ok := r.subWg[topic]; ok {
			// Cancel first to signal goroutine to stop
			if cancel, ok := r.subCancel[topic]; ok {
				cancel()
			}

			// Wait for goroutine with timeout
			done := make(chan struct{})

			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(goroutineWaitTimeout):
				if r.logger != nil {
					r.logger.Debugf("timeout waiting for subscription goroutine for topic '%s'", topic)
				}
			}

			delete(r.subWg, topic)
		}

		// Close channels
		if ch, ok := r.receiveChan[topic]; ok {
			r.chanClosed[topic] = true

			close(ch)
			delete(r.receiveChan, topic)
		}
	}
}

// Publish publishes a message to a Redis channel.
func (r *Client) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "redis-publish")
	defer span.End()

	if r.metrics != nil {
		r.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)
	}

	if r.pubConn == nil || topic == "" {
		return errPublisherNotConfigured
	}

	if !r.isConnected() {
		return errClientNotConnected
	}

	start := time.Now()
	err := r.pubConn.Publish(ctx, topic, message).Err()
	end := time.Since(start)

	if err != nil {
		if r.logger != nil {
			r.logger.Errorf("failed to publish message to Redis channel '%s', error: %v", topic, err)
		}

		return err
	}

	if r.logger != nil {
		r.logger.Debug(&Log{
			Mode:          "PUB",
			CorrelationID: span.SpanContext().TraceID().String(),
			MessageValue:  strings.Join(strings.Fields(string(message)), " "),
			Topic:         topic,
			Host:          sanitizeRedisAddr(r.cfg.Addr),
			PubSubBackend: "REDIS",
			Time:          end.Microseconds(),
		})
	}

	if r.metrics != nil {
		r.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)
	}

	return nil
}

// Subscribe subscribes to a Redis channel and returns a single message.
func (r *Client) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if err := r.validateSubscribe(topic); err != nil {
		return nil, err
	}

	spanCtx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "redis-subscribe")
	defer span.End()

	if r.metrics != nil {
		r.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_total_count", "topic", topic)
	}

	msgChan := r.ensureSubscription(topic)

	msg := r.waitForMessage(ctx, spanCtx, span, topic, msgChan)

	return msg, nil
}

// validateSubscribe validates subscription prerequisites.
func (r *Client) validateSubscribe(topic string) error {
	if r.subConn == nil {
		return errClientNotConnected
	}

	if !r.isConnected() {
		time.Sleep(defaultRetryTimeout)

		return errClientNotConnected
	}

	if topic == "" {
		return errEmptyTopicName
	}

	return nil
}

// ensureSubscription ensures a subscription is started for the topic.
func (r *Client) ensureSubscription(topic string) chan *pubsub.Message {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.subStarted[topic]
	if !exists {
		// Initialize channel before starting subscription
		r.receiveChan[topic] = make(chan *pubsub.Message, messageBufferSize)
		r.chanClosed[topic] = false

		// Create cancel context for this subscription
		subCtx, cancel := context.WithCancel(context.Background())
		r.subCancel[topic] = cancel

		// Create WaitGroup for this subscription
		wg := &sync.WaitGroup{}
		wg.Add(1)
		r.subWg[topic] = wg

		// Start subscription in goroutine
		go func() {
			defer wg.Done()
			defer cancel() // Ensure cancel is always called

			r.subscribeToChannel(subCtx, topic)
		}()

		r.subStarted[topic] = struct{}{}
	}

	return r.receiveChan[topic]
}

// waitForMessage waits for a message from the channel.
func (r *Client) waitForMessage(ctx context.Context, spanCtx context.Context, span trace.Span,
	topic string, msgChan chan *pubsub.Message) *pubsub.Message {
	select {
	case msg := <-msgChan:
		if r.metrics != nil {
			r.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_success_count", "topic", topic)
		}

		if r.logger != nil && msg != nil {
			r.logger.Debug(&Log{
				Mode:          "SUB",
				CorrelationID: span.SpanContext().TraceID().String(),
				MessageValue:  strings.Join(strings.Fields(string(msg.Value)), " "),
				Topic:         topic,
				Host:          sanitizeRedisAddr(r.cfg.Addr),
				PubSubBackend: "REDIS",
				Time:          0, // Redis doesn't provide timing info
			})
		}

		return msg
	case <-ctx.Done():
		return nil
	}
}

// subscribeToChannel subscribes to a Redis channel and forwards messages to the receive channel.
func (r *Client) subscribeToChannel(ctx context.Context, topic string) {
	if r.subConn == nil {
		r.logError("subscriber connection is nil for topic '%s'", topic)

		return
	}

	redisPubSub := r.subConn.Subscribe(ctx, topic)
	if redisPubSub == nil {
		r.logError("failed to create PubSub connection for topic '%s'", topic)

		return
	}

	r.storePubSubConnection(topic, redisPubSub)

	defer r.cleanupPubSubConnection(topic, redisPubSub)

	ch := redisPubSub.Channel()
	if ch == nil {
		r.logError("failed to get channel from PubSub for topic '%s'", topic)

		return
	}

	r.processMessages(ctx, topic, ch)
}

// storePubSubConnection stores the PubSub connection for potential unsubscribe.
func (r *Client) storePubSubConnection(topic string, pubSub *redis.PubSub) {
	r.mu.Lock()
	r.subPubSub[topic] = pubSub
	r.mu.Unlock()
}

// cleanupPubSubConnection cleans up the PubSub connection.
func (r *Client) cleanupPubSubConnection(topic string, pubSub *redis.PubSub) {
	if pubSub != nil {
		pubSub.Close()
	}

	r.mu.Lock()
	delete(r.subPubSub, topic)
	r.mu.Unlock()
}

// processMessages processes messages from the Redis channel.
func (r *Client) processMessages(ctx context.Context, topic string, ch <-chan *redis.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				r.logDebug("Redis subscription channel closed for topic '%s', attempting reconnect", topic)

				return
			}

			if msg == nil {
				continue
			}

			r.handleMessage(ctx, topic, msg)
		}
	}
}

// handleMessage handles a single message from Redis.
func (r *Client) handleMessage(ctx context.Context, topic string, msg *redis.Message) {
	m := pubsub.NewMessage(ctx)
	m.Topic = topic
	m.Value = []byte(msg.Payload)
	m.Committer = newRedisMessage(msg, r.logger)

	r.mu.RLock()
	msgChan, exists := r.receiveChan[topic]
	closed := r.chanClosed[topic]
	r.mu.RUnlock()

	if !exists || closed {
		return
	}

	select {
	case msgChan <- m:
	case <-ctx.Done():
		return
	default:
		r.logDebug("message channel full for topic '%s', dropping message", topic)
	}
}

// Health returns the health status of the Redis connection.
func (r *Client) Health() datasource.Health {
	res := datasource.Health{
		Status: "DOWN",
		Details: map[string]any{
			"backend": "REDIS",
			"addr":    sanitizeRedisAddr(r.cfg.Addr),
		},
	}

	if r.pubConn == nil {
		if r.logger != nil {
			r.logger.Error("datasource not initialized")
		}

		return res
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRetryTimeout)
	defer cancel()

	if err := r.pubConn.Ping(ctx).Err(); err != nil {
		if r.logger != nil {
			r.logger.Errorf("health check failed: %v", err)
		}

		return res
	}

	res.Status = "UP"

	return res
}

// CreateTopic is a no-op for Redis (channels are created on first publish/subscribe).
func (*Client) CreateTopic(_ context.Context, _ string) error {
	// Redis channels are created automatically on first publish/subscribe
	return nil
}

// DeleteTopic is a no-op for Redis (channels cannot be deleted).
func (*Client) DeleteTopic(_ context.Context, _ string) error {
	// Redis doesn't support deleting channels
	// Unsubscribing from all channels is the closest equivalent
	return nil
}

// Unsubscribe unsubscribes from a Redis channel.
// This method follows the same pattern as MQTT's Unsubscribe implementation.
func (r *Client) Unsubscribe(topic string) error {
	if r.subConn == nil {
		return errClientNotConnected
	}

	if topic == "" {
		return errEmptyTopicName
	}

	r.mu.Lock()
	_, exists := r.subStarted[topic]
	r.mu.Unlock()

	if !exists {
		return nil
	}

	r.mu.Lock()
	r.chanClosed[topic] = true
	r.mu.Unlock()

	r.unsubscribeFromRedis(topic)
	r.cancelSubscription(topic)
	r.waitForGoroutine(topic)
	r.cleanupSubscription(topic)

	r.logDebug("unsubscribed from Redis channel '%s'", topic)

	return nil
}

// unsubscribeFromRedis unsubscribes from the Redis channel.
func (r *Client) unsubscribeFromRedis(topic string) {
	r.mu.RLock()
	pubSub, ok := r.subPubSub[topic]
	r.mu.RUnlock()

	if !ok || pubSub == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), unsubscribeOpTimeout)
	defer cancel()

	if err := pubSub.Unsubscribe(ctx, topic); err != nil {
		r.logError("error while unsubscribing from Redis channel '%s', error: %v", topic, err)
	}
}

// cancelSubscription cancels the subscription context.
func (r *Client) cancelSubscription(topic string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cancel, ok := r.subCancel[topic]; ok {
		cancel()
		delete(r.subCancel, topic)
	}
}

// waitForGoroutine waits for the subscription goroutine to finish.
func (r *Client) waitForGoroutine(topic string) {
	r.mu.RLock()
	wg, ok := r.subWg[topic]
	r.mu.RUnlock()

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
		r.logDebug("timeout waiting for subscription goroutine for topic '%s'", topic)
	}

	r.mu.Lock()
	delete(r.subWg, topic)
	r.mu.Unlock()
}

// cleanupSubscription cleans up subscription resources.
func (r *Client) cleanupSubscription(topic string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ch, ok := r.receiveChan[topic]; ok {
		close(ch)
		delete(r.receiveChan, topic)
	}

	delete(r.subStarted, topic)
	delete(r.chanClosed, topic)
}

// Query retrieves messages from a Redis channel.
// Uses a separate query connection to avoid conflicts with active subscriptions.
func (r *Client) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if !r.isConnected() {
		return nil, errClientNotConnected
	}

	if query == "" {
		return nil, errEmptyTopicName
	}

	connToUse := r.getQueryConnection()
	if connToUse == nil {
		return nil, errClientNotConnected
	}

	timeout, limit := parseQueryArgs(args...)

	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	redisPubSub := connToUse.Subscribe(queryCtx, query)
	if redisPubSub == nil {
		return nil, errPubSubConnectionFailed
	}

	defer redisPubSub.Close()

	ch := redisPubSub.Channel()
	if ch == nil {
		return nil, errPubSubChannelFailed
	}

	return r.collectMessages(queryCtx, ch, limit), nil
}

// getQueryConnection returns the query connection or falls back to subConn.
func (r *Client) getQueryConnection() *redis.Client {
	if r.queryConn != nil {
		return r.queryConn
	}

	return r.subConn
}

// collectMessages collects messages from the channel up to the limit.
func (*Client) collectMessages(ctx context.Context, ch <-chan *redis.Message, limit int) []byte {
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

			if msg == nil {
				continue
			}

			if len(result) > 0 {
				result = append(result, '\n')
			}

			result = append(result, []byte(msg.Payload)...)
			collected++
		}
	}

	return result
}

// Close closes all Redis connections.
func (r *Client) Close() error {
	r.markAllChannelsClosed()
	r.cancelAllSubscriptions()
	r.unsubscribeAllChannels()
	r.waitForAllGoroutines()
	r.cleanupAllSubscriptions()

	errs := r.closeAllConnections()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// markAllChannelsClosed marks all channels as closed.
func (r *Client) markAllChannelsClosed() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for topic := range r.receiveChan {
		r.chanClosed[topic] = true
	}
}

// cancelAllSubscriptions cancels all subscription contexts.
func (r *Client) cancelAllSubscriptions() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for topic, cancel := range r.subCancel {
		cancel()
		delete(r.subCancel, topic)
	}
}

// unsubscribeAllChannels unsubscribes from all Redis channels.
func (r *Client) unsubscribeAllChannels() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for topic, pubSub := range r.subPubSub {
		if pubSub != nil {
			_ = pubSub.Unsubscribe(context.Background(), topic)
		}

		delete(r.subPubSub, topic)
	}
}

// waitForAllGoroutines waits for all subscription goroutines to finish.
func (r *Client) waitForAllGoroutines() {
	r.mu.RLock()
	topics := make([]string, 0, len(r.subWg))

	for topic := range r.subWg {
		topics = append(topics, topic)
	}

	r.mu.RUnlock()

	for _, topic := range topics {
		r.waitForGoroutine(topic)
	}
}

// cleanupAllSubscriptions cleans up all subscription resources.
func (r *Client) cleanupAllSubscriptions() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for topic, ch := range r.receiveChan {
		close(ch)
		delete(r.receiveChan, topic)
	}

	for topic := range r.subStarted {
		delete(r.subStarted, topic)
	}

	for topic := range r.subWg {
		delete(r.subWg, topic)
	}

	for topic := range r.chanClosed {
		delete(r.chanClosed, topic)
	}
}

// closeAllConnections closes all Redis connections.
func (r *Client) closeAllConnections() []error {
	var errs []error

	if r.pubConn != nil {
		if err := r.pubConn.Close(); err != nil && !strings.Contains(err.Error(), "client is closed") {
			errs = append(errs, err)
		}
	}

	if r.subConn != nil {
		if err := r.subConn.Close(); err != nil && !strings.Contains(err.Error(), "client is closed") {
			errs = append(errs, err)
		}
	}

	if r.queryConn != nil {
		if err := r.queryConn.Close(); err != nil && !strings.Contains(err.Error(), "client is closed") {
			errs = append(errs, err)
		}
	}

	return errs
}

// loggerAdapter adapts pubsub.Logger to our Logger interface.
type loggerAdapter struct {
	pubsub.Logger
}

func (l *loggerAdapter) Debug(args ...any) {
	l.Logger.Debug(args...)
}

func (l *loggerAdapter) Log(args ...any) {
	l.Logger.Log(args...)
}
