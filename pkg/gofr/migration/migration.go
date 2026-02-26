package migration

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"time"

	"github.com/gogo/protobuf/sortkeys"
	"github.com/google/uuid"
	goRedis "github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

var (
	errLockAcquisitionFailed = errors.New("failed to acquire migration lock")
	errLockReleaseFailed     = errors.New("failed to release migration lock")
)

const (
	// lockKey is the key used for distributed locking.
	lockKey = "gofr_migrations_lock"

	// Default values for configuration.
	defaultRetry = 500 * time.Millisecond
	// defaultLockTTL is the duration for which the migration lock is valid.
	// It is kept at 15 seconds to provide a safety margin for network jitters or transient failures.
	defaultLockTTL = 15 * time.Second
	// defaultRefresh is the interval at which the migration lock is renewed.
	// A 5-second interval allows for up to 2 failed refresh attempts before the 15-second TTL expires,
	// ensuring the lock stays robust while still allowing fairly quick recovery if a process crashes.
	defaultRefresh = 5 * time.Second
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

	// Create migration tables BEFORE acquiring locks (lock table must exist first)
	err := mg.checkAndCreateMigrationTable(c)
	if err != nil {
		c.Fatalf("failed to create gofr_migration table, err: %v", err)

		return
	}

	// Optimistic pre-check: only acquire locks if there MIGHT be new migrations
	// This is a fast path to avoid lock contention when no migrations are needed
	lastMigration, err := mg.getLastMigration(c)
	if err != nil {
		c.Fatalf("migration failed: could not verify migration state from datasources, err: %v", err)

		return
	}

	if !hasNewMigrations(keys, lastMigration) {
		c.Infof("no new migrations to run")

		return
	}

	ownerID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())

	if err = mg.lock(ctx, cancel, c, ownerID); err != nil {
		cancel()

		if unlockErr := mg.unlock(c, ownerID); unlockErr != nil {
			c.Errorf("failed to cleanup lock after acquisition failure: %v", unlockErr)
		}

		c.Fatalf("migration failed: could not acquire locks, err: %v", err)

		return
	}

	defer func() {
		cancel()

		if err = mg.unlock(c, ownerID); err != nil {
			c.Errorf("failed to unlock during cleanup: %v", err)
		}
	}()

	runMigrations(ctx, c, mg, &ds, migrationsMap, keys, lastMigration)
}

func hasNewMigrations(keys []int64, lastMigration int64) bool {
	for _, k := range keys {
		if k > lastMigration {
			return true
		}
	}

	return false
}

func runMigrations(ctx context.Context, c *container.Container, mg migrator, ds *Datasource, migrationsMap map[int64]Migrate,
	keys []int64, lastMigration int64) {
	for _, currentMigration := range keys {
		if currentMigration <= lastMigration {
			c.Infof("skipping migration %v", currentMigration)

			continue
		}

		// Check if lock refresh failed before starting the migration
		select {
		case <-ctx.Done():
			c.Fatalf("migration %v aborted: lock refresh failed", currentMigration)

			return
		default:
		}

		c.Infof("running migration %v", currentMigration)

		migrationInfo := mg.beginTransaction(c)

		// Check if lock refresh failed after starting the transaction but before execution
		select {
		case <-ctx.Done():
			mg.rollback(c, migrationInfo)
			c.Fatalf("migration %v aborted: lock refresh failed", currentMigration)

			return
		default:
		}

		// Replacing the objects in datasource object only for those Datasources which support transactions.
		ds.SQL = migrationInfo.SQLTx
		ds.Redis = migrationInfo.RedisTx

		if !isNil(migrationInfo.OracleTx) {
			ds.Oracle = &oracleTransactionWrapper{tx: migrationInfo.OracleTx}
		}

		migrationInfo.StartTime = time.Now().UTC()
		migrationInfo.MigrationNumber = currentMigration

		err := migrationsMap[currentMigration].UP(*ds)

		// Check if lock refresh failed during migration execution
		select {
		case <-ctx.Done():
			mg.rollback(c, migrationInfo)
			c.Fatalf("migration %v aborted: lock refresh failed during execution", currentMigration)

			return
		default:
		}

		if err != nil {
			c.Errorf("failed to run migration : [%v], err: %v", currentMigration, err)
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

func getKeys(migrationsMap map[int64]Migrate) (invalidKeys, validKeys []int64) {
	for k := range migrationsMap {
		if migrationsMap[k].UP != nil {
			validKeys = append(validKeys, k)
		} else {
			invalidKeys = append(invalidKeys, k)
		}
	}

	return invalidKeys, validKeys
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
	initializers := getInitializers(c, ds)

	var active []datasourceInitializer

	for _, init := range initializers {
		if init.condition() {
			active = append(active, init)
		}
	}

	if len(active) == 0 {
		return nil, false
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].logIdentifier < active[j].logIdentifier
	})

	// Build the chain starting from the base Datasource
	for i := len(active) - 1; i >= 0; i-- {
		active[i].setDS()
		mg = active[i].apply(mg)
		c.Debugf("initialized data source for %s", active[i].logIdentifier)
	}

	return mg, true
}

func getInitializers(c *container.Container, ds *Datasource) []datasourceInitializer {
	return []datasourceInitializer{
		{
			condition:     func() bool { return !isNil(c.SQL) },
			setDS:         func() { ds.SQL = c.SQL },
			apply:         func(m migrator) migrator { return (&sqlDS{c.SQL}).apply(m) },
			logIdentifier: "SQL",
		},
		{
			condition:     func() bool { return !isNil(c.Redis) },
			setDS:         func() { ds.Redis = c.Redis },
			apply:         func(m migrator) migrator { return redisDS{c.Redis}.apply(m) },
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
			apply:         func(m migrator) migrator { return clickHouseDS{c.Clickhouse}.apply(m) },
			logIdentifier: "Clickhouse",
		},
		{
			condition:     func() bool { return !isNil(c.Oracle) },
			setDS:         func() { ds.Oracle = c.Oracle },
			apply:         func(m migrator) migrator { return oracleDS{c.Oracle}.apply(m) },
			logIdentifier: "Oracle",
		},
		{
			condition:     func() bool { return !isNil(c.PubSub) },
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
			apply:         func(m migrator) migrator { return (&openTSDBMigrator{filePath: "gofr_migrations.json", migrator: m}) },
			logIdentifier: "OpenTSDB",
		},
		{
			condition:     func() bool { return !isNil(c.ScyllaDB) },
			setDS:         func() { ds.ScyllaDB = c.ScyllaDB },
			apply:         func(m migrator) migrator { return scyllaDS{c.ScyllaDB}.apply(m) },
			logIdentifier: "ScyllaDB",
		},
	}
}

func isNil(i any) bool {
	if i == nil {
		return true
	}

	val := reflect.ValueOf(i)
	k := val.Kind()

	if k == reflect.Ptr || k == reflect.Interface || k == reflect.Slice || k == reflect.Map || k == reflect.Chan || k == reflect.Func {
		return val.IsNil()
	}

	return false
}
