package factory

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/inmemory"
	"gofr.dev/pkg/cache/observability"
	"gofr.dev/pkg/cache/redis"
	"gofr.dev/pkg/gofr/logging"
)

type config struct {
	inMemoryOptions []inmemory.Option
	redisOptions    []redis.Option
	logger          logging.Logger
	metrics         *observability.Metrics
}

type Option func(*config)

// WithLogger sets a custom logging.Logger for the cache.
func WithLogger(logger logging.Logger) Option {
	return func(c *config) {
		c.logger = logger
	}
}

// WithObservabilityLogger sets a custom observability.Logger for both in-memory and Redis caches.
func WithObservabilityLogger(logger observability.Logger) Option {
	return func(c *config) {
		c.inMemoryOptions = append(c.inMemoryOptions, inmemory.WithLogger(logger))
		c.redisOptions = append(c.redisOptions, redis.WithLogger(logger))
	}
}

// WithMetrics sets a metrics collector for both in-memory and Redis caches.
func WithMetrics(metrics *observability.Metrics) Option {
	return func(c *config) {
		c.metrics = metrics
		c.inMemoryOptions = append(c.inMemoryOptions, inmemory.WithMetrics(metrics))
		c.redisOptions = append(c.redisOptions, redis.WithMetrics(metrics))
	}
}

// WithRedisAddr sets the Redis connection address.
func WithRedisAddr(addr string) Option {
	return func(c *config) {
		c.redisOptions = append(c.redisOptions, redis.WithAddr(addr))
	}
}

// WithRedisPassword sets the password for Redis authentication.
func WithRedisPassword(password string) Option {
	return func(c *config) {
		c.redisOptions = append(c.redisOptions, redis.WithPassword(password))
	}
}

// WithRedisDB sets the Redis database number.
func WithRedisDB(db int) Option {
	return func(c *config) {
		c.redisOptions = append(c.redisOptions, redis.WithDB(db))
	}
}

// WithTTL sets the time-to-live for cache entries (applies to both in-memory and Redis caches).
func WithTTL(ttl time.Duration) Option {
	return func(c *config) {
		c.inMemoryOptions = append(c.inMemoryOptions, inmemory.WithTTL(ttl))
		c.redisOptions = append(c.redisOptions, redis.WithTTL(ttl))
	}
}

// WithMaxItems sets the maximum number of items for in-memory cache.
func WithMaxItems(maxItems int) Option {
	return func(c *config) {
		c.inMemoryOptions = append(c.inMemoryOptions, inmemory.WithMaxItems(maxItems))
	}
}

type contextAwareLogger struct {
	logging.Logger
}

func extractTraceID(ctx context.Context) map[string]any {
	if ctx == nil {
		return nil
	}

	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.IsValid() {
		return map[string]any{"__trace_id__": sc.TraceID().String()}
	}

	return nil
}

func (l *contextAwareLogger) Errorf(ctx context.Context, format string, args ...any) {
	if traceMap := extractTraceID(ctx); traceMap != nil {
		args = append(args, traceMap)
	}

	l.Logger.Errorf(format, args...)
}

func (l *contextAwareLogger) Warnf(ctx context.Context, format string, args ...any) {
	if traceMap := extractTraceID(ctx); traceMap != nil {
		args = append(args, traceMap)
	}

	l.Logger.Warnf(format, args...)
}

func (l *contextAwareLogger) Infof(ctx context.Context, format string, args ...any) {
	if traceMap := extractTraceID(ctx); traceMap != nil {
		args = append(args, traceMap)
	}

	l.Logger.Infof(format, args...)
}

func (l *contextAwareLogger) Debugf(ctx context.Context, format string, args ...any) {
	if traceMap := extractTraceID(ctx); traceMap != nil {
		args = append(args, traceMap)
	}

	l.Logger.Debugf(format, args...)
}

func (l *contextAwareLogger) Hitf(ctx context.Context, message string, duration time.Duration, operation string) {
	args := []any{message, operation, duration}
	if traceMap := extractTraceID(ctx); traceMap != nil {
		args = append(args, traceMap)
	}

	l.Logger.Infof("%s: %s, duration: %s", args...)
}

func (l *contextAwareLogger) Missf(ctx context.Context, message string, duration time.Duration, operation string) {
	args := []any{message, operation, duration}
	if traceMap := extractTraceID(ctx); traceMap != nil {
		args = append(args, traceMap)
	}

	l.Logger.Infof("%s: %s, duration: %s", args...)
}

func (l *contextAwareLogger) LogRequest(ctx context.Context, level, message string, tag any, duration time.Duration, operation string) {
	logMessage := "message: %s, tag: %v, duration: %v, operation: %s"
	args := []any{message, tag, duration, operation}

	if traceMap := extractTraceID(ctx); traceMap != nil {
		args = append(args, traceMap)
	}

	switch level {
	case "INFO":
		l.Logger.Infof(logMessage, args...)
	case "DEBUG":
		l.Logger.Debugf(logMessage, args...)
	case "WARN":
		l.Logger.Warnf(logMessage, args...)
	case "ERROR":
		l.Logger.Errorf(logMessage, args...)
	default:
		args = append(args, "unsupported log level", level)
		l.Logger.Logf(logMessage, args...)
	}
}

func getTracer(name string) trace.Tracer {
	return otel.GetTracerProvider().Tracer(name)
}

type cacheBuilder interface {
	build(ctx context.Context, cfg *config) (cache.Cache, error)
}

type inMemoryBuilder struct {
	name string
}

func (b *inMemoryBuilder) build(ctx context.Context, cfg *config) (cache.Cache, error) {
	cfg.inMemoryOptions = append(cfg.inMemoryOptions, inmemory.WithName(b.name))

	c, err := inmemory.NewInMemoryCache(ctx, cfg.inMemoryOptions...)
	if err != nil {
		return nil, err
	}

	c.UseTracer(getTracer("gofr-inmemory-cache"))

	return c, nil
}

type redisBuilder struct {
	name string
}

func (b *redisBuilder) build(ctx context.Context, cfg *config) (cache.Cache, error) {
	cfg.redisOptions = append(cfg.redisOptions, redis.WithName(b.name))

	c, err := redis.NewRedisCache(ctx, cfg.redisOptions...)
	if err != nil {
		return nil, err
	}

	c.UseTracer(getTracer("gofr-redis-cache"))

	return c, nil
}

func newCacheWithBuilder(ctx context.Context, builder cacheBuilder, opts ...Option) (cache.Cache, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.logger != nil {
		adaptedLogger := &contextAwareLogger{Logger: cfg.logger}
		cfg.inMemoryOptions = append(cfg.inMemoryOptions, inmemory.WithLogger(adaptedLogger))
		cfg.redisOptions = append(cfg.redisOptions, redis.WithLogger(adaptedLogger))
	}

	return builder.build(ctx, cfg)
}

func NewInMemoryCache(ctx context.Context, name string, opts ...Option) (cache.Cache, error) {
	return newCacheWithBuilder(ctx, &inMemoryBuilder{name: name}, opts...)
}

func NewRedisCache(ctx context.Context, name string, opts ...Option) (cache.Cache, error) {
	return newCacheWithBuilder(ctx, &redisBuilder{name: name}, opts...)
}

func NewCache(ctx context.Context, cacheType, name string, opts ...Option) (cache.Cache, error) {
	switch cacheType {
	case "redis":
		return NewRedisCache(ctx, name, opts...)
	default:
		return NewInMemoryCache(ctx, name, opts...)
	}
}
