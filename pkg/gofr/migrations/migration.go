package migrations

import (
	"fmt"
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

func Migrate(migrationsMap map[int64]Migration, migrator Migrator, container *container.Container) {
	if migrationsMap == nil || migrator == nil {
		container.Logger.Error("Migration failed as migrationsMap or migrator is nil")

		return
	}

	invalidKeys := ""

	// Sort migrations by version
	keys := make([]int64, 0, len(migrationsMap))
	for k, v := range migrationsMap {
		if v.UP == nil {
			invalidKeys += fmt.Sprintf("%v,", k)

			continue
		}

		keys = append(keys, k)
	}

	if len(invalidKeys) > 0 {
		container.Logger.Errorf("Migrations Failed as UP not defined for the following keys : %v", invalidKeys[0:len(invalidKeys)-1])

		return
	}

	sortkeys.Int64s(keys)

	migrator.Migrate(keys, migrationsMap, container)
}
