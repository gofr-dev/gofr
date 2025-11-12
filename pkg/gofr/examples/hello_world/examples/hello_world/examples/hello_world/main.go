package main

import (
    "github.com/gofr-dev/gofr/pkg/gofr"
)

func main() {
    app := gofr.New()

    // basic route
    app.GET("/hello", func(ctx *gofr.Context) (interface{}, error) {
        return map[string]string{"message": "Hello, GoFr contributor!"}, nil
    })

    app.Start()
}
