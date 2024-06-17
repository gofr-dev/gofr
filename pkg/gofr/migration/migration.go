package migration

import (
	"reflect"
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
		c.Errorf("migration run failed! UP not defined for the following keys: %v", invalidKeys)

		return
	}

	sortkeys.Int64s(keys)

	ds, mg, ok := getMigrator(c)

	// Returning with an error log as migration would eventually fail as No databases are initialized.
	// Pub/Sub is considered as initialized if its configurations are given.
	if !ok {
		c.Errorf("no migrations are running as datasources are not initialized")

		return
	}

	err := mg.checkAndCreateMigrationTable(c)
	if err != nil {
		c.Errorf("failed to create gofr_migration table, err: %v", err)

		return
	}

	lastMigration := mg.getLastMigration(c)

	for _, currentMigration := range keys {
		if currentMigration <= lastMigration {
			c.Debugf("skipping migration %v", currentMigration)

			continue
		}

		c.Logger.Debugf("running migration %v", currentMigration)

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
			c.Errorf("failed to commit migration, err: %v", err)

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

func getMigrator(c *container.Container) (Datasource, Migrator, bool) {
	var (
		ok bool
		ds Datasource
		mg Migrator = ds
	)

	if !isNil(c.SQL) {
		ok = true

		ds.SQL = c.SQL

		mg = sqlMigratorObject{ds.SQL}.apply(mg)

		c.Debug("initialized data source for SQL")
	}

	if !isNil(c.Redis) {
		ok = true

		ds.Redis = c.Redis

		mg = redisMigratorObject{ds.Redis}.apply(mg)

		c.Debug("initialized data source for redis")
	}

	if c.PubSub != nil {
		ok = true
	}

	return ds, mg, ok
}

func isNil(i interface{}) bool {
	// Get the value of the interface
	val := reflect.ValueOf(i)

	// If the interface is not assigned or is nil, return true
	return !val.IsValid() || val.IsNil()
}
