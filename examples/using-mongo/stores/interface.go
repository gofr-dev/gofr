package stores

import (
	"gofr.dev/examples/using-mongo/models"
	"gofr.dev/pkg/gofr"
)

type Customer interface {
	Get(ctx *gofr.Context, name string) ([]models.Customer, error)
	Create(ctx *gofr.Context, model models.Customer) error
	Delete(ctx *gofr.Context, name string) (int, error)
}
