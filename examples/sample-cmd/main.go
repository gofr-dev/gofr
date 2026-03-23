package main

import (
	"fmt"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/cmd/terminal"
)

func main() {
	// Create a new command-line application
	app := gofr.NewCMD()

	// Add a sub-command "hello" with its handler, help and description
	app.SubCommand("hello", func(c *gofr.Context) (any, error) {
		return "Hello World!", nil
	},
		gofr.AddDescription("Print 'Hello World!'"),
		gofr.AddHelp("hello world option"),
	)

	// Add a sub-command "params" with its handler, help and description
	app.SubCommand("params", func(c *gofr.Context) (any, error) {
		return fmt.Sprintf("Hello %s!", c.Param("name")), nil
	})

	app.SubCommand("spinner", spinner)

	app.SubCommand("progress", progress)

	// A command that always fails — useful for verifying failure metrics
	app.SubCommand("fail", func(_ *gofr.Context) (any, error) {
		return nil, fmt.Errorf("simulated failure for metrics testing")
	}, gofr.AddDescription("Always fails (for testing failure metrics)"))

	// A command that simulates a slow batch job — verifies extended histogram buckets
	app.SubCommand("batch", func(c *gofr.Context) (any, error) {
		duration := 3 * time.Second

		if d := c.Param("duration"); d != "" {
			parsed, err := time.ParseDuration(d)
			if err == nil {
				duration = parsed
			}
		}

		time.Sleep(duration)

		return fmt.Sprintf("Batch completed in %s", duration), nil
	}, gofr.AddDescription("Simulates a batch job with configurable duration"))

	// Run the command-line application
	app.Run()
}

func spinner(ctx *gofr.Context) (any, error) {
	// initialize the spinner
	sp := terminal.NewDotSpinner(ctx.Out)
	sp.Spin(ctx)

	defer sp.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(2 * time.Second):
	}

	return "Process Complete", nil
}

func progress(ctx *gofr.Context) (any, error) {
	p, err := terminal.NewProgressBar(ctx.Out, 100)
	if err != nil {
		ctx.Warn("error initializing progress bar, err : %v", err)
	}

	for i := 1; i <= 100; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			// do a time taking process or compute a small subset of a bigger problem,
			// this could be processing batches of a data set.

			// increment the progress to display on the progress bar.
			p.Incr(int64(1))
		}
	}

	return "Process Complete", nil
}
