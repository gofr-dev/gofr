package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gofr.dev/pkg/cache/observability"
	"gofr.dev/pkg/cache/redis"
)

func main() {
	ctx := context.Background()

	// Step 1: Setup Prometheus metrics
	metrics := observability.NewMetrics("gofr", "redis_cache")

	// Step 2: Create a monitored Redis cache
	cache, err := redis.NewRedisCache(ctx,
		redis.WithName("demo-redis"),
		redis.WithTTL(10*time.Second),
		redis.WithMetrics(metrics),
	)
	if err != nil {
		panic(err)
	}

	// Step 3: Expose /metrics endpoint
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		fmt.Println("Metrics available at http://localhost:8080/metrics")
		http.ListenAndServe(":8080", nil)
	}()

	// Step 4: Force metric activity
	go func() {
		for {
			cache.Set(ctx, "alpha", 42)   // triggers `sets_total`
			cache.Get(ctx, "alpha")       // triggers `hits_total`
			cache.Get(ctx, "nonexistent") // triggers `misses_total`
			cache.Delete(ctx, "alpha")    // triggers `deletes_total`
			cache.Set(ctx, "alpha", 100)
			time.Sleep(2 * time.Second)
		}
	}()

	// Keep alive
	select {}
}
