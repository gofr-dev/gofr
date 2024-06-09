package main

import (
	"fmt"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.NewCMD()

	app.SubCommand("hello", func(c *gofr.Context) (interface{}, error) {
		return "Hello World!\n", nil
	})

	app.SubCommand("params", func(c *gofr.Context) (interface{}, error) {
		return fmt.Sprintf("Hello %s!\n", c.Param("name")), nil
	})

	app.Run()

}
