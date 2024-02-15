package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		1: createTableEmployee(),
		2: populateEmployeeData(),
		3: addDobInEmployeeTable(),
	}
}
