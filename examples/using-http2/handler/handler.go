package handler

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/template"
)

type Person struct {
	Username string
	Password string
}

// HomeHandler renders home.html file
// Uses server push to give out the app.css file required by the home.html file
func HomeHandler(ctx *gofr.Context) (interface{}, error) {
	if ctx.ServerPush != nil {
		ctx.Logger.Info("gofr.png required by home.html pushed using server push")

		err := ctx.ServerPush.Push("/static/gofr.png", nil)
		if err != nil {
			return nil, err
		}
	}

	return template.Template{File: "home.html", Type: template.HTML}, nil
}

// ServeStatic Renders the file present in /static folder
func ServeStatic(ctx *gofr.Context) (interface{}, error) {
	fileName := ctx.PathParam("name")

	return template.Template{File: fileName}, nil
}
