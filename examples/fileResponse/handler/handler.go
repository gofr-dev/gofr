package handler

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/template"
)

func FileHandler(ctx *gofr.Context) (interface{}, error) {
	return template.Template{Directory: ctx.TemplateDir, File: "gofr.png", Data: nil, Type: template.FILE}, nil
}
