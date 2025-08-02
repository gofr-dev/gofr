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

The `main.go` file demonstrates how to use the cache `factory` to create cache instances.

### Cache Factory

The `gofr.dev/pkg/cache/factory` package provides a convenient way to create different types of caches.

- **`NewInMemoryCache(...)`**: Creates an in-memory cache.
- **`NewRedisCache(...)`**: Creates a Redis-backed cache.
- **`NewCache(...)`**: A generic factory that creates a cache based on a type string (`"inmemory"` or `"redis"`).

### Example: Creating an In-Memory Cache

```go
import (
    "context"
    "time"
    "gofr.dev/pkg/cache/factory"
    "gofr.dev/pkg/cache/observability"
)

// 1. Create a metrics collector
metrics := observability.NewMetrics("gofr", "cache")

// 2. Create a logger
logger := observability.NewStdLogger()

// 3. Create the cache instance using the factory
c, err := factory.NewInMemoryCache(ctx, 
    "my-inmemory-cache", // A unique name for the cache
    5*time.Minute,       // Default time-to-live for items
    1000,                // Maximum number of items (for LRU eviction)
    factory.WithLogger(logger), 
    metrics,
)
```

### Example: Creating a Redis Cache

```go
c, err := factory.NewRedisCache(ctx, 
    "my-redis-cache",
    10*time.Minute,
    factory.WithLogger(logger),
    metrics,
    // You can also provide Redis-specific options
    // "localhost:6379", // Address
    // "password",       // Password
    // 0,                // DB number
)
```

---

## Configuration & Options

Both cache types can be configured using functional options.

### In-Memory Cache Options

- **`WithName(string)`**: A logical name for the cache, used in logs and metrics.
- **`WithTTL(time.Duration)`**: The default time-to-live for cache entries.
- **`WithMaxItems(int)`**: The maximum number of items before LRU eviction is triggered. `0` means no limit.
- **`WithLogger(observability.Logger)`**: A custom logger.
- **`WithMetrics(*observability.Metrics)`**: A metrics collector.

### Redis Cache Options

- **`WithName(string)`**: A logical name for the cache.
- **`WithTTL(time.Duration)`**: The default time-to-live for entries.
- **`WithAddr(string)`**: The Redis server address (e.g., `"localhost:6379"`).
- **`WithPassword(string)`**: The Redis server password.
- **`WithDB(int)`**: The Redis database number.
- **`WithLogger(observability.Logger)`**: A custom logger.
- **`WithMetrics(*observability.Metrics)`**: A metrics collector.

---

## Observability: Logging & Metrics

### Logging

The cache components produce structured, colored logs for better readability. You can provide your own logger implementation that satisfies the `observability.Logger` interface, or use the provided `NewStdLogger()` or `NewNopLogger()` (to disable logging).

### Metrics

The cache exposes a comprehensive set of Prometheus metrics:

- **`gofr_cache_hits_total`**: Total cache hits.
- **`gofr_cache_misses_total`**: Total cache misses.
- **`gofr_cache_sets_total`**: Total set operations.
- **`gofr_cache_deletes_total`**: Total delete operations.
- **`gofr_inmemory_cache_evictions_total`**: (In-memory only) Total evictions due to capacity limits.
- **`gofr_inmemory_cache_items_current`**: (In-memory only) Current number of items.
- **`gofr_cache_operation_latency_seconds`**: Latency histogram for cache operations.

All metrics are labeled with `cache_name` to distinguish between different cache instances.
