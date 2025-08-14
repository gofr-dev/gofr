# Gofr Cache Example: In-Memory & Redis

## Overview

This example provides a hands-on demonstration of Gofr's cache layer. It showcases how to instantiate, configure, and use both **in-memory** and **Redis-backed** caches. The cache is designed with a unified interface, making it easy to swap between different backend implementations.

The example is also fully observable, exposing a rich set of Prometheus metrics for cache performance monitoring. A pre-configured monitoring stack is provided to visualize these metrics with Grafana.

---

## Directory Structure

```
using-cache/
├── main.go          # Example Go application using the cache
├── monitoring.sh    # Script to launch the monitoring stack
└── README.md        # This file
```

---

## Getting Started

### 1. Run the Go Example

First, run the sample application. It will start an HTTP server to expose Prometheus metrics on port `8080`.

```sh
go run main.go
```

You should see output indicating that the metrics server is running. The application will continuously perform cache operations in the background to generate metrics.

### 2. Launch the Monitoring Stack

To visualize the cache metrics, open a new terminal and run the `monitoring.sh` script. This will start Prometheus and Grafana in Docker containers.

```sh
./monitoring.sh
```

This script is a convenience wrapper that navigates to the centralized monitoring setup located in `pkg/cache/monitoring` and starts the Docker Compose stack.

Once the script completes, you can access the following:

