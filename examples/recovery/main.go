package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"gofr.dev/pkg/gofr"
)

// Handler manages panic counters for the recovery example.
type Handler struct {
	panicCount   atomic.Int64
	recoverCount atomic.Int64
}

func main() {
	app := gofr.New()
	handler := &Handler{}

	// HTTP handler that panics - automatically recovered
	app.GET("/panic", func(c *gofr.Context) (any, error) {
		handler.panicCount.Add(1)
		panic(fmt.Sprintf("intentional panic #%d", handler.panicCount.Load()))
		//nolint:unreachable // panic is intentional for recovery demonstration
		return nil, nil
	})

	// HTTP handler that uses GoSafe for goroutines
	app.GET("/async-panic", func(c *gofr.Context) (any, error) {
		c.GoSafe(func() {
			handler.recoverCount.Add(1)
			panic(fmt.Sprintf("goroutine panic #%d", handler.recoverCount.Load()))
		})
		return map[string]string{"status": "processing"}, nil
	})

	// HTTP handler that works normally
	app.GET("/status", func(c *gofr.Context) (any, error) {
		return map[string]any{
			"panic_count":   handler.panicCount.Load(),
			"recover_count": handler.recoverCount.Load(),
			"timestamp":     time.Now().Unix(),
		}, nil
	})

	// Cron job that panics - automatically recovered
	app.AddCronJob("*/5 * * * * *", "panic-job", func(c *gofr.Context) {
		c.Infof("Cron job running at %s", time.Now().Format(time.RFC3339))
		// Uncomment to test cron panic recovery:
		// panic("cron job panic")
	})

	// Cron job that logs recovery status
	app.AddCronJob("*/10 * * * * *", "status-job", func(c *gofr.Context) {
		c.Infof("Status: %d panics, %d recovered goroutines", handler.panicCount.Load(), handler.recoverCount.Load())
	})

	app.Run()
}
