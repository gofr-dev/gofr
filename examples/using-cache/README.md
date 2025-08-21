# Gofr Cache: In-Memory & Redis

## Overview

Gofr provides a unified cache interface with two implementations: **in-memory** and **Redis-backed**. Both implementations support the same `cache.Cache` interface, making them interchangeable in your application.

## Cache Interface

All cache implementations implement the `cache.Cache` interface:

```go
type Cache interface {
    Get(ctx context.Context, key string) (any, bool, error)
    Set(ctx context.Context, key string, value any) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    Clear(ctx context.Context) error
    Close(ctx context.Context) error
    UseTracer(tracer trace.Tracer)
}
```

## Usage

### Method 1: Using App Convenience Methods (Recommended)

```go
import "gofr.dev/pkg/gofr"

app := gofr.New()

// Add in-memory cache
app.AddInMemoryCache(ctx, "my-cache", 5*time.Minute, 1000)

// Add Redis cache
app.AddRedisCache(ctx, "my-redis-cache", 10*time.Minute, "localhost:6379")

// Get cache instance
cache := app.GetCache("my-cache")
```

### Method 2: Direct Instantiation

#### In-Memory Cache

```go
import "gofr.dev/pkg/cache/inmemory"

cache, err := inmemory.NewInMemoryCache(ctx,
    inmemory.WithName("my-cache"),
    inmemory.WithTTL(5*time.Minute),
    inmemory.WithMaxItems(1000),
)
```

**Configuration Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithTTL(duration)` | `time.Duration` | `1 minute` | Default TTL for cache entries. Zero disables expiration |
| `WithMaxItems(int)` | `int` | `0` (no limit) | Maximum items before LRU eviction |
| `WithName(string)` | `string` | `"default"` | Cache name for logging/metrics |
| `WithLogger(logger)` | `observability.Logger` | `NewStdLogger()` | Custom logger implementation |
| `WithMetrics(metrics)` | `*observability.Metrics` | `NewMetrics("gofr", "inmemory_cache")` | Prometheus metrics collector |

#### Redis Cache

```go
import "gofr.dev/pkg/cache/redis"

cache, err := redis.NewRedisCache(ctx,
    redis.WithName("my-redis-cache"),
    redis.WithTTL(10*time.Minute),
    redis.WithAddr("localhost:6379"),
    redis.WithPassword("password"),
    redis.WithDB(0),
)
```

**Configuration Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithTTL(duration)` | `time.Duration` | `1 minute` | Default TTL for cache entries |
| `WithAddr(string)` | `string` | `"localhost:6379"` | Redis server address |
| `WithPassword(string)` | `string` | `""` | Redis authentication password |
| `WithDB(int)` | `int` | `0` | Redis database number (0-255) |
| `WithName(string)` | `string` | `"default-redis"` | Cache name for logging/metrics |
| `WithLogger(logger)` | `observability.Logger` | `NewStdLogger()` | Custom logger implementation |
| `WithMetrics(metrics)` | `*observability.Metrics` | `NewMetrics("gofr", "redis_cache")` | Prometheus metrics collector |

### Method 3: Factory Pattern

```go
import "gofr.dev/pkg/cache/factory"

// In-memory cache
cache, err := factory.NewInMemoryCache(ctx, "my-cache",
    factory.WithTTL(5*time.Minute),
    factory.WithMaxItems(1000),
)

// Redis cache
cache, err := factory.NewRedisCache(ctx, "my-redis-cache",
    factory.WithTTL(10*time.Minute),
    factory.WithRedisAddr("localhost:6379"),
)
```

## Features

### In-Memory Cache
- **LRU eviction**: Automatically removes least recently used items when capacity is reached
- **TTL support**: Automatic expiration of cache entries
- **Thread-safe**: Concurrent access supported with RWMutex
- **Background cleanup**: Periodic cleanup of expired items
- **O(1) operations**: Get, Set, Delete operations are constant time
- **Memory efficient**: Uses doubly-linked list for LRU implementation

### Redis Cache
- **Persistence**: Data survives application restarts
- **TTL support**: Automatic expiration handled by Redis
- **Connection pooling**: Managed by go-redis client
- **Serialization**: Automatic JSON serialization for complex types
- **Network resilience**: Built-in retry and connection management
- **Cluster support**: Can connect to Redis Cluster or Sentinel

