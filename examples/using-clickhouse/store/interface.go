package store

import (
	"github.com/google/uuid"

	"gofr.dev/examples/using-clickhouse/models"
	"gofr.dev/pkg/gofr"
)

type Store interface {
	Get(ctx *gofr.Context) ([]models.User, error)
	GetByID(ctx *gofr.Context, id uuid.UUID) (models.User, error)
	Create(ctx *gofr.Context, user models.User) (models.User, error)
}
