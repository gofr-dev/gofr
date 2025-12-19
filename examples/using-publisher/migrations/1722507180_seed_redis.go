package migrations

import (
	"context"

	"gofr.dev/pkg/gofr/migration"
)

func seedRedis() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			return d.Redis.Set(context.Background(), "gofr-example-seed", "ok", 0).Err()
		},
	}
}

