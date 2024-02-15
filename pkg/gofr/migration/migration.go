package migration

import (
	"time"

	"github.com/gogo/protobuf/sortkeys"

	"gofr.dev/pkg/gofr/container"
)

type MigrateFunc func(d Datasource) error

type Migrate struct {
	UP MigrateFunc
}

func Run(migrationsMap map[int64]Migrate, c *container.Container) {
	invalidKeys, keys := getSequence(migrationsMap)
	if len(invalidKeys) > 0 {
		c.Logger.Errorf("Run Failed! UP not defined for the following keys: %v", invalidKeys)
	}

	sortkeys.Int64s(keys)

	var lastMigration int64

	if c.DB != nil {
		err := ensureSQLMigrationTableExists(c)
		if err != nil {
			c.Logger.Errorf("unable to verify sql migration table due to : %v", err)
			return
		}

		lastMigration = getLastMigration(c)
	}

	for _, v := range keys {
		if v <= lastMigration {
			continue
		}

		start := time.Now()

		tx, err := c.DB.Begin()
		if err != nil {
			rollbackAndLog(c, tx)
			return
		}

		//p := c.Redis.TxPipeline()

		sql := newMysql(tx)

		datasource := newDatasource(c.Logger, sql, nil)

		err = migrationsMap[v].UP(datasource)
		if err != nil {
			rollbackAndLog(c, tx)
			return
		}

		sqlPostRun(c, tx, v, start)
	}
}

func getSequence(migrationsMap map[int64]Migrate) ([]int64, []int64) {
	invalidKey := make([]int64, 0, len(migrationsMap))
	keys := make([]int64, 0, len(migrationsMap))

	for k, v := range migrationsMap {
		if v.UP == nil {
			invalidKey = append(invalidKey, k)

			continue
		}

		keys = append(keys, k)
	}

	return invalidKey, keys
}
