package main

import (
	"fmt"

	"gofr.dev/pkg/gofr"
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

	// Run the command-line application
	app.Run()
}
