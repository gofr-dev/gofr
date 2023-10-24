package main

import (
	"os"

	"gofr.dev/examples/fileResponse/handler"
	"gofr.dev/pkg/gofr"
)

func main() {
	// Create the application object
	app := gofr.New()
	rootPath, _ := os.Getwd()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	// overriding default template location.
	app.TemplateDir = rootPath + "/static"

	app.GET("/file", handler.FileHandler)

	app.Start()
}
