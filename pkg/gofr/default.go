package gofr

import (
	"embed"

	"github.com/vikash/gofr/pkg/gofr/http/response"
)

const (
	defaultHTTPPort  = 8000
	defaultDBPort    = 3306
	defaultRedisPort = 6379
)

func healthHandler(c *Context) (interface{}, error) {
	return "OK", nil
}

//go:embed static/*
var static embed.FS

func faviconHandler(c *Context) (interface{}, error) {
	data, err := static.ReadFile("static/favicon.ico")
	return response.File{
		Content:     data,
		ContentType: "image/x-icon",
	}, err
}
