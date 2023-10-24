package store

import (
	"gofr.dev/examples/using-elasticsearch/model"
	"gofr.dev/pkg/gofr"
)

type Customer interface {
	Get(context *gofr.Context, name string) ([]model.Customer, error)
	GetByID(context *gofr.Context, id string) (model.Customer, error)
	Update(context *gofr.Context, customer model.Customer, id string) (model.Customer, error)
	Create(context *gofr.Context, customer model.Customer) (model.Customer, error)
	Delete(context *gofr.Context, id string) error
}
