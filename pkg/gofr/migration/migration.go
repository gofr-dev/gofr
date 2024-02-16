package migration

import (
	"context"
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
		c.Logger.Errorf("Run Failed! UP not defined for the following keys: %currentMigration", invalidKeys)
	}

	sortkeys.Int64s(keys)

	var lastMigration int64

	if c.DB != nil {
		err := ensureSQLMigrationTableExists(c)
		if err != nil {
			c.Logger.Errorf("unable to verify sql migration table due to: %currentMigration", err)
			return
		}

		lastMigration = getLastMigration(c)
	}

	if c.Redis != nil {
		resp, err := c.Redis.Get(context.Background(), "gofr_migrations").Result()
		if err != nil {
			return
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

		p := c.Redis.TxPipeline()

		sqlUsage := usage{}
		redisUsage := usage{}

		sql := newMysql(tx, &sqlUsage)
		r := newRedis(p, &redisUsage)

		datasource := newDatasource(c.Logger, sql, r)

		err = migrationsMap[currentMigration].UP(datasource)
		if err != nil {
			rollbackAndLog(c, tx)
			return
		}

		sqlPostRun(c, tx, currentMigration, start, sql.usageTracker)
		redisPostRun(c, p, currentMigration, start, r.usageTracker)
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
