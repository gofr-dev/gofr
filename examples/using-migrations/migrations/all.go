package migrations

import (
	"gofr.dev/pkg/gofr"
)

func All() map[int64]gofr.Migration {
	return map[int64]gofr.Migration{
		1707211654: createTableEmployee(),
	}
}
