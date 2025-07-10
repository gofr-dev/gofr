# Gofr Cache Example: In-Memory & Redis Cache with Observability

## Overview

This example demonstrates a **observable, and extensible cache layer** using the Gofr framework. It supports both **in-memory** and **Redis** backends, exposes rich **Prometheus metrics**, and provides **Grafana dashboards**. The setup is fully containerized for local development and observability.

---

## Features

- **Unified Cache Interface**: Swap between in-memory and Redis with a single config change.
- **Pluggable Observability**: Prometheus metrics for hits, misses, sets, deletes, evictions, and latency.
- **Flexible Configuration**: TTL, max items, custom logger, and metrics.
- **Production-Ready**: LRU eviction, background cleanup, error handling, and graceful shutdown.
- **Dockerized Monitoring**: Simple setup for Prometheus and Grafana with pre-provisioned dashboards.
- **Extensible**: Add new cache backends or observability hooks with minimal effort.

---

## Directory Structure

```
using-cache/
├── Dockerfile                # Grafana container
├── docker-compose.yml        # Orchestrates Prometheus & Grafana
├── grafana-dashboard.json    # Example dashboard
├── main.go                   # Example Go application using the cache
├── prometheus.yml            # Prometheus scrape config
├── provisioning/
│   ├── dashboards/
│   │   ├── cache-metrics.json   # Grafana dashboard
│   │   └── dashboard.yaml       # Grafana dashboard provisioning
│   └── datasources/
│       └── prometheus.yaml      # Grafana datasource provisioning
```

---

## Quick Start

### 1. Run Everything with Docker Compose

```sh
docker-compose up --build
```
- **Grafana**: [http://localhost:3000](http://localhost:3000) (user: `admin`, pass: `admin`)
- **Prometheus**: [http://localhost:9090](http://localhost:9090)
- **App Metrics**: [http://localhost:8080/metrics](http://localhost:8080/metrics)

> **Note:** The Go app must be running locally for Prometheus to scrape metrics. See [main.go](./main.go).

### 2. Run the Go Example

```sh
go run main.go
```

---

## Usage: Go Cache API

### Unified Cache Interface

All caches implement:

```go
Get(ctx, key) (value, found, error)
Set(ctx, key, value) error
Delete(ctx, key) error
Exists(ctx, key) (bool, error)
Clear(ctx) error
Close(ctx) error
```

### Example: In-Memory Cache

```go
import (
    "context"
    "time"
    "gofr.dev/pkg/cache/factory"
    "gofr.dev/pkg/cache/observability"
)

ctx := context.Background()

metrics := observability.NewMetrics("gofr", "cache")

c, err := factory.NewInMemoryCache(ctx, 
    "default", 
    5*time.Minute, 
    1000, 
    factory.WithLogger(observability.NewStdLogger()), 
    metrics
)

if err != nil { panic(err) }

c.Set(ctx, "alpha", 42)
v, found, _ := c.Get(ctx, "alpha")
c.Delete(ctx, "alpha")
```

### Example: Redis Cache

```go
c, err := factory.NewRedisCache(ctx, 
    "default", 
    5*time.Minute, 
    factory.WithLogger(observability.NewStdLogger()), 
    metrics
)
```

### Dynamic Cache Selection

```go
cacheType := "inmemory" // or "redis"
c, err := factory.NewCache(ctx, 
    cacheType, 
    "default", 
    5*time.Minute, 
    1000, 
    factory.WithLogger(observability.NewStdLogger()), 
    metrics
)
```

---

## Configuration Options

### In-Memory Cache
- **name**: Logical name for metrics/logging
- **ttl**: Default time-to-live for entries
- **maxItems**: Max items before LRU eviction (0 = unlimited)
- **logger**: Custom logger (optional)
- **metrics**: Prometheus metrics (optional)

### Redis Cache
- **name**: Logical name for metrics/logging
- **ttl**: Default time-to-live for entries
- **address**: Redis server address (default: `localhost:6379`)
- **password**: Redis password (optional)
- **db**: Redis DB number (optional)
- **logger**: Custom logger (optional)
- **metrics**: Prometheus metrics (optional)

---

## Observability & Metrics

### Exposed Metrics

- `gofr_cache_hits_total` / `gofr_inmemory_cache_hits_total`
- `gofr_cache_misses_total` / `gofr_inmemory_cache_misses_total`
- `gofr_cache_sets_total` / `gofr_inmemory_cache_sets_total`
- `gofr_cache_deletes_total` / `gofr_inmemory_cache_deletes_total`
- `gofr_inmemory_cache_evictions_total`
- `gofr_inmemory_cache_items_current`
- `gofr_cache_operation_latency_seconds`

All metrics are labeled by `cache_name`.

### Prometheus Setup

`prometheus.yml`:
```yaml
global:
  scrape_interval: 5s
scrape_configs:
  - job_name: 'gofr-cache'
    static_configs:
      - targets: ['host.docker.internal:8080']
```

### Grafana Provisioning

- **Datasource**: `provisioning/datasources/prometheus.yaml`
- **Dashboards**: `provisioning/dashboards/cache-metrics.json`, `dashboard.yaml`

---

## Docker & Monitoring Stack

### docker-compose.yml

- **Prometheus**: Scrapes Go app metrics
- **Grafana**: Pre-provisioned with Prometheus datasource and cache dashboard

### Dockerfile (Grafana)

```Dockerfile
FROM grafana/grafana:latest
EXPOSE 3000
# COPY provisioning /etc/grafana/provisioning
CMD ["/run.sh"]
```

---

## Advanced: Extending & Troubleshooting

### Adding a New Cache Backend
- Implement the `cache.Cache` interface
- Add a factory method in `pkg/cache/factory`
- Register metrics/logging as needed

### Customizing Metrics
- Use `observability.NewMetrics(namespace, subsystem)`
- Add custom labels or buckets if needed

### Logging
- Use `observability.NewStdLogger()` or implement your own logger

### Common Issues
- **Prometheus can't scrape metrics**: Ensure Go app is running and accessible at `localhost:8080`
- **Grafana dashboard empty**: Check Prometheus datasource config and scrape targets
- **Redis connection errors**: Check address, password, and network

---

## References
- [main.go](./main.go): Example usage
- [pkg/cache/factory](../../pkg/cache/factory): Factory methods
- [pkg/cache/inmemory](../../pkg/cache/inmemory): In-memory implementation
- [pkg/cache/redis](../../pkg/cache/redis): Redis implementation
- [pkg/cache/observability](../../pkg/cache/observability): Metrics & logging

