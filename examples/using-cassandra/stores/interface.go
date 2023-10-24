package stores

import (
	"gofr.dev/examples/using-cassandra/models"
	"gofr.dev/pkg/gofr"
)

type Person interface {
	Get(ctx *gofr.Context, filter models.Person) []models.Person
	Create(ctx *gofr.Context, data models.Person) ([]models.Person, error)
	Delete(ctx *gofr.Context, id string) error
	Update(ctx *gofr.Context, data models.Person) ([]models.Person, error)
}
