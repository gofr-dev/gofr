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
	app.SubCommand("hello", func(c *gofr.Context) (interface{}, error) {
		return "Hello World!", nil
	},
		gofr.AddDescription("Print 'Hello World!'"),
		gofr.AddHelp("hello world option"),
	)

	// Add a sub-command "params" with its handler, help and description
	app.SubCommand("params", func(c *gofr.Context) (interface{}, error) {
		return fmt.Sprintf("Hello %s!", c.Param("name")), nil
	})

	app.SubCommand("spinner", func(ctx *gofr.Context) (interface{}, error) {
		// initialize the spinner and defer stop it
		defer terminal.NewDotSpinner(ctx.Out).Spin().Stop()

		// stimulate a time-taking process
		time.Sleep(2 * time.Second)

		return "Process Complete", nil
	})

	app.SubCommand("progress", func(ctx *gofr.Context) (interface{}, error) {
		p := terminal.NewProgressBar(ctx.Out, 100)

		for i := 1; i <= 100; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// do a time taking process or compute a small subset of a bigger problem,
				// this could be processing batches of a data set.
				time.Sleep(50 * time.Millisecond)

				// increment the progress to display on the progress bar.
				p.Incr(int64(1))
			}
		}

		return "Process Complete", nil
	})

	// Run the command-line application
	app.Run()
}
