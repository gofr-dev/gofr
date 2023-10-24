package services

import (
	"context"

	"gofr.dev/examples/using-http-service/models"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/service"
)

type HTTPService interface {
	Get(ctx context.Context, api string, params map[string]interface{}) (*service.Response, error)
	Bind(resp []byte, i interface{}) error
}

type User interface {
	Get(ctx *gofr.Context, name string) (models.User, error)
}
