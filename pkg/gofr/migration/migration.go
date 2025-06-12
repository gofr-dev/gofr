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

type transactionData struct {
	StartTime       time.Time
	MigrationNumber int64

	SQLTx   *gofrSql.Tx
	RedisTx goRedis.Pipeliner
}

func Run(migrationsMap map[int64]Migrate, c *container.Container) {
	invalidKeys, keys := getKeys(migrationsMap)
	if len(invalidKeys) > 0 {
		c.Errorf("migration run failed! UP not defined for the following keys: %v", invalidKeys)

		return
	}

	sortkeys.Int64s(keys)

	ds, mg, ok := getMigrator(c)
	ds.Logger = c.Logger

	// Returning with an error log as migration would eventually fail as No databases are initialized.
	// Pub/Sub is considered as initialized if its configurations are given.
	if !ok {
		c.Errorf("no migrations are running as datasources are not initialized")

		return
	}

	err := mg.checkAndCreateMigrationTable(c)
	if err != nil {
		c.Fatalf("failed to create gofr_migration table, err: %v", err)

		return
	}

	lastMigration := mg.getLastMigration(c)

	for _, currentMigration := range keys {
		if currentMigration <= lastMigration {
			c.Infof("skipping migration %v", currentMigration)

			continue
		}

		c.Logger.Infof("running migration %v", currentMigration)

		migrationInfo := mg.beginTransaction(c)

		// Replacing the objects in datasource object only for those Datasources which support transactions.
		ds.SQL = migrationInfo.SQLTx
		ds.Redis = migrationInfo.RedisTx

		migrationInfo.StartTime = time.Now()
		migrationInfo.MigrationNumber = currentMigration

		err = migrationsMap[currentMigration].UP(ds)
		if err != nil {
			c.Logger.Errorf("failed to run migration : [%v], err: %v", currentMigration, err)

			mg.rollback(c, migrationInfo)

			return
		}

		err = mg.commitMigration(c, migrationInfo)
		if err != nil {
			c.Errorf("failed to commit migration, err: %v", err)

			mg.rollback(c, migrationInfo)

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

func getMigrator(c *container.Container) (Datasource, migrator, bool) {
	var (
		ok bool
		ds Datasource
		mg migrator = &ds
	)

	// Define datasource initializers in a single slice
	datasources := []struct {
		isAvailable func() bool
		initialize  func()
		name        string
	}{
		{
			isAvailable: func() bool { return !isNil(c.SQL) },
			initialize: func() {
				ds.SQL = c.SQL
				s := &sqlDS{ds.SQL}
				mg = s.apply(mg)
			},
			name: "SQL",
		},
		{
			isAvailable: func() bool { return !isNil(c.Redis) },
			initialize: func() {
				ds.Redis = c.Redis
				mg = redisDS{ds.Redis}.apply(mg)
			},
			name: "redis",
		},
		{
			isAvailable: func() bool { return !isNil(c.Clickhouse) },
			initialize: func() {
				ds.Clickhouse = c.Clickhouse
				mg = clickHouseDS{ds.Clickhouse}.apply(mg)
			},
			name: "Clickhouse",
		},
		{
			isAvailable: func() bool { return c.PubSub != nil },
			initialize: func() {
				ds.PubSub = c.PubSub
				mg = pubsubDS{c.PubSub}.apply(mg)
			},
			name: "pubsub",
		},
		{
			isAvailable: func() bool { return !isNil(c.Cassandra) },
			initialize: func() {
				ds.Cassandra = cassandraDS{c.Cassandra}
				mg = cassandraDS{c.Cassandra}.apply(mg)
			},
			name: "Cassandra",
		},
		{
			isAvailable: func() bool { return !isNil(c.Mongo) },
			initialize: func() {
				ds.Mongo = mongoDS{c.Mongo}
				mg = mongoDS{c.Mongo}.apply(mg)
			},
			name: "Mongo",
		},
		{
			isAvailable: func() bool { return !isNil(c.ArangoDB) },
			initialize: func() {
				ds.ArangoDB = arangoDS{c.ArangoDB}
				mg = arangoDS{c.ArangoDB}.apply(mg)
			},
			name: "ArangoDB",
		},
		{
			isAvailable: func() bool { return !isNil(c.SurrealDB) },
			initialize: func() {
				ds.SurrealDB = surrealDS{c.SurrealDB}
				mg = surrealDS{c.SurrealDB}.apply(mg)
			},
			name: "surrealDB",
		},
		{
			isAvailable: func() bool { return !isNil(c.DGraph) },
			initialize: func() {
				ds.DGraph = dgraphDS{c.DGraph}
				mg = dgraphDS{c.DGraph}.apply(mg)
			},
			name: "dgraph",
		},
	}

	// Initialize each available datasource
	for _, ds := range datasources {
		if ds.isAvailable() {
			ok = true

			ds.initialize()

			c.Debug("initialized data source for " + ds.name)
		}
	}

	return ds, mg, ok
}

func isNil(i any) bool {
	// Get the value of the interface
	val := reflect.ValueOf(i)

	// If the interface is not assigned or is nil, return true
	return !val.IsValid() || val.IsNil()
}
