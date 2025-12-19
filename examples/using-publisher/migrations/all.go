package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		1721801313: createTopics(),
		1722507180: seedRedis(),
	}
}
