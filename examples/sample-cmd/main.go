package main

import (
	"fmt"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.NewCMD()

	app.SubCommand("hello", func(_ *gofr.Context) (interface{}, error) {
		return "Hello World!", nil
	}, gofr.AddHelp("hello world option"), gofr.AddFullPattern("hello-world"))

	app.SubCommand("params", func(c *gofr.Context) (interface{}, error) {
		return fmt.Sprintf("Hello %s!", c.Param("name")), nil
	})

	app.Run()
}
