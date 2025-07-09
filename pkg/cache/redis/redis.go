package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/observability"
)

// Common errors
var (
	ErrEmptyKey  = errors.New("key cannot be empty")
	ErrNilValue  = errors.New("value cannot be nil")
	ErrNilClient = errors.New("redis client is nil")
)

type redisCache struct {
	client  *redis.Client
	ttl     time.Duration
	name    string
	logger  observability.Logger
	metrics *observability.Metrics
}

// Option configures the Redis cache
type Option func(*redisCache) error

// WithAddr sets the Redis server address
func WithAddr(addr string) Option {
	return func(c *redisCache) error {
		if addr == "" {
			return errors.New("address cannot be empty")
		}
		opts := c.client.Options()
		opts.Addr = addr
		c.client = redis.NewClient(opts)
		return nil
	}
}

// WithPassword sets the Redis server password
func WithPassword(password string) Option {
	return func(c *redisCache) error {
		opts := c.client.Options()
		opts.Password = password
		c.client = redis.NewClient(opts)
		return nil
	}
}

// WithDB sets the Redis database number
func WithDB(db int) Option {
	return func(c *redisCache) error {
		if db < 0 || db > 15 {
			return errors.New("database number must be between 0 and 15")
		}
		opts := c.client.Options()
		opts.DB = db
		c.client = redis.NewClient(opts)
		return nil
	}
}

// WithTTL sets the default TTL for cache entries
func WithTTL(ttl time.Duration) Option {
	return func(c *redisCache) error {
		if ttl < 0 {
			return errors.New("TTL cannot be negative")
		}
		c.ttl = ttl
		return nil
	}
}

// WithName sets a friendly name for the cache instance
func WithName(name string) Option {
	return func(c *redisCache) error {
		if name != "" {
			c.name = name
		}
		return nil
	}
}

// WithLogger sets the logger for the cache
func WithLogger(logger observability.Logger) Option {
	return func(c *redisCache) error {
		if logger != nil {
			c.logger = logger
		}
		return nil
	}
}

// WithMetrics sets the metrics for the cache
func WithMetrics(m *observability.Metrics) Option {
	return func(c *redisCache) error {
		if m != nil {
			c.metrics = m
		}
		return nil
	}
}

// NewRedisCache creates a new Redis-backed cache instance
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

	c.logger.Infof("Redis cache '%s' initialized on %s, DB %d, TTL=%v",
		c.name, c.client.Options().Addr, c.client.Options().DB, c.ttl)

	return c, nil
}

// validateKey ensures key is non-empty
func (c *redisCache) validateKey(key string) error {
	if key == "" {
		return ErrEmptyKey
	}
	return nil
}

// serializeValue converts a value to JSON for storage
func (c *redisCache) serializeValue(value interface{}) (string, error) {
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

// Set inserts or updates a cache entry with the default TTL
func (c *redisCache) Set(ctx context.Context, key string, value interface{}) error {
	start := time.Now()
	if err := c.validateKey(key); err != nil {
		c.logger.Errorf("Set failed: %v", err)
		return err
	}
	if value == nil {
		c.logger.Errorf("Set failed: %v", ErrNilValue)
		return ErrNilValue
	}

	serializedValue, err := c.serializeValue(value)
	if err != nil {
		c.logger.Errorf("Set failed to serialize value for key '%s': %v", key, err)
		return err
	}

	if err := c.client.Set(ctx, key, serializedValue, c.ttl).Err(); err != nil {
		c.logger.Errorf("Redis Set failed for key '%s': %v", key, err)
		return err
	}
	c.logger.Debugf("Set key '%s'", key)
	if c.metrics != nil {
		c.metrics.Sets().WithLabelValues(c.name).Inc()
		c.metrics.Items().WithLabelValues(c.name).Set(float64(c.countKeys(ctx)))
		c.metrics.Latency().WithLabelValues(c.name, "set").Observe(time.Since(start).Seconds())
	}
	return nil
}

// Get retrieves a cache entry
func (c *redisCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	start := time.Now()
	if err := c.validateKey(key); err != nil {
		c.logger.Errorf("Get failed: %v", err)
		return nil, false, err
	}

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debugf("Cache miss for key '%s'", key)
			if c.metrics != nil {
				c.metrics.Misses().WithLabelValues(c.name).Inc()
				c.metrics.Latency().WithLabelValues(c.name, "get").Observe(time.Since(start).Seconds())
			}
			return nil, false, nil // Key does not exist
		}
		c.logger.Errorf("Redis Get failed for key '%s': %v", key, err)
		return nil, false, err
	}

	c.logger.Debugf("Cache hit for key '%s'", key)
	if c.metrics != nil {
		c.metrics.Hits().WithLabelValues(c.name).Inc()
		c.metrics.Latency().WithLabelValues(c.name, "get").Observe(time.Since(start).Seconds())
	}
	return val, true, nil
}

// Delete removes a cache entry
func (c *redisCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	if err := c.validateKey(key); err != nil {
		c.logger.Errorf("Delete failed: %v", err)
		return err
	}

	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Errorf("Redis Del failed for key '%s': %v", key, err)
		return err
	}
	c.logger.Debugf("Deleted key '%s'", key)
	if c.metrics != nil {
		c.metrics.Deletes().WithLabelValues(c.name).Inc()
		c.metrics.Items().WithLabelValues(c.name).Set(float64(c.countKeys(ctx)))
		c.metrics.Latency().WithLabelValues(c.name, "delete").Observe(time.Since(start).Seconds())
	}
	return nil
}

// Exists checks if a key is in the cache
func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	start := time.Now()
	if err := c.validateKey(key); err != nil {
		c.logger.Errorf("Exists failed: %v", err)
		return false, err
	}

	res, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		c.logger.Errorf("Redis Exists failed for key '%s': %v", key, err)
		return false, err
	}
	if c.metrics != nil {
		c.metrics.Latency().WithLabelValues(c.name, "exists").Observe(time.Since(start).Seconds())
	}
	return res > 0, nil
}

// Clear removes all keys from the current database
func (c *redisCache) Clear(ctx context.Context) error {
	start := time.Now()
	c.logger.Warnf("Clearing redis cache '%s' (FLUSHDB)", c.name)
	if err := c.client.FlushDB(ctx).Err(); err != nil {
		c.logger.Errorf("Redis FlushDB failed: %v", err)
		return err
	}
	if c.metrics != nil {
		c.metrics.Items().WithLabelValues(c.name).Set(0)
		c.metrics.Latency().WithLabelValues(c.name, "clear").Observe(time.Since(start).Seconds())
	}
	return nil
}

// Close terminates the connection to the Redis server
func (c *redisCache) Close(ctx context.Context) error {
	if c.client == nil {
		return ErrNilClient
	}
	if err := c.client.Close(); err != nil {
		c.logger.Errorf("Failed to close redis client: %v", err)
		return err
	}
	c.logger.Infof("Redis cache '%s' closed", c.name)
	return nil
}

// countKeys returns the number of keys in the current Redis DB
func (c *redisCache) countKeys(ctx context.Context) int64 {
	res, err := c.client.DBSize(ctx).Result()
	if err != nil {
		c.logger.Errorf("DBSize failed: %v", err)
		return 0
	}
	return res
}
