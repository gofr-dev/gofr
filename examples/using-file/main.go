package main

import (
	"gofr.dev/examples/using-file/handler"
	"gofr.dev/pkg/file"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.NewCMD()

	fileAbstracter, err := file.NewWithConfig(app.Config, "test.txt", "rw")
	if err != nil {
		app.Logger.Error("Unable to initialize", err)
		return
	}

	h := handler.New(fileAbstracter)

	app.GET("read", h.Read)
	app.GET("write", h.Write)
	app.GET("list", h.List)
	app.GET("move", h.Move)

	app.Start()
}
