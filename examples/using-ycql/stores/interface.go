package stores

import (
	"gofr.dev/examples/using-ycql/models"
	"gofr.dev/pkg/gofr"
)

type Shop interface {
	Get(ctx *gofr.Context, filter models.Shop) []models.Shop
	Create(ctx *gofr.Context, data models.Shop) ([]models.Shop, error)
	Delete(ctx *gofr.Context, id string) error
	Update(ctx *gofr.Context, data models.Shop) ([]models.Shop, error)
}
