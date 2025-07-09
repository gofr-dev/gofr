# Gofr Redis Cache Example

A fast, observable Redis-backed cache for Go with Prometheus/Grafana integration.

---

## Features

- **Redis Backend**: Uses Redis for distributed, persistent caching
- **TTL Expiry**: Items expire automatically after a configurable time-to-live
- **Prometheus Metrics**: Out-of-the-box metrics for hits, misses, sets, deletes, and latency
- **Grafana Dashboard**: Prebuilt dashboard for instant cache observability
- **Flexible API**: Set, Get, Delete, Exists, Clear, and Close operations
- **Customizable**: Configure name, TTL, logger, and metrics

---

## Quick Start

### 1. Clone & Run Locally (Requires Redis running on localhost:6379)

```sh
cd gofr/examples/using-redis-cache
go run main.go
```

- The app exposes Prometheus metrics at [http://localhost:8080/metrics](http://localhost:8080/metrics)

### 2. Run with Docker Compose (Prometheus + Grafana + Redis)

```sh
docker-compose up -d
```

- Prometheus: [http://localhost:9090](http://localhost:9090)
- Grafana: [http://localhost:3001](http://localhost:3001) (login: `admin`/`admin`)
- App metrics: [http://localhost:8080/metrics](http://localhost:8080/metrics)
- Redis: [localhost:6379](localhost:6379)

---

## Example Usage

```go
package main

import (
    "context"
    "time"
    "gofr.dev/pkg/cache/redis"
    "gofr.dev/pkg/cache/observability"
)

func main() {
    ctx := context.Background()
    metrics := observability.NewMetrics("gofr", "redis_cache")
    cache, err := redis.NewRedisCache(ctx,
        redis.WithName("demo-redis"),
        redis.WithTTL(10*time.Second),
        redis.WithMetrics(metrics),
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
- `WithTTL(ttl time.Duration)`: Set default time-to-live for entries
- `WithLogger(logger)`: Custom logger (optional)
- `WithMetrics(metrics)`: Custom Prometheus metrics (optional)

---

## Observability & Metrics

The cache exposes rich Prometheus metrics:

| Metric Name                        | Description                        |
|------------------------------------|------------------------------------|
| gofr_redis_cache_hits_total        | Total cache hits                   |
| gofr_redis_cache_misses_total      | Total cache misses                 |
| gofr_redis_cache_sets_total        | Total set operations               |
| gofr_redis_cache_deletes_total     | Total delete operations            |
| gofr_redis_cache_items_current     | Current number of items            |
| gofr_redis_cache_operation_latency_seconds | Latency of cache operations |

### Prometheus Scrape Config

```
scrape_configs:
  - job_name: 'gofr-redis-cache'
    static_configs:
      - targets: ['host.docker.internal:8080']
```

---

## Grafana Dashboard

- Prebuilt dashboard: `provisioning/dashboards/cache-metrics.json`
- Auto-provisioned via `provisioning/dashboards/dashboard.yaml`
- Visualizes hits, misses, sets, deletes, and item count

**Grafana URL:** [http://localhost:3001](http://localhost:3001)

---

## File Structure

```
examples/using-redis-cache/
├── main.go                # Example app
├── docker-compose.yml     # Prometheus + Grafana + Redis setup
├── prometheus.yml         # Prometheus config
├── provisioning/
│   ├── datasources/
│   │   └── prometheus.yaml
│   └── dashboards/
│       ├── cache-metrics.json
│       └── dashboard.yaml
``` 