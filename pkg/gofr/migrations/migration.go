package migrations

import (
	"github.com/gogo/protobuf/sortkeys"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/redis"
)

type MigrateFunc func(d Datasource) error

type Migration struct {
	UP MigrateFunc
}

type Migrator interface {
	Migrate(keys []int64, migrationsMap map[int64]Migration, container *container.Container)
}

type Logger interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type Datasource struct {
	DB    SQL
	Redis *redis.Redis

	Logger
}

func Migrate(migrationsMap map[int64]Migration, dialect Migrator, container *container.Container) {
	// Sort migrations by version
	keys := make([]int64, 0, len(migrationsMap))
	for k := range migrationsMap {
		keys = append(keys, k)
	}

	sortkeys.Int64s(keys)

	dialect.Migrate(keys, migrationsMap, container)
}
