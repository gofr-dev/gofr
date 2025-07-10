package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gofr.dev/pkg/cache/factory"
	"gofr.dev/pkg/cache/observability"
)

func main() {
	ctx := context.Background()

	metrics := observability.NewMetrics("gofr", "cache")

	// Simple, explicit configuration - exactly as you wanted
	// c, err := factory.NewInMemoryCache(ctx, 
	// 	"default", 
	// 	5*time.Minute, 
	// 	1000, 
	// 	factory.WithLogger(observability.NewStdLogger()), 
	// 	metrics)
	// if err != nil {
	// 	panic(fmt.Sprintf("Failed to create cache: %v", err))
	// }

	// Alternative: Redis cache with same pattern
	c, err := factory.NewRedisCache(ctx, "default", 5*time.Minute, 
		factory.WithLogger(observability.NewStdLogger()), 
		metrics)

	if err != nil {
		panic(err)
	}

	// Alternative: Dynamic cache type
	// cacheType := "inmemory" // or "redis"
	// c, err := factory.NewCache(ctx, cacheType, "default", 5*time.Minute, 1000,
	// 	factory.WithLogger(observability.NewStdLogger()), 
	// 	metrics)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		fmt.Println("Metrics available at http://localhost:8080/metrics")
		http.ListenAndServe(":8080", nil)
	}()

	go func() {
		for {
			c.Set(ctx, "alpha", 42)        // triggers sets_total
			c.Get(ctx, "alpha")            // triggers hits_total
			c.Get(ctx, "nonexistent")      // triggers misses_total
			c.Delete(ctx, "alpha")         // triggers deletes_total
			c.Set(ctx, "alpha", 100)
			time.Sleep(2 * time.Second)
		}
	}()

	select {}
}