package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/observability"
)

// Common errors.
var (
	// ErrEmptyKey is returned when an operation is attempted with an empty key.
	ErrEmptyKey = errors.New("key cannot be empty")
	// ErrNilValue is returned when a nil value is provided to Set.
	ErrNilValue = errors.New("value cannot be nil")
	// ErrNilClient is returned when the Redis client is not initialized.
	ErrNilClient = errors.New("redis client is nil")
	// ErrAddressEmpty is returned when an empty address is provided.
	ErrAddressEmpty = errors.New("address cannot be empty")
	// ErrInvalidDatabaseNumber is returned when a database number outside the valid range is provided.
	ErrInvalidDatabaseNumber = errors.New("database number must be between 0 and 255")
	// ErrNegativeTTL is returned when a negative TTL is provided.
	ErrNegativeTTL = errors.New("TTL cannot be negative")
)

type redisCache struct {
	client  *redis.Client
	ttl     time.Duration
	name    string
	logger  observability.Logger
	metrics *observability.Metrics
	tracer  *trace.Tracer
}

type Option func(*redisCache) error

// WithAddr sets the network address of the Redis server (e.g., "localhost:6379").
func WithAddr(addr string) Option {
	return func(c *redisCache) error {
		if addr == "" {
			return ErrAddressEmpty
		}

		opts := c.client.Options()
		opts.Addr = addr
		c.client = redis.NewClient(opts)

		return nil
	}
}

// WithPassword sets the password for authenticating with the Redis server.
func WithPassword(password string) Option {
	return func(c *redisCache) error {
		opts := c.client.Options()
		opts.Password = password
		c.client = redis.NewClient(opts)

		return nil
	}
}

// WithDB sets the Redis database number to use.
// The database number must be between 0 and 255.
func WithDB(db int) Option {
	return func(c *redisCache) error {
		if db < 0 || db > 255 {
			return ErrInvalidDatabaseNumber
		}

		opts := c.client.Options()
		opts.DB = db
		c.client = redis.NewClient(opts)

		return nil
	}
}

// WithTTL sets the default time-to-live (TTL) for all entries in the cache.
// Redis will automatically remove items after this duration.
// A TTL of zero means items will not expire.
func WithTTL(ttl time.Duration) Option {
	return func(c *redisCache) error {
		if ttl < 0 {
			return ErrNegativeTTL
		}

		c.ttl = ttl

		return nil
	}
}

// WithName sets a descriptive name for the cache instance.
// This name is used in logs and metrics to identify the cache.
func WithName(name string) Option {
	return func(c *redisCache) error {
		if name != "" {
			c.name = name
		}

		return nil
	}
}

// WithLogger provides a custom logger for the cache.
// If not provided, a default standard library logger is used.
func WithLogger(logger observability.Logger) Option {
	return func(c *redisCache) error {
		if logger != nil {
			c.logger = logger
		}

		return nil
	}
}

// WithMetrics provides a metrics collector for the cache.
// If provided, the cache will record metrics for its operations.
func WithMetrics(m *observability.Metrics) Option {
	return func(c *redisCache) error {
		if m != nil {
			c.metrics = m
		}

		return nil
	}
}

// NewRedisCache creates and returns a new Redis-backed cache instance.
// It establishes a connection to the Redis server and pings it to ensure connectivity.
// It takes zero or more Option functions to customize its configuration.
// By default, it connects to "localhost:6379" with a 1-minute TTL.
func NewRedisCache(ctx context.Context, opts ...Option) (cache.Cache, error) {
	// Default client connects to localhost:6379
	defaultClient := redis.NewClient(&redis.Options{})

	c := &redisCache{
		client:  defaultClient,
		ttl:     time.Minute,
		name:    "default-redis",
		logger:  observability.NewStdLogger(),
		metrics: observability.NewMetrics("gofr", "redis_cache"),
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("failed to configure redis cache: %w", err)
		}
	}

	// Verify the connection is alive
	if err := c.client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	c.logger.Infof(ctx, "Redis cache '%s' initialized on %s, DB %d, TTL=%v",
		c.name, c.client.Options().Addr, c.client.Options().DB, c.ttl)

	return c, nil
}

func (c *redisCache) UseTracer(tracer trace.Tracer) {
	c.tracer = &tracer
}

// validateKey ensures key is non-empty.
func validateKey(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	return nil
}

// serializeValue converts a value to JSON for storage.
func (*redisCache) serializeValue(value any) (string, error) {
	// Handle simple types directly to maintain readability in Redis
	switch v := value.(type) {
	case string:
		return v, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		if v {
			return "true", nil
		}

		return "false", nil
	default:
		// For complex types, use JSON
		bytes, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("failed to serialize value: %w", err)
		}

		return string(bytes), nil
	}
}

