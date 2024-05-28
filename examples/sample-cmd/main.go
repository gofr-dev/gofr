package main

import (
	"fmt"

	"gofr.dev/pkg/gofr"
)

func main() {
	// Create a new command-line application
	app := gofr.NewCMD()

	// Add a sub-command "hello" with its handler and description
	app.SubCommand("hello", func(c *gofr.Context) (interface{}, error) {
		return "Hello World!", nil
	}, "Prints 'Hello World!'")

	// Add a sub-command "params" with its handler and description
	app.SubCommand("params", func(c *gofr.Context) (interface{}, error) {
		return fmt.Sprintf("Hello %s!", c.Param("name")), nil
	}, "Prints 'Hello <name>!'")

	// Run the command-line application
	app.Run()
}
