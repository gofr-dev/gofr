package main

import "gofr.dev/pkg/gofr"

func main() {
	app := gofr.New()

	app.GraphQLQuery("hello", func(c *gofr.Context) (interface{}, error) {
		return "world", nil
	})

	app.Run()
}
