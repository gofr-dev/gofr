package main

import "github.com/vikash/gofr/pkg/gofr"

func main() {
	app := gofr.NewCMD()

	app.SubCommand("hello", func(c *gofr.Context) (interface{}, error) {
		return "Hello World!", nil
	})

	app.Run()

}
