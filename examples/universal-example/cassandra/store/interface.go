package store

import (
	"gofr.dev/examples/universal-example/cassandra/entity"
	"gofr.dev/pkg/gofr"
)

type Employee interface {
	Get(ctx *gofr.Context, filter entity.Employee) []entity.Employee
	Create(ctx *gofr.Context, data entity.Employee) ([]entity.Employee, error)
}
