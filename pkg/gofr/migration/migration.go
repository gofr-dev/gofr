package migration

import (
	"time"

	"github.com/gogo/protobuf/sortkeys"

	"gofr.dev/pkg/gofr/container"
	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

type MigrateFunc func(d Datasource) error

type Migrate struct {
	UP MigrateFunc
}

func Run(migrationsMap map[int64]Migrate, c *container.Container) {
	invalidKeys, keys := getKeys(migrationsMap)
	if len(invalidKeys) > 0 {
		c.Errorf("Run Failed! UP not defined for the following keys: %v", invalidKeys)

		return
	}

	sortkeys.Int64s(keys)

	var (
		ok bool
		ds Datasource
		mg Migrator = ds
	)

	mg, ok = updateMigrator(c, ds, mg)

	if c.PubSub != nil {
		ok = true
	}

	// Returning with an error log as migration would eventually fail as No databases are initialized.
	// Pub/Sub is considered as initialized if its configurations are given.
	if !ok {
		c.Errorf("No Migrations are running as datasources are not initialized")

		return
	}

	err := mg.checkAndCreateMigrationTable(c)
	if err != nil {
		c.Errorf("Failed to create migration table: %v", err)

		return
	}

	lastMigration := mg.getLastMigration(c)

	for _, currentMigration := range keys {
		if currentMigration <= lastMigration {
			continue
		}

		transactionsObjects := mg.beginTransaction(c)

		ds.SQL = newMysql(transactionsObjects.SQLTx)
		ds.Redis = newRedis(transactionsObjects.RedisTx)
		ds.PubSub = newPubSub(c.PubSub)

		transactionsObjects.StartTime = time.Now()
		transactionsObjects.MigrationNumber = currentMigration

		err = migrationsMap[currentMigration].UP(ds)
		if err != nil {
			mg.rollback(c, transactionsObjects)

			return
		}

		err = mg.commitMigration(c, transactionsObjects)
		if err != nil {
			c.Errorf("Failed to migrationData migration: %v", err)

			mg.rollback(c, transactionsObjects)

			return
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

func updateMigrator(c *container.Container, ds Datasource, mg Migrator) (Migrator, bool) {
	var ok bool

	sql, _ := c.SQL.(*gofrSql.DB)

	if sql != nil && sql.DB != nil {
		ok = true

		ds.SQL = sql

		mg = sqlMigratorObject{ds.SQL}.apply(mg)
	}

	redisClient, _ := c.Redis.(*gofrRedis.Redis)

	if redisClient != nil && redisClient.Client != nil {
		ok = true

		ds.Redis = redisClient

		mg = redisMigratorObject{ds.Redis}.apply(mg)
	}

	return mg, ok
}
