package main

import (
	"fmt"
	"time"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Stream the current time every second.
	// c.Context.Done() fires on both client disconnect and server shutdown.
	app.GET("/events", func(c *gofr.Context) (any, error) {
		return gofr.SSEResponse(func(stream *gofr.SSEStream) error {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			i := 0

			for {
				select {
				case <-c.Context.Done():
					// Graceful cleanup: release resources, close DB cursors, etc.
					return nil
				case t := <-ticker.C:
					if err := stream.Send(gofr.SSEEvent{
						ID:   fmt.Sprintf("%d", i),
						Name: "time",
						Data: map[string]string{"time": t.Format(time.RFC3339)},
					}); err != nil {
						return err
					}

					i++
				}
			}
		}), nil
	})

	// A countdown that sends 11 events and closes.
	app.GET("/countdown", func(c *gofr.Context) (any, error) {
		return gofr.SSEResponse(func(stream *gofr.SSEStream) error {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for i := 10; i >= 0; i-- {
				select {
				case <-c.Context.Done():
					return nil
				case <-ticker.C:
					if err := stream.SendEvent("countdown", map[string]int{"remaining": i}); err != nil {
						return err
					}
				}
			}

			return stream.SendEvent("done", "Countdown complete!")
		}), nil
	})

	app.Run()
}
