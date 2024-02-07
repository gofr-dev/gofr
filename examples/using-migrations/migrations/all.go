package migrations

import (
	"gofr.dev/pkg/gofr/migrations"
)

func All() map[int64]migrations.Migration {
	return map[int64]migrations.Migration{
		1707211654: createTableEmployee(),
	}
}
