package migrations

import (
	"context"

	"gofr.dev/pkg/gofr/migration"
)

func addRedisKey() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			return d.Redis.Set(context.Background(), "test", "test-value", 0).Err()
		},
	}
}
