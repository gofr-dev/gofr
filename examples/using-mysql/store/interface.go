package store

import (
	"gofr.dev/examples/using-mysql/models"
	"gofr.dev/pkg/gofr"
)

type Store interface {
	Get(ctx *gofr.Context) ([]models.Employee, error)
	Create(ctx *gofr.Context, customer models.Employee) (models.Employee, error)
}
