package gofr

import (
	"github.com/vikash/gofr/pkg/gofr/http/response"
	"github.com/vikash/gofr/pkg/gofr/static"
)

const (
	defaultHTTPPort  = 8000
	defaultDBPort    = 3306
	defaultRedisPort = 6379
)

func healthHandler(c *Context) (interface{}, error) {
	return "OK", nil
}

func faviconHandler(c *Context) (interface{}, error) {
	data, err := static.Files.ReadFile("favicon.ico")

	return response.File{
		Content:     data,
		ContentType: "image/x-icon",
	}, err
}