### Common Features
- **Observability**: Built-in logging and Prometheus metrics
- **Tracing**: OpenTelemetry integration with span attributes
- **Error handling**: Comprehensive error types and validation
- **Context support**: All operations accept context for cancellation/timeout
- **Type safety**: Strong typing with proper error handling

## Monitoring & Observability

### Metrics

Both cache implementations expose comprehensive Prometheus metrics:

#### Common Metrics
- `gofr_{backend}_hits_total`: Total cache hits
- `gofr_{backend}_misses_total`: Total cache misses  
- `gofr_{backend}_sets_total`: Total set operations
- `gofr_{backend}_deletes_total`: Total delete operations
- `gofr_{backend}_items_current`: Current number of items
- `gofr_{backend}_operation_latency_seconds`: Operation latency histogram

#### In-Memory Only Metrics
- `gofr_inmemory_cache_evictions_total`: Items evicted due to capacity limits

Replace `{backend}` with `inmemory_cache` or `redis_cache`.

### Logging

Cache operations are logged with structured information:
- Operation type (GET, SET, DELETE, etc.)
- Cache name
- Key being operated on
- Duration
- Success/failure status

### Tracing

OpenTelemetry spans are created for each cache operation with attributes:
- `cache.name`: Cache instance name
- `cache.key`: Key being operated on
- `cache.operation`: Operation type

## Docker & Monitoring Stack

### Quick Start with Monitoring

The example includes a pre-configured monitoring stack with Prometheus and Grafana:

```bash
# Start the application
go run main.go

# In another terminal, start the monitoring stack
./monitoring.sh
```

### Monitoring Stack Components

#### Prometheus Configuration
```yaml
# pkg/cache/monitoring/prometheus.yml
global:
  scrape_interval: 5s
scrape_configs:
  - job_name: 'gofr-cache'
    static_configs:
      - targets: ['host.docker.internal:8080']
```

#### Grafana Setup
- **URL**: http://localhost:3000
- **Username**: `admin`
- **Password**: `admin`
- **Pre-configured**: Prometheus data source and cache metrics dashboard

#### Docker Compose Services
```yaml
services:
  prometheus:
    image: prom/prometheus:latest
    ports: ["9090:9090"]
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports: ["3000:3000"]
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - ./provisioning:/etc/grafana/provisioning
```

### Manual Monitoring Setup

If you prefer to set up monitoring manually:

1. **Start Redis** (if using Redis cache):
```bash
docker run -d --name redis -p 6379:6379 redis:alpine
```

2. **Start Prometheus**:
```bash
docker run -d --name prometheus -p 9090:9090 \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus:latest
```

3. **Start Grafana**:
```bash
docker run -d --name grafana -p 3000:3000 \
  -e GF_SECURITY_ADMIN_PASSWORD=admin \
  grafana/grafana:latest
```

## Example Application

The `main.go` file demonstrates:

1. **Cache initialization** using app convenience methods
2. **Metrics exposure** on HTTP endpoint (port 2121)
3. **Continuous operations** to generate observable metrics
4. **Graceful shutdown** handling with signal management

### Running the Example

```bash
# Start the application
go run main.go

# View Grafana dashboard (after starting monitoring stack)
http://localhost:3000
```

### Application Configuration

The example application uses these default settings:
- **Metrics port**: 2121 (configurable via `METRICS_PORT` env var)
- **Cache TTL**: 5 minutes
- **Max items**: 1000 (in-memory only)
- **Redis address**: localhost:6379

## Error Handling

### Common Errors

| Error | Description | Resolution |
|-------|-------------|------------|
| `ErrCacheClosed` | Operation attempted on closed cache | Ensure cache is not closed |
| `ErrEmptyKey` | Empty key provided | Provide non-empty key |
| `ErrNilValue` | Nil value provided to Set | Provide non-nil value |
| `ErrInvalidMaxItems` | Negative maxItems value | Use non-negative value |
| `ErrAddressEmpty` | Empty Redis address | Provide valid Redis address |
| `ErrInvalidDatabaseNumber` | Invalid Redis DB number | Use 0-255 range |
| `ErrNegativeTTL` | Negative TTL value | Use non-negative duration |

### Error Handling Best Practices

```go
// Always check for errors
value, found, err := cache.Get(ctx, "key")
if err != nil {
    log.Printf("Cache get error: %v", err)
    // Handle error appropriately
    return err
}

// Check if key exists
if !found {
    // Key doesn't exist, handle accordingly
    return nil
}
```
