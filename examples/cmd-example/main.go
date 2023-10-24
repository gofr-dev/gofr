package main

import (
	"errors"
	"fmt"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/template"
)

func main() {
	app := gofr.NewCMD()

	app.GET("hello", func(ctx *gofr.Context) (interface{}, error) {
		name := ctx.PathParam("name")

		if name == "" {
			return "Hello!", nil
		}

		return fmt.Sprintf("Hello %s!", name), nil
	})

	app.GET("error", func(ctx *gofr.Context) (interface{}, error) {
		return nil, errors.New("some error occurred")
	})

	app.GET("bind", func(ctx *gofr.Context) (interface{}, error) {
		var a struct {
			Name   string
			IsGood bool
		}

		_ = ctx.Bind(&a)

		return fmt.Sprintf("Name: %s Good: %v", a.Name, a.IsGood), nil
	})

	app.GET("temp", func(ctx *gofr.Context) (interface{}, error) {
		filename := ctx.PathParam("filename")
		return template.Template{
			Directory: "templates",
			File:      filename,
			Data:      nil,
			Type:      template.FILE,
		}, nil
	})

	app.GET("file", func(ctx *gofr.Context) (interface{}, error) {
		return template.File{
			Content:     []byte("Hello"),
			ContentType: "text/plain",
		}, nil
	})

	app.Start()
}
