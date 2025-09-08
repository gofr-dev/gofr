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

	// Initialize the GoFr app.
	app := gofr.New()

	// Method 1: Using the app's convenience methods (recommended for most cases)
	// Tracing is automatically handled by the factory - no manual setup required!
	app.AddInMemoryCache(ctx, "default", 5*time.Minute, 1000)
	// app.AddRedisCache(ctx, "default", 5*time.Minute, "localhost:6379")

	// Method 2: Using the factory directly (for more control)
	// c, err := factory.NewInMemoryCache(
	//     ctx,
	//     "default",
	//     factory.WithLogger(app.Logger()),
	//     factory.WithTTL(5*time.Minute),
	//     factory.WithMaxItems(1000),
	// )
	// if err != nil {
	//     panic(fmt.Sprintf("failed to create cache: %v", err))
	// }
	// app.container.AddCache("default", c)

	// Get the cache instance into a variable 'c' in the main scope.
	c := app.GetCache("default")
	if c == nil {
		panic("failed to get cache from app container")
	}

	// Goroutine to run the app's metrics server.
	go func() {
		port := app.Config.Get("METRICS_PORT")
		if port == "" {
			port = "2121"
		}
		fmt.Printf("Metrics available at http://localhost:%s/metrics\n", port)
		app.Run()
		cancel()
	}()

	// Goroutine to simulate cache usage.
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
