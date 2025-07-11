# Cache Module

A high-performance, feature-rich caching module for Go applications with support for multiple backends and comprehensive observability features.

[![Go Reference](https://pkg.go.dev/badge/gofr.dev/pkg/cache.svg)](https://pkg.go.dev/gofr.dev/pkg/cache)
[![Go Report Card](https://goreportcard.com/badge/gofr.dev/pkg/cache)](https://goreportcard.com/report/gofr.dev/pkg/cache)

## Features

- 🚀 Multiple cache backends:
  - In-memory cache with TTL support
  - Redis integration
- 📊 Built-in observability:
  - Prometheus metrics (hits, misses, errors)
  - OpenTelemetry tracing
  - Structured logging
- 🔄 Context support for cancellation and timeouts
- 🔒 Thread-safe operations
- 🎯 Query wrapping with automatic caching
- 🔌 Pluggable architecture with middleware support

## Installation
```
bash go get gofr.dev/pkg/cache
``` 

## Quick Start

### Using Redis Cache
```
go // Configure Redis rdbConfig := config.RdbConfig{ Host: "localhost", Port: "6379", Password: "", }
// Connect to Redis rdb, err := rdbConfig.RdbConnect(context.Background()) if err != nil { log.Fatal(err) }
// Create Redis cache cache := redis.NewRedisConfig(rdb)
``` 

### Using In-Memory Cache
```
go // Create in-memory cache cache := memory.NewInMemoryCache()
``` 

### Basic Operations
```
go ctx := context.Background()
// Set a value with TTL err := cache.Set(ctx, "key", "value", 5*time.Minute)
// Get a value value, err := cache.Get(ctx, "key")
// Delete a value err := cache.Delete(ctx, "key")
``` 

### Query Wrapping
```
go result, err := cache.WrapQuery(ctx, "user:123", 5*time.Minute, func(ctx context.Context) (string, error) { // Expensive operation (e.g., database query) return fetchUserData(ctx) })
``` 

### FLOW CHARTS
```aiignore

+---------+     GET/SET     +------------------+
| Handler |  -------------> |   Cache Layer    |
+---------+                 | (Redis/InMemory) |
                            +--------+---------+
                                     |
                              MISS   |
                                     v
                            +---------------+
                            | Fallback/Data |
                            |  (e.g., DB)   |
                            +---------------+


+----------+      WrapQuery       +-------------------+
|  Store   |  ------------------> | cache.WrapQuery() |
+----------+                     +---------+---------+
                                           |
                                  +--------v--------+
                                  | Try Get(key)    |
                                  |    ↓            |
                                  |  Miss → fn()    |
                                  |     → Set       |
                                  +------------------+
```

## Observability

### Metrics

The module exposes the following Prometheus metrics:
- `cache_hits_total{backend="..."}`
- `cache_misses_total{backend="..."}`
- `cache_errors_total{backend="..."}`

### Tracing

OpenTelemetry spans are created for all cache operations:
- `cache.Get`
- `cache.Set`
- `cache.Delete`
- `cache.WrapQuery`

### Logging

Enable logging with the logging wrapper:
```
go cache = cache.WithLogging(cache, logger)
``` 

## Middleware Stack

Build your cache stack with middleware:
```
go cache := memory.NewInMemoryCache() cache = cache.WithLogging(cache, logger) // Add logging cache = cache.NewTracer(cache) // Add tracing cache = cache.WithContextSupport(cache) // Add context support
``` 

## Interface

The cache interface provides a simple and consistent API:
```
go type Cache interface { Get(ctx context.Context, key string) (string, error) Set(ctx context.Context, key string, value string, ttl time.Duration) error Delete(ctx context.Context, key string) error WrapQuery(ctx context.Context, key string, ttl time.Duration, queryFn func(ctx context.Context) (string, error)) (string, error) Close() error }
``` 

## Thread Safety

All implementations (in-memory and Redis) are thread-safe and can be safely used in concurrent applications.
