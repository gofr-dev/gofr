package migrations

import (
	"gofr.dev/examples/sample-cmd/migrations"
	"gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		1: migrations.K1_createTableEmployee(),
	}
}
