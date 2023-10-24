package store

import (
	"gofr.dev/pkg/gofr"
)

type Customer interface {
	List(ctx *gofr.Context, collection string, filter Filter) ([]Model, error)
	Create(ctx *gofr.Context, collection string, customer Model) error
	Update(ctx *gofr.Context, collection string, customer Model) error
	Delete(ctx *gofr.Context, collection string, customer Model) error
}
