# Gofr In-Memory Cache Example

A blazing-fast, thread-safe, in-memory cache for Go with built-in observability, LRU eviction, TTL expiration, and Prometheus/Grafana integration.

---

## Features

- **Thread-safe**: Safe for concurrent use by multiple goroutines
- **LRU Eviction**: Least Recently Used items are evicted when capacity is reached
- **TTL Expiry**: Items expire automatically after a configurable time-to-live
- **Prometheus Metrics**: Out-of-the-box metrics for hits, misses, sets, deletes, evictions, and latency
- **Grafana Dashboard**: Prebuilt dashboard for instant cache observability
- **Flexible API**: Set, Get, Delete, Exists, Clear, and Close operations
- **Customizable**: Configure name, TTL, max items, logger, and metrics

---

## Quick Start

### 1. Clone & Run Locally

```sh
cd gofr/examples/using-inmemory-cache
go run main.go
```

- The app exposes Prometheus metrics at [http://localhost:8080/metrics](http://localhost:8080/metrics)

### 2. Run with Docker Compose (Prometheus + Grafana)

```sh
docker-compose up -d
```

- Prometheus: [http://localhost:9090](http://localhost:9090)
- Grafana: [http://localhost:3001](http://localhost:3001) (login: `admin`/`admin`)
- App metrics: [http://localhost:8080/metrics](http://localhost:8080/metrics)

---

## Example Usage

```go
package main

import (
    "context"
    "time"
    "gofr.dev/pkg/cache/inmemory"
    "gofr.dev/pkg/cache/observability"
)

func main() {
    ctx := context.Background()
    metrics := observability.NewMetrics("gofr", "inmemory_cache")
    cache, err := inmemory.NewInMemoryCache(
        inmemory.WithName("demo"),
        inmemory.WithTTL(10*time.Second),
        inmemory.WithMaxItems(100),
        inmemory.WithMetrics(metrics),
    )
    if err != nil {
        panic(err)
    }
    // Set a value
    cache.Set(ctx, "alpha", 42)
    // Get a value
    val, found, _ := cache.Get(ctx, "alpha")
    // Delete a value
    cache.Delete(ctx, "alpha")
    // Check existence
    exists, _ := cache.Exists(ctx, "alpha")
    // Clear all
    cache.Clear(ctx)
    // Close when done
    cache.Close(ctx)
}
```

---

## Configuration Options

- `WithName(name string)`: Set a friendly cache name (for metrics)
- `WithTTL(ttl time.Duration)`: Set default time-to-live for entries (0 disables expiry)
- `WithMaxItems(max int)`: Set max number of items (0 = unlimited)
- `WithLogger(logger)`: Custom logger (optional)
- `WithMetrics(metrics)`: Custom Prometheus metrics (optional)

Or use convenience constructors:

```go
cache, _ := inmemory.NewDefaultCache()      // 5m TTL, 1000 items
cache, _ := inmemory.NewDebugCache("dev")  // 1m TTL, 100 items
cache, _ := inmemory.NewProductionCache("prod", 10*time.Minute, 5000)
```

---

## Observability & Metrics

The cache exposes rich Prometheus metrics:

| Metric Name                        | Description                        |
|------------------------------------|------------------------------------|
| gofr_inmemory_cache_hits_total     | Total cache hits                   |
| gofr_inmemory_cache_misses_total   | Total cache misses                 |
| gofr_inmemory_cache_sets_total     | Total set operations               |
| gofr_inmemory_cache_deletes_total  | Total delete operations            |
| gofr_inmemory_cache_evictions_total| Total evictions (LRU)              |
| gofr_inmemory_cache_items_current  | Current number of items            |
| gofr_inmemory_cache_operation_latency_seconds | Latency of cache operations |

### Prometheus Scrape Config

```
scrape_configs:
  - job_name: 'gofr-cache'
    static_configs:
      - targets: ['host.docker.internal:8080']
```

---

## Grafana Dashboard

- Prebuilt dashboard: `provisioning/dashboards/cache-metrics.json`
- Auto-provisioned via `provisioning/dashboards/dashboard.yaml`
- Visualizes hits, misses, sets, deletes, evictions, and item count

**Grafana URL:** [http://localhost:3001](http://localhost:3001)

---

## Advanced

- **Thread Safety:** All operations are safe for concurrent goroutines
- **LRU Policy:** Least recently used items are evicted first when at capacity
- **TTL Expiry:** Expired items are cleaned up periodically (interval = TTL/4, min 10s)
- **Error Handling:**
  - `ErrCacheClosed`: Cache is closed
  - `ErrEmptyKey`: Key is empty
  - `ErrNilValue`: Value is nil
  - `ErrInvalidMaxItems`: Invalid max items
- **Testing:** See [`inmemory_test.go`](../../pkg/cache/inmemory/inmemory_test.go) for edge cases and concurrency tests

---

## File Structure

```
examples/using-inmemory-cache/
├── main.go                # Example app
├── docker-compose.yml     # Prometheus + Grafana setup
├── prometheus.yml         # Prometheus config
├── provisioning/
│   ├── datasources/
│   │   └── prometheus.yaml
│   └── dashboards/
│       ├── cache-metrics.json
│       └── dashboard.yaml
```
