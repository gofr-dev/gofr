package migration

import (
	goRedis "github.com/redis/go-redis/v9"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	"time"

	"github.com/gogo/protobuf/sortkeys"

	"gofr.dev/pkg/gofr/container"
)

type MigrateFunc func(d Datasource) error

type Migrate struct {
	UP MigrateFunc
}

func Run(migrationsMap map[int64]Migrate, c *container.Container) {
	invalidKeys, keys := getKeys(migrationsMap)
	if len(invalidKeys) > 0 {
		c.Logger.Errorf("Run Failed! UP not defined for the following keys: %v", invalidKeys)

		return
	}

	sortkeys.Int64s(keys)

	var lastMigration int64

	if c.DB != nil {
		err := ensureSQLMigrationTableExists(c)
		if err != nil {
			c.Logger.Errorf("Unable to verify sql migration table due to: %v", err)

			return
		}

		lastMigration = getSQLLastMigration(c)
	}

	if c.Redis != nil {
		redisLastMigration := getRedisLastMigration(c)

		switch {
		case redisLastMigration == -1:
			return

		case redisLastMigration > lastMigration:
			lastMigration = redisLastMigration

		}
	}

	for _, currentMigration := range keys {
		if currentMigration <= lastMigration {
			continue
		}

		start := time.Now()

		var datasource Datasource
		var sqlTx *gofrSql.Tx
		var redisTx goRedis.Pipeliner
		var err error

		if c.DB != nil {
			sqlTx, err = c.DB.Begin()
			if err != nil {
				rollbackAndLog(c, sqlTx)

				return
			}

			datasource.DB = newMysql(sqlTx)
		}

		if c.Redis != nil {
			redisTx = c.Redis.TxPipeline()

			datasource.Redis = newRedis(redisTx)
		}

		err = migrationsMap[currentMigration].UP(datasource)
		if err != nil {
			rollbackAndLog(c, sqlTx)

			return
		}

		if c.DB != nil {
			sqlPostRun(c, sqlTx, currentMigration, start)
		}

		if c.Redis != nil {
			redisPostRun(c, redisTx, currentMigration, start)
		}
	}
}

func getKeys(migrationsMap map[int64]Migrate) ([]int64, []int64) {
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
