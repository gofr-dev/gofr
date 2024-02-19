package migration

import (
	"context"
	"encoding/json"
	"strconv"
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
		table, err := c.Redis.HGetAll(context.Background(), "gofr_migrations").Result()
		if err != nil {
			return
		}

		val := make(map[int64]migration)

		for key, value := range table {
			integer_value, _ := strconv.ParseInt(key, 10, 64)

			if integer_value > lastMigration {
				lastMigration = integer_value
			}

			d := []byte(value)

			var migrationData migration

			_ = json.Unmarshal(d, &migrationData)

			val[integer_value] = migrationData
		}
	}

	for _, currentMigration := range keys {
		if currentMigration <= lastMigration {
			continue
		}

		start := time.Now()

		tx, err := c.DB.Begin()
		if err != nil {
			rollbackAndLog(c, tx)

			return
		}

		redisTx := c.Redis.TxPipeline()
		sql := newMysql(tx)
		r := newRedis(redisTx)

		datasource := newDatasource(c.Logger, sql, r)

		err = migrationsMap[currentMigration].UP(datasource)
		if err != nil {
			rollbackAndLog(c, tx)

			return
		}

		sqlPostRun(c, tx, currentMigration, start)
		redisPostRun(c, redisTx, currentMigration, start)
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
