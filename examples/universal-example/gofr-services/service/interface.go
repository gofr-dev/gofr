package service

import (
	"context"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/service"
)

type Service interface {
	Log(ctx *gofr.Context, serviceName string) (string, error)
	Hello(ctx *gofr.Context, name string) (string, error)
}

type HTTPService interface {
	Get(ctx context.Context, api string, params map[string]interface{}) (*service.Response, error)
	Bind(resp []byte, i interface{}) error
}
