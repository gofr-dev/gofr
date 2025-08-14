// DISCLAIMER:
// This is a simple simulation of using a cache with some seed metrics
// to demonstrate how the cache factory, metrics, and observability hooks work.
// It continuously sets, gets, and deletes cache keys in a loop to generate
// measurable metrics for Prometheus.
// NOTE:
// - This is not intended for production use as-is.
// - Actual implementations may differ significantly depending on requirements,
//   storage backends, error handling, concurrency, and performance optimizations.

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gofr.dev/pkg/cache/factory"
	"gofr.dev/pkg/cache/observability"
	"gofr.dev/pkg/gofr"
)

func main() {
	// Cancellable context that ends on SIGINT/SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\nReceived shutdown signal, exiting...")
		cancel()
	}()

	metrics := observability.NewMetrics("gofr", "cache")

	c, err := factory.NewInMemoryCache(ctx,
		"default",
		5*time.Minute,
		1000,
		factory.WithObservabilityLogger(observability.NewStdLogger()),
		factory.WithMetrics(metrics))
	if err != nil {
		panic(fmt.Sprintf("Failed to create cache: %v", err))
	}

	// Alternative: Redis cache
	// c, err := factory.NewRedisCache(ctx, "default", 5*time.Minute,
	// 	factory.WithObservabilityLogger(observability.NewStdLogger()),
	// 	factory.WithMetrics(metrics),
	//  factory.WithRedisAddr("localhost:6379"))

	// Alternative: Dynamic cache type
	// cacheType := "inmemory" // or "redis"
	// c, err := factory.NewCache(ctx, cacheType, "default", 5*time.Minute, 1000,
	// 	factory.WithObservabilityLogger(observability.NewStdLogger()),
	// 	factory.WithMetrics(metrics))

	// Start GoFr app to serve metrics via its built-in metrics server.
	app := gofr.New()

	go func() {
		port := app.Config.Get("METRICS_PORT")
		if port == "" {
			port = "2121"
		}
		fmt.Printf("Metrics available at http://localhost:%s/metrics\n", port)
		app.Run()
		cancel()
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				c.Set(ctx, "alpha", 42)   // triggers sets_total
				c.Get(ctx, "alpha")       // triggers hits_total
				c.Get(ctx, "nonexistent") // triggers misses_total
				c.Delete(ctx, "alpha")    // triggers deletes_total
				c.Set(ctx, "alpha", 100)
				time.Sleep(2 * time.Second)
			}
		}
	}()

	// Wait until the context is canceled.
	<-ctx.Done()
	fmt.Println("Shutdown complete.")
}
