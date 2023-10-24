package main

import (
	"os"

	"gofr.dev/examples/template-examples/handler"
	"gofr.dev/pkg/gofr"
)

func main() {
	// Create the application object
	app := gofr.New()
	rootPath, _ := os.Getwd()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	// overriding default template location.
	app.TemplateDir = rootPath + "/templates"

	app.GET("/test", handler.Template)

	app.GET("/image", handler.Image)

	app.Start()
}
