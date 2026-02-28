package main

import (
	"fmt"
	"time"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Stream the current time every second.
	app.SSE("/events", func(ctx *gofr.Context, stream *gofr.SSEStream) error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		i := 0

		for {
			select {
			case <-ctx.Done():
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
	})

	// A countdown that sends 11 events and closes.
	app.SSE("/countdown", func(ctx *gofr.Context, stream *gofr.SSEStream) error {
		for i := 10; i >= 0; i-- {
			select {
			case <-ctx.Done():
				return nil
			default:
				if err := stream.SendEvent("countdown", map[string]int{"remaining": i}); err != nil {
					return err
				}

				time.Sleep(500 * time.Millisecond)
			}
		}

		return stream.SendEvent("done", "Countdown complete!")
	})

	app.Run()
}