- **Application Metrics**: [http://localhost:8080/metrics](http://localhost:8080/metrics)
- **Grafana Dashboard**: [http://localhost:3000](http://localhost:3000) (user: `admin`, pass: `admin`)
- **Prometheus UI**: [http://localhost:9090](http://localhost:9090)

---

## The `cache.Cache` Interface

All cache implementations in Gofr adhere to the `cache.Cache` interface, ensuring consistent and predictable behavior regardless of the backend. This interface is defined in `pkg/cache/cache.go`.

```go
type Cache interface {
    Get(ctx context.Context, key string) (interface{}, bool, error)
    Set(ctx context.Context, key string, value interface{}) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    Clear(ctx context.Context) error
    Close(ctx context.Context) error
}
```

- **`Get(key)`**: Retrieves an item. Returns the value, a boolean indicating if the item was found, and an error.
- **`Set(key, value)`**: Stores an item. This will overwrite an existing item with the same key.
- **`Delete(key)`**: Removes an item.
- **`Exists(key)`**: Checks for an item's existence without retrieving it.
- **`Clear()`**: Removes all items from the cache.
- **`Close()`**: Releases any resources held by the cache.

---

## Implementation & Usage

Gofr provides two main ways to create a cache:
1.  **Direct Instantiation**: Using the `New...Cache` constructor from the specific implementation package (`inmemory` or `redis`). This gives you full access to all configuration options.
2.  **Factory**: Using the `factory` package, which provides a convenient, unified way to create caches, especially when the cache type might be determined at runtime.

### Direct Instantiation

This is the recommended approach when you know the specific cache type you need and want to use all its features.

#### Example: Creating an In-Memory Cache

The `inmemory` package provides a constructor `NewInMemoryCache` that accepts functional options for detailed configuration.

```go
import (
    "context"
    "time"
    "gofr.dev/pkg/cache/inmemory"
    "gofr.dev/pkg/cache/observability"
)

// 1. Create a metrics collector and logger
metrics := observability.NewMetrics("gofr", "inmemory_cache")
logger := observability.NewStdLogger()

// 2. Create the cache instance with detailed options
c, err := inmemory.NewInMemoryCache(context.Background(),
    inmemory.WithName("my-inmemory-cache"), // A unique name for the cache
    inmemory.WithTTL(5*time.Minute),       // Default time-to-live for items
    inmemory.WithMaxItems(1000),           // Maximum number of items (for LRU eviction)
    inmemory.WithLogger(logger),
    inmemory.WithMetrics(metrics),
)
```

The `inmemory` package also includes convenient presets:
- **`NewDefaultCache(ctx, name)`**: Creates a cache with a 5-minute TTL and 1000-item limit.
- **`NewDebugCache(ctx, name)`**: Creates a cache with a 1-minute TTL and 100-item limit, useful for development.
- **`NewProductionCache(ctx, name, ttl, maxItems)`**: Creates a cache with explicitly defined TTL and size for production use.

#### Example: Creating a Redis Cache

Similarly, the `redis` package provides a constructor for creating a Redis-backed cache.

```go
import (
    "context"
    "time"
    "gofr.dev/pkg/cache/redis"
    "gofr.dev/pkg/cache/observability"
)

// 1. Create a metrics collector and logger
metrics := observability.NewMetrics("gofr", "redis_cache")
logger := observability.NewStdLogger()

// 2. Create the Redis cache instance
c, err := redis.NewRedisCache(context.Background(),
    redis.WithName("my-redis-cache"),
    redis.WithTTL(10*time.Minute),
    redis.WithAddr("localhost:6379"),
    redis.WithPassword("your-password"),
    redis.WithDB(0),
    redis.WithLogger(logger),
    redis.WithMetrics(metrics),
)
```

### Using the Factory

The `gofr.dev/pkg/cache/factory` package is useful for creating a cache when the type might be configured dynamically.

#### Example: Generic Cache Factory

The `NewCache` function creates an instance based on a type string (`"inmemory"` or `"redis"`).

```go
import "gofr.dev/pkg/cache/factory"

// Create an in-memory cache via the factory
inMemoryCache, err := factory.NewCache(ctx, "inmemory", "my-inmemory-cache", 5*time.Minute, 1000)

// Create a Redis cache via the factory
// Note: The generic factory has limited options for Redis.
// For full control (e.g., password, DB), use direct instantiation.
redisCache, err := factory.NewCache(ctx, "redis", "my-redis-cache", 10*time.Minute, 0, "localhost:6379")
```

The factory also provides specific constructors like `factory.NewInMemoryCache` and `factory.NewRedisCache`, which offer a subset of the options available through direct instantiation.

---

## Configuration & Options

Both cache types are configured using functional options passed to their constructors.

### In-Memory Cache Options (`inmemory.Option`)

- **`ctx`**: A cancellable context.
- **`WithName(string)`**: A logical name for the cache, used in logs and metrics.
- **`WithTTL(time.Duration)`**: The default time-to-live for cache entries.
- **`WithMaxItems(int)`**: The maximum number of items before LRU eviction is triggered. `0` means no limit.
- **`WithLogger(observability.Logger)`**: A custom logger.
- **`WithMetrics(*observability.Metrics)`**: A metrics collector.

### Redis Cache Options (`redis.Option`)

- **`ctx`**: A cancellable context.
- **`WithName(string)`**: A logical name for the cache.
- **`WithTTL(time.Duration)`**: The default time-to-live for entries.
- **`WithAddr(string)`**: The Redis server address (e.g., `"localhost:6379"`).
- **`WithPassword(string)`**: The Redis server password.
- **`WithDB(int)`**: The Redis database number (0-15 is the default for most Redis setups).
- **`WithLogger(observability.Logger)`**: A custom logger.
- **`WithMetrics(*observability.Metrics)`**: A metrics collector.

---

## Observability: Logging & Metrics

### Logging

The cache components produce structured, colored logs for better readability. You can provide your own logger implementation that satisfies the `observability.Logger` interface, or use the provided `NewStdLogger()` or `NewNopLogger()` (to disable logging).

### Metrics

The cache exposes a comprehensive set of Prometheus metrics. The exact metric names depend on the namespace and subsystem provided when creating the `*observability.Metrics` instance (e.g., `observability.NewMetrics("gofr", "inmemory_cache")`).

All metrics are labeled with `cache_name` to distinguish between different cache instances.

#### Common Metrics

These metrics are available for both **in-memory** and **redis** caches. Replace `{backend}` with `inmemory_cache` or `redis_cache`.

- **`gofr_{backend}_hits_total`**: Total cache hits.
- **`gofr_{backend}_misses_total`**: Total cache misses.
- **`gofr_{backend}_sets_total`**: Total set operations.
- **`gofr_{backend}_deletes_total`**: Total delete operations.
- **`gofr_{backend}_items_current`**: Current number of items in the cache. For Redis, this is the `DBSIZE`.
- **`gofr_{backend}_operation_latency_seconds`**: Latency histogram for cache operations.

#### In-Memory Only Metrics

- **`gofr_inmemory_cache_evictions_total`**: Total items evicted due to capacity limits.