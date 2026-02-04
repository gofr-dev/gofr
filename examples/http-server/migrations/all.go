package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		20240204120000: createEmployeeTable(),
		20240204120100: addRedisKey(),
	}
}
