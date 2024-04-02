package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		1708322067: createTableEmployee(),
		1708322089: addEmployeeInRedis(),
		1708322090: createTopicsForStore(),
	}
}