// Set adds or updates a key-value pair in the Redis cache with the default TTL.
// The value is serialized before being stored. Simple types are stored as strings,
// while complex types are JSON-marshaled.
// This operation is thread-safe.
func (c *redisCache) Set(ctx context.Context, key string, value any) error {
	start := time.Now()

	if err := validateKey(key); err != nil {
		c.logger.Errorf(ctx, "Set failed: %v", err)
		return err
	}

	if value == nil {
		c.logger.Errorf(ctx, "Set failed: %v", ErrNilValue)
		return ErrNilValue
	}

	serializedValue, err := c.serializeValue(value)
	if err != nil {
		c.logger.Errorf(ctx, "Set failed to serialize value for key '%s': %v", key, err)
		return err
	}

	if err := c.client.Set(ctx, key, serializedValue, c.ttl).Err(); err != nil {
		c.logger.Errorf(ctx, "Redis Set failed for key '%s': %v", key, err)
		return err
	}

	duration := time.Since(start)
	c.logger.LogRequest(ctx, "DEBUG", "Set new cache key", "SUCCESS", duration, key)

	if c.metrics != nil {
		c.metrics.Sets().WithLabelValues(c.name).Inc()
		c.metrics.Items().WithLabelValues(c.name).Set(float64(c.countKeys(ctx)))
		c.metrics.Latency().WithLabelValues(c.name, "set").Observe(duration.Seconds())
	}

	return nil
}

// Get retrieves an item from the Redis cache.
// If the key is found, it returns the stored value and true.
// The caller is responsible for deserializing it if necessary.
// If the key is not found, it returns nil and false.
// This operation is thread-safe.
func (c *redisCache) Get(ctx context.Context, key string) (value any, found bool, err error) {
	start := time.Now()

	if keyerr := validateKey(key); keyerr != nil {
		c.logger.Errorf(ctx, "Get failed: %v", keyerr)
		return nil, false, keyerr
	}

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			duration := time.Since(start)
			c.logger.Missf(ctx, "GET", duration, key)

			if c.metrics != nil {
				c.metrics.Misses().WithLabelValues(c.name).Inc()
				c.metrics.Latency().WithLabelValues(c.name, "get").Observe(duration.Seconds())
			}

			return nil, false, nil // Key does not exist
		}

		c.logger.Errorf(ctx, "Redis Get failed for key '%s': %v", key, err)

		return nil, false, err
	}

	duration := time.Since(start)
	c.logger.Hitf(ctx, "GET", duration, key)

	if c.metrics != nil {
		c.metrics.Hits().WithLabelValues(c.name).Inc()
		c.metrics.Latency().WithLabelValues(c.name, "get").Observe(duration.Seconds())
	}

	return val, true, nil
}

// Delete removes a key from the Redis cache.
// If the key does not exist, the operation is a no-op.
// This operation is thread-safe.
func (c *redisCache) Delete(ctx context.Context, key string) error {
	start := time.Now()

	if err := validateKey(key); err != nil {
		c.logger.Errorf(ctx, "Delete failed: %v", err)
		return err
	}

	duration := time.Since(start)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Errorf(ctx, "Redis Del failed for key '%s': %v", key, err)
		return err
	}

	c.logger.LogRequest(ctx, "DEBUG", "Deleted cache key", "SUCCESS", duration, key)

	if c.metrics != nil {
		c.metrics.Deletes().WithLabelValues(c.name).Inc()
		c.metrics.Items().WithLabelValues(c.name).Set(float64(c.countKeys(ctx)))
		c.metrics.Latency().WithLabelValues(c.name, "delete").Observe(duration.Seconds())
	}

	return nil
}

// Exists checks if a key exists in the Redis cache.
// It returns true if the key is present, false otherwise.
// This operation is thread-safe.
func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	start := time.Now()

	if err := validateKey(key); err != nil {
		c.logger.Errorf(ctx, "Exists failed: %v", err)
		return false, err
	}

	res, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		c.logger.Errorf(ctx, "Redis Exists failed for key '%s': %v", key, err)
		return false, err
	}

	if c.metrics != nil {
		c.metrics.Latency().WithLabelValues(c.name, "exists").Observe(time.Since(start).Seconds())
	}

	return res > 0, nil
}

// Clear removes all keys from the current Redis database (using FLUSHDB).
// This is a destructive operation and should be used with caution.
// This operation is thread-safe.
func (c *redisCache) Clear(ctx context.Context) error {
	start := time.Now()

	if err := c.client.FlushDB(ctx).Err(); err != nil {
		c.logger.Errorf(ctx, "Redis FlushDB failed: %v", err)
		return err
	}

	duration := time.Since(start)
	c.logger.LogRequest(ctx, "WARN", "Cleared all keys", "SUCCESS", duration, c.name)

	if c.metrics != nil {
		c.metrics.Items().WithLabelValues(c.name).Set(0)
		c.metrics.Latency().WithLabelValues(c.name, "clear").Observe(duration.Seconds())
	}

	return nil
}

// Close closes the connection to the Redis server.
// It's important to call Close to release network resources.
func (c *redisCache) Close(ctx context.Context) error {
	if c.client == nil {
		return ErrNilClient
	}

	if err := c.client.Close(); err != nil {
		c.logger.Errorf(ctx, "Failed to close redis client: %v", err)
		return err
	}

	c.logger.Infof(ctx, "Redis cache '%s' closed", c.name)

	return nil
}

// countKeys returns the number of keys in the current Redis DB.
func (c *redisCache) countKeys(ctx context.Context) int64 {
	res, err := c.client.DBSize(ctx).Result()
	if err != nil {
		c.logger.Errorf(ctx, "DBSize failed: %v", err)
		return 0
	}

	return res
}
