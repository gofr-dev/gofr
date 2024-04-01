package migration

import (
	"reflect"
	"time"

	"github.com/gogo/protobuf/sortkeys"
	goRedis "github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

type MigrateFunc func(d Datasource) error

type Migrate struct {
	UP MigrateFunc
}

// TODO : Use composition to handler different databases which would also remove this nolint
//
//nolint:gocyclo // reducing complexity may hamper readability.
func Run(migrationsMap map[int64]Migrate, c container.Interface) {
	invalidKeys, keys := getKeys(migrationsMap)
	if len(invalidKeys) > 0 {
		c.Errorf("Run Failed! UP not defined for the following keys: %v", invalidKeys)

		return
	}

	sortkeys.Int64s(keys)

	var lastMigration int64

	if !reflect.ValueOf(c.GetDB()).IsNil() {
		err := ensureSQLMigrationTableExists(c)
		if err != nil {
			c.Errorf("Unable to verify sql migration table due to: %v", err)

			return
		}

		lastMigration = getSQLLastMigration(c)
	}

	if !reflect.ValueOf(c.GetRedis()).IsNil() {
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

		var (
			datasource Datasource
			sqlTx      *gofrSql.Tx
			redisTx    goRedis.Pipeliner
			err        error
		)

		if !reflect.ValueOf(c.GetPubSub()).IsNil() {
			datasource.PubSub = newPubSub(c.GetPubSub())
		}

		if c.GetDB() != nil {
			sqlTx, err = c.GetDB().Begin()
			if err != nil {
				c.Errorf("unable to begin transaction: %v", err)

				return
			}

			datasource.SQL = newMysql(sqlTx)
		}

		if c.GetRedis() != nil {
			redisTx = c.GetRedis().TxPipeline()

			datasource.Redis = newRedis(redisTx)
		}

		err = migrationsMap[currentMigration].UP(datasource)
		if err != nil {
			rollbackAndLog(c, currentMigration, sqlTx, err)

			return
		}

		if c.GetDB() != nil {
			sqlPostRun(c, sqlTx, currentMigration, start)
		}

		if c.GetRedis() != nil {
			redisPostRun(c, redisTx, currentMigration, start)
		}
	}
}

func getKeys(migrationsMap map[int64]Migrate) (invalidKey, keys []int64) {
	invalidKey = make([]int64, 0, len(migrationsMap))
	keys = make([]int64, 0, len(migrationsMap))

	for k, v := range migrationsMap {
		if v.UP == nil {
			invalidKey = append(invalidKey, k)

			continue
		}

		keys = append(keys, k)
	}

	return invalidKey, keys
}
