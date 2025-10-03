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

	SQLTx    *gofrSql.Tx
	RedisTx  goRedis.Pipeliner
	OracleTx container.OracleTx
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

		if migrationInfo.OracleTx != nil {
			ds.Oracle = &oracleTransactionWrapper{tx: migrationInfo.OracleTx}
		}

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
		ds Datasource
		mg migrator = &ds
		ok bool
	)

	mg, ok = initializeDatasources(c, &ds, mg)

	return ds, mg, ok
}

type datasourceInitializer struct {
	condition     func() bool
	setDS         func()
	apply         func(m migrator) migrator
	logIdentifier string
}

func initializeDatasources(c *container.Container, ds *Datasource, mg migrator) (migrator, bool) {
	var initialized bool

	initializers := []datasourceInitializer{
		{
			condition:     func() bool { return !isNil(c.SQL) },
			setDS:         func() { ds.SQL = c.SQL },
			apply:         func(m migrator) migrator { return (&sqlDS{ds.SQL}).apply(m) },
			logIdentifier: "SQL",
		},
		{
			condition:     func() bool { return !isNil(c.Redis) },
			setDS:         func() { ds.Redis = c.Redis },
			apply:         func(m migrator) migrator { return redisDS{ds.Redis}.apply(m) },
			logIdentifier: "Redis",
		},
		{
			condition:     func() bool { return !isNil(c.DGraph) },
			setDS:         func() { ds.DGraph = dgraphDS{c.DGraph} },
			apply:         func(m migrator) migrator { return dgraphDS{c.DGraph}.apply(m) },
			logIdentifier: "DGraph",
		},
		{
			condition:     func() bool { return !isNil(c.Clickhouse) },
			setDS:         func() { ds.Clickhouse = c.Clickhouse },
			apply:         func(m migrator) migrator { return clickHouseDS{ds.Clickhouse}.apply(m) },
			logIdentifier: "Clickhouse",
		},
		{
			condition:     func() bool { return !isNil(c.Oracle) },
			setDS:         func() { ds.Oracle = c.Oracle },
			apply:         func(m migrator) migrator { return oracleDS{c.Oracle}.apply(m) },
			logIdentifier: "Oracle",
		},

		{
			condition:     func() bool { return c.PubSub != nil },
			setDS:         func() { ds.PubSub = c.PubSub },
			apply:         func(m migrator) migrator { return pubsubDS{c.PubSub}.apply(m) },
			logIdentifier: "PubSub",
		},
		{
			condition:     func() bool { return !isNil(c.Cassandra) },
			setDS:         func() { ds.Cassandra = cassandraDS{c.Cassandra} },
			apply:         func(m migrator) migrator { return cassandraDS{c.Cassandra}.apply(m) },
			logIdentifier: "Cassandra",
		},
		{
			condition:     func() bool { return !isNil(c.Mongo) },
			setDS:         func() { ds.Mongo = mongoDS{c.Mongo} },
			apply:         func(m migrator) migrator { return mongoDS{c.Mongo}.apply(m) },
			logIdentifier: "Mongo",
		},
		{
			condition:     func() bool { return !isNil(c.ArangoDB) },
			setDS:         func() { ds.ArangoDB = arangoDS{c.ArangoDB} },
			apply:         func(m migrator) migrator { return arangoDS{c.ArangoDB}.apply(m) },
			logIdentifier: "ArangoDB",
		},
		{
			condition:     func() bool { return !isNil(c.SurrealDB) },
			setDS:         func() { ds.SurrealDB = surrealDS{c.SurrealDB} },
			apply:         func(m migrator) migrator { return surrealDS{c.SurrealDB}.apply(m) },
			logIdentifier: "SurrealDB",
		},
		{
			condition:     func() bool { return !isNil(c.Elasticsearch) },
			setDS:         func() { ds.Elasticsearch = c.Elasticsearch },
			apply:         func(m migrator) migrator { return elasticsearchDS{c.Elasticsearch}.apply(m) },
			logIdentifier: "Elasticsearch",
		},
		{
			condition:     func() bool { return !isNil(c.OpenTSDB) },
			setDS:         func() { ds.OpenTSDB = c.OpenTSDB },
			apply:         func(m migrator) migrator { return openTSDBDS{c.OpenTSDB, "gofr_migrations.json"}.apply(m) },
			logIdentifier: "OpenTSDB",
		},
		{
			condition:     func() bool { return !isNil(c.ScyllaDB) },
			setDS:         func() { ds.ScyllaDB = c.ScyllaDB },
			apply:         func(m migrator) migrator { return scyllaDS{c.ScyllaDB}.apply(m) },
			logIdentifier: "ScyllaDB",
		},
	}

	for _, init := range initializers {
		if !init.condition() {
			continue
		}

		init.setDS()
		mg = init.apply(mg)
		initialized = true

		c.Debugf("initialized data source for %s", init.logIdentifier)
	}

	return mg, initialized
}

func isNil(i any) bool {
	// Get the value of the interface.
	val := reflect.ValueOf(i)

	// If the interface is not assigned or is nil, return true.
	return !val.IsValid() || val.IsNil()
}
