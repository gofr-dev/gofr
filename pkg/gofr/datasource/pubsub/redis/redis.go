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
	// Handle Redis URI format: redis://user:password@host:port/db
	if strings.Contains(addr, "@") {
		// Find the last @ symbol to handle edge cases with multiple @ symbols
		lastAt := strings.LastIndex(addr, "@")
		if lastAt >= 0 && lastAt < len(addr)-1 {
			// Extract the host:port part (after last @)
			hostPart := addr[lastAt+1:]
			
			// Check if there's a scheme (redis:// or rediss://)
			if strings.HasPrefix(addr, "redis://") {
				return "redis://" + hostPart
			}
			if strings.HasPrefix(addr, "rediss://") {
				return "rediss://" + hostPart
			}
			
			// No scheme, just return host:port
			return hostPart
		}
	}
	
	// No credentials found, return as-is (safe for host:port format)
	return addr
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
		if r.logger != nil {
			r.logger.Errorf("could not initialize Redis, error: %v", err)
		}

		return
	}

	if r.logger != nil {
		r.logger.Debugf("connecting to Redis at '%s'", sanitizeRedisAddr(r.cfg.Addr))
	}

	options, err := createRedisOptions(r.cfg)
	if err != nil {
		if r.logger != nil {
			r.logger.Errorf("failed to create Redis options: %v", err)
		}

		return
	}

	// Create publisher connection
	r.pubConn = redis.NewClient(options)

	// Create subscriber connection (separate connection for blocking operations)
	subOptions := *options
	r.subConn = redis.NewClient(&subOptions)

	// Create query connection (separate connection for Query operations to avoid conflicts)
	queryOptions := *options
	r.queryConn = redis.NewClient(&queryOptions)

	if err := r.testConnections(); err != nil {
		if r.logger != nil {
			r.logger.Errorf("failed to connect to Redis at '%s', error: %v", sanitizeRedisAddr(r.cfg.Addr), err)
		}

		go r.retryConnect()

		return
	}

	// Test query connection
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.DialTimeout)
	defer cancel()

	if err := r.queryConn.Ping(ctx).Err(); err != nil {
		if r.logger != nil {
			r.logger.Errorf("failed to connect query connection to Redis at '%s', error: %v", sanitizeRedisAddr(r.cfg.Addr), err)
		}
	}

	if r.logger != nil {
		r.logger.Logf("connected to Redis at '%s'", sanitizeRedisAddr(r.cfg.Addr))
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
			if r.logger != nil {
				r.logger.Errorf("failed to create Redis options during retry: %v", err)
			}

			continue
		}

		// Recreate connections
		if r.pubConn != nil {
			_ = r.pubConn.Close()
		}

		if r.subConn != nil {
			_ = r.subConn.Close()
		}

		if r.queryConn != nil {
			_ = r.queryConn.Close()
		}

		r.pubConn = redis.NewClient(options)
		subOptions := *options
		r.subConn = redis.NewClient(&subOptions)
		queryOptions := *options
		r.queryConn = redis.NewClient(&queryOptions)

		ctx, cancel := context.WithTimeout(context.Background(), r.cfg.DialTimeout)
		pubErr := r.pubConn.Ping(ctx).Err()
		subErr := r.subConn.Ping(ctx).Err()
		queryErr := r.queryConn.Ping(ctx).Err()

		cancel()

		if pubErr != nil || subErr != nil || queryErr != nil {
			if r.logger != nil {
				r.logger.Errorf("could not connect to Redis at '%s', pub error: %v, sub error: %v, query error: %v",
					sanitizeRedisAddr(r.cfg.Addr), pubErr, subErr, queryErr)
			}

			continue
		}

		if r.logger != nil {
			r.logger.Logf("reconnected to Redis at '%s'", sanitizeRedisAddr(r.cfg.Addr))
		}

		// Restart subscriptions if they were active
		r.restartSubscriptions()

		return
	}
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
		if r.logger != nil {
			r.logger.Errorf("subscriber connection is nil for topic '%s'", topic)
		}

		return
	}

	redisPubSub := r.subConn.Subscribe(ctx, topic)
	if redisPubSub == nil {
		if r.logger != nil {
			r.logger.Errorf("failed to create PubSub connection for topic '%s'", topic)
		}

		return
	}

	// Store PubSub connection for potential unsubscribe
	r.mu.Lock()
	r.subPubSub[topic] = redisPubSub
	r.mu.Unlock()

	defer func() {
		if redisPubSub != nil {
			redisPubSub.Close()
		}

		// Clean up from map
		r.mu.Lock()
		delete(r.subPubSub, topic)
		r.mu.Unlock()
	}()

	ch := redisPubSub.Channel()
	if ch == nil {
		if r.logger != nil {
			r.logger.Errorf("failed to get channel from PubSub for topic '%s'", topic)
		}

		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				// Channel closed, try to reconnect
				if r.logger != nil {
					r.logger.Debugf("Redis subscription channel closed for topic '%s', attempting reconnect", topic)
				}

				return
			}

			if msg == nil {
				continue
			}

			// Create pubsub.Message
			m := pubsub.NewMessage(ctx)
			m.Topic = topic
			m.Value = []byte(msg.Payload)
			m.Committer = newRedisMessage(msg, r.logger)

			// Check if channel is closed before writing (race condition fix)
			r.mu.RLock()
			msgChan, exists := r.receiveChan[topic]
			closed := r.chanClosed[topic]
			r.mu.RUnlock()

			if exists && !closed {
				select {
				case msgChan <- m:
				case <-ctx.Done():
					return
				default:
					// Channel full, log warning
					if r.logger != nil {
						r.logger.Debugf("message channel full for topic '%s', dropping message", topic)
					}
				}
			}
		}
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
	defer r.mu.Unlock()

	// Check if subscription exists
	_, exists := r.subStarted[topic]
	if !exists {
		// Already unsubscribed or never subscribed
		return nil
	}

	// Mark channel as closed first to prevent writes
	r.chanClosed[topic] = true

	// Unsubscribe from Redis channel first (before canceling context)
	if pubSub, ok := r.subPubSub[topic]; ok && pubSub != nil {
		ctx, cancel := context.WithTimeout(context.Background(), unsubscribeOpTimeout)
		if err := pubSub.Unsubscribe(ctx, topic); err != nil {
			if r.logger != nil {
				r.logger.Errorf("error while unsubscribing from Redis channel '%s', error: %v", topic, err)
			}

			// Continue with cleanup even if unsubscribe fails
		}

		cancel()
	}

	// Cancel the subscription context to stop the goroutine
	// The goroutine will handle closing the PubSub connection in its defer
	if cancel, ok := r.subCancel[topic]; ok {
		cancel()
		delete(r.subCancel, topic)
	}

	// Wait for goroutine to finish (with timeout)
	if wg, ok := r.subWg[topic]; ok {
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

	// Close and remove the receive channel (after goroutine is done or timed out)
	if ch, ok := r.receiveChan[topic]; ok {
		close(ch)
		delete(r.receiveChan, topic)
	}

	// Remove from started subscriptions and closed tracking
	delete(r.subStarted, topic)
	delete(r.chanClosed, topic)

	if r.logger != nil {
		r.logger.Debugf("unsubscribed from Redis channel '%s'", topic)
	}

	return nil
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

	// Use queryConn if available, otherwise fallback to subConn
	connToUse := r.queryConn
	if connToUse == nil {
		if r.subConn == nil {
			return nil, errClientNotConnected
		}

		connToUse = r.subConn
	}

	timeout, limit := parseQueryArgs(args...)

	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Subscribe to channel for query using query connection
	redisPubSub := connToUse.Subscribe(queryCtx, query)
	if redisPubSub == nil {
		return nil, errPubSubConnectionFailed
	}

	defer redisPubSub.Close()

	ch := redisPubSub.Channel()
	if ch == nil {
		return nil, errPubSubChannelFailed
	}

	var result []byte

	collected := 0

	for collected < limit {
		select {
		case <-queryCtx.Done():
			return result, nil
		case msg, ok := <-ch:
			if !ok {
				return result, nil
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

	return result, nil
}

// Close closes all Redis connections.
func (r *Client) Close() error {
	var errs []error

	// Cancel all subscriptions and wait for goroutines
	r.mu.Lock()

	// Mark all channels as closed first
	for topic := range r.receiveChan {
		r.chanClosed[topic] = true
	}

	// Cancel all subscription contexts
	for topic, cancel := range r.subCancel {
		cancel()
		delete(r.subCancel, topic)
	}

	// Unsubscribe from all Redis channels
	for topic, pubSub := range r.subPubSub {
		if pubSub != nil {
			_ = pubSub.Unsubscribe(context.Background(), topic)
		}

		delete(r.subPubSub, topic)
	}

	r.mu.Unlock()

	// Wait for all goroutines to finish (outside lock to avoid deadlock)
	for topic, wg := range r.subWg {
		done := make(chan struct{})

		go func(w *sync.WaitGroup) {
			w.Wait()
			close(done)
		}(wg)

		select {
		case <-done:
		case <-time.After(goroutineWaitTimeout):
			if r.logger != nil {
				r.logger.Debugf("timeout waiting for subscription goroutine for topic '%s'", topic)
			}
		}
	}

	// Close channels after goroutines are done
	r.mu.Lock()

	for topic, ch := range r.receiveChan {
		close(ch)
		delete(r.receiveChan, topic)
	}

	// Clear subscription state
	for topic := range r.subStarted {
		delete(r.subStarted, topic)
	}

	// Clear WaitGroups and closed tracking
	for topic := range r.subWg {
		delete(r.subWg, topic)
	}

	for topic := range r.chanClosed {
		delete(r.chanClosed, topic)
	}

	r.mu.Unlock()

	// Close connections
	// Ignore "client is closed" errors as connections may already be closed
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

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
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
