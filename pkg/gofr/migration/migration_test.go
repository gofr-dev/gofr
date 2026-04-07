package migration

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	goRedis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var (
	errRandomDB      = errors.New("random db error")
	errGenericCommit = errors.New("commit error")
)

func TestMigration_InvalidKeys(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		c, _ := container.NewMockContainer(t)

		Run(map[int64]Migrate{
			1: {UP: nil},
		}, c)
	})

	assert.Contains(t, logs, "migration run failed! UP not defined for the following keys: [1]")
}

func TestMigration_NoDatasource(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		c := container.NewContainer(nil)
		c.Logger = logging.NewLogger(logging.DEBUG)

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				return nil
			}},
		}, c)
	})

	assert.Contains(t, logs, "no migrations are running")
}

func Test_getMigratorDBInitialisation(t *testing.T) {
	cntnr, _ := container.NewMockContainer(t)

	datasource, _, isInitialized := getMigrator(cntnr)

	assert.NotNil(t, datasource.SQL, "TEST Failed \nSQL not initialized, but should have been initialized")
	assert.NotNil(t, datasource.Redis, "TEST Failed \nRedis not initialized, but should have been initialized")
	assert.True(t, isInitialized, "TEST Failed \nNo datastores are Initialized")
}

func TestMigrationRunClickhouseSuccess(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				err := d.Clickhouse.Exec(t.Context(), "SELECT * FROM users")
				if err != nil {
					return err
				}

				d.Logger.Infof("Clickhouse Migration Ran Successfully")

				return nil
			}},
		}

		mockClickHouse, mockContainer := initializeClickHouseRunMocks(t)

		// Pre-check
		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil).Times(2)

		mockClickHouse.EXPECT().Exec(gomock.Any(), "SELECT * FROM users").Return(nil)
		mockClickHouse.EXPECT().Exec(gomock.Any(), insertChGoFrMigrationRow, int64(1),
			"UP", gomock.Any(), gomock.Any()).Return(nil)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "Migration 1 ran successfully")
	assert.Contains(t, logs, "Clickhouse Migration Ran Successfully")
}

func TestMigrationRunClickhouseMigrationFailure(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		mockClickHouse, mockContainer := initializeClickHouseRunMocks(t)

		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				err := d.Clickhouse.Exec(t.Context(), "SELECT * FROM users")
				if err != nil {
					return err
				}

				return nil
			}},
		}

		// Pre-check
		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil).Times(2)

		mockClickHouse.EXPECT().Exec(gomock.Any(), "SELECT * FROM users").Return(sql.ErrConnDone)

		Run(migrationMap, mockContainer)

		assert.True(t, mockClickHouse.ctrl.Satisfied())
	})

	assert.Contains(t, logs, "failed to run migration : [1], err: sql: connection is already closed")
}

func TestMigrationRunClickhouseMigrationFailureWhileCheckingTable(t *testing.T) {
	mockClickHouse, mockContainer := initializeClickHouseRunMocks(t)

	testutil.StderrOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				err := d.Clickhouse.Exec(t.Context(), "SELECT * FROM users")
				if err != nil {
					return err
				}

				return nil
			}},
		}

		// checkAndCreateMigrationTable is called first
		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(sql.ErrConnDone)

		Run(migrationMap, mockContainer)
	})

	assert.True(t, mockClickHouse.ctrl.Satisfied())
}

func TestMigrationRunClickhouseCurrentMigrationEqualLastMigration(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			0: {UP: func(d Datasource) error {
				err := d.Clickhouse.Exec(t.Context(), "SELECT * FROM users")
				if err != nil {
					return err
				}

				return nil
			}},
		}

		mockClickHouse, mockContainer := initializeClickHouseRunMocks(t)

		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "no new migrations to run")
}

func TestMigrationRunClickhouseCommitError(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				err := d.Clickhouse.Exec(t.Context(), "SELECT * FROM users")
				if err != nil {
					return err
				}

				return nil
			}},
		}

		mockClickHouse, mockContainer := initializeClickHouseRunMocks(t)

		// Pre-check
		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil).Times(2)

		mockClickHouse.EXPECT().Exec(gomock.Any(), "SELECT * FROM users").Return(nil)
		mockClickHouse.EXPECT().Exec(gomock.Any(), insertChGoFrMigrationRow, int64(1),
			"UP", gomock.Any(), gomock.Any()).Return(sql.ErrConnDone)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "failed to commit migration, err: sql: connection is already closed")
}

func initializeClickHouseRunMocks(t *testing.T) (*MockClickhouse, *container.Container) {
	t.Helper()

	mockClickHouse := NewMockClickhouse(gomock.NewController(t))

	mockContainer, _ := container.NewMockContainer(t)
	mockContainer.SQL = nil
	mockContainer.Redis = nil
	mockContainer.Mongo = nil
	mockContainer.Cassandra = nil
	mockContainer.PubSub = nil
	mockContainer.ArangoDB = nil
	mockContainer.SurrealDB = nil
	mockContainer.DGraph = nil
	mockContainer.Elasticsearch = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil
	mockContainer.Oracle = nil
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)
	mockContainer.Clickhouse = mockClickHouse

	return mockClickHouse, mockContainer
}

func TestMigration_SQLLockError(t *testing.T) {
	mockContainer, mocks := container.NewMockContainer(t)

	// Disable other datasources
	mockContainer.Redis = nil
	mockContainer.Cassandra = nil
	mockContainer.Clickhouse = nil
	mockContainer.Mongo = nil
	mockContainer.ArangoDB = nil
	mockContainer.Elasticsearch = nil
	mockContainer.Oracle = nil
	mockContainer.PubSub = nil
	mockContainer.DGraph = nil
	mockContainer.SurrealDB = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil

	ctrl := gomock.NewController(t)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	migrationMap := map[int64]Migrate{
		1: {UP: func(_ Datasource) error { return nil }},
	}

	createMigrations := `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`
	createLocks := `CREATE TABLE IF NOT EXISTS gofr_migration_locks (
    lock_key VARCHAR(64) PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL
);`

	// 1. checkAndCreateMigrationTable
	mocks.SQL.ExpectExec(createMigrations).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec(createLocks).WillReturnResult(sqlmock.NewResult(0, 0))

	// 2. getLastMigration
	mocks.SQL.ExpectQuery("SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;").WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(0))

	// 3. lock fails with non-duplicate error
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errRandomDB)

	// 4. unlock in defer
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))

	// Expectations for logger
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Fatalf(gomock.Any(), gomock.Any()).Times(1)

	Run(migrationMap, mockContainer)
}

func TestMigration_CommitFailure(t *testing.T) {
	mockContainer, mocks := container.NewMockContainer(t)

	// Disable other datasources
	mockContainer.Redis = nil
	mockContainer.Cassandra = nil
	mockContainer.Clickhouse = nil
	mockContainer.Mongo = nil
	mockContainer.ArangoDB = nil
	mockContainer.Elasticsearch = nil
	mockContainer.Oracle = nil
	mockContainer.PubSub = nil
	mockContainer.DGraph = nil
	mockContainer.SurrealDB = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil

	ctrl := gomock.NewController(t)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	migrationMap := map[int64]Migrate{
		1: {UP: func(_ Datasource) error { return nil }},
	}

	createMigrations := `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`
	createLocks := `CREATE TABLE IF NOT EXISTS gofr_migration_locks (
    lock_key VARCHAR(64) PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL
);`

	// 1. checkAndCreateMigrationTable
	mocks.SQL.ExpectExec(createMigrations).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec(createLocks).WillReturnResult(sqlmock.NewResult(0, 0))

	// 2. getLastMigration
	mocks.SQL.ExpectQuery("SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;").WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(0))

	// 3. lock
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. re-fetch getLastMigration under lock
	mocks.SQL.ExpectQuery("SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;").WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(0))

	// 5. beginTransaction
	mocks.SQL.ExpectBegin()

	// 6. commitMigration fails
	testErr := errGenericCommit

	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mocks.SQL.ExpectCommit().WillReturnError(testErr)

	// 7. rollback in runMigrations
	mocks.SQL.ExpectRollback()

	// 8. unlock in defer
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	// Expectations for logger
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Fatalf(gomock.Any(), gomock.Any()).AnyTimes()

	Run(migrationMap, mockContainer)
}

func TestMigration_RaceCondition_SkipUnderLock(t *testing.T) {
	mockContainer, mocks := container.NewMockContainer(t)

	// Disable other datasources
	mockContainer.Redis = nil
	mockContainer.Cassandra = nil
	mockContainer.Clickhouse = nil
	mockContainer.Mongo = nil
	mockContainer.ArangoDB = nil
	mockContainer.Elasticsearch = nil
	mockContainer.Oracle = nil
	mockContainer.PubSub = nil
	mockContainer.DGraph = nil
	mockContainer.SurrealDB = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil

	ctrl := gomock.NewController(t)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	migrationMap := map[int64]Migrate{
		1: {UP: func(_ Datasource) error { return nil }},
	}

	createMigrations := `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`
	createLocks := `CREATE TABLE IF NOT EXISTS gofr_migration_locks (
    lock_key VARCHAR(64) PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL
);`

	// 1. checkAndCreateMigrationTable
	mocks.SQL.ExpectExec(createMigrations).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec(createLocks).WillReturnResult(sqlmock.NewResult(0, 0))

	// 2. pre-check getLastMigration returns 0 (migration pending)
	mocks.SQL.ExpectQuery("SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;").
		WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(0))

	// 3. lock succeeds
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. re-fetch getLastMigration under lock returns 1 (another pod already ran it)
	mocks.SQL.ExpectQuery("SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;").
		WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(1))

	// 5. unlock in defer (no migration was executed)
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	// Expectations for logger
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	// This is the key assertion: "no new migrations to run (verified under lock)" should be logged
	mockLogger.EXPECT().Info("no new migrations to run (verified under lock)").Times(1)

	Run(migrationMap, mockContainer)
}

func Test_RunMigrations_SkipAlreadyRun(t *testing.T) {
	mockContainer, mocks := container.NewMockContainer(t)

	// Disable other datasources
	mockContainer.Redis = nil
	mockContainer.Cassandra = nil
	mockContainer.Clickhouse = nil
	mockContainer.Mongo = nil
	mockContainer.ArangoDB = nil
	mockContainer.Elasticsearch = nil
	mockContainer.Oracle = nil
	mockContainer.PubSub = nil
	mockContainer.DGraph = nil
	mockContainer.SurrealDB = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil

	ctrl := gomock.NewController(t)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	migrationMap := map[int64]Migrate{
		1: {UP: func(_ Datasource) error { return nil }},
	}

	createMigrations := `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`
	createLocks := `CREATE TABLE IF NOT EXISTS gofr_migration_locks (
    lock_key VARCHAR(64) PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL
);`

	// 1. checkAndCreateMigrationTable
	mocks.SQL.ExpectExec(createMigrations).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec(createLocks).WillReturnResult(sqlmock.NewResult(0, 0))

	// 2. getLastMigration returns 1
	mocks.SQL.ExpectQuery("SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;").WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(1))

	// Expectations for logger
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()

	Run(migrationMap, mockContainer)
}

// TestEndToEnd_OnlySQLUsed_RedisNotRecorded tests the full migration run flow
// where a migration only uses SQL — Redis should NOT get a migration record.
func TestEndToEnd_OnlySQLUsed_RedisNotRecorded(t *testing.T) {
	mockContainer, mocks := container.NewMockContainer(t)

	// Disable all datasources except SQL and Redis
	mockContainer.Cassandra = nil
	mockContainer.Clickhouse = nil
	mockContainer.Mongo = nil
	mockContainer.ArangoDB = nil
	mockContainer.Elasticsearch = nil
	mockContainer.Oracle = nil
	mockContainer.PubSub = nil
	mockContainer.DGraph = nil
	mockContainer.SurrealDB = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil

	// Use a real miniredis for Redis transactions (needed by redisMigrator)
	s, _ := miniredis.Run()
	defer s.Close()

	realRedisClient := goRedis.NewClient(&goRedis.Options{Addr: s.Addr()})

	// Mock Redis is used for container (has HealthCheck), but real client for transactions
	mocks.Redis.EXPECT().HGetAll(gomock.Any(), "gofr_migrations").
		Return(goRedis.NewMapStringStringResult(map[string]string{}, nil)).Times(2)
	mocks.Redis.EXPECT().TxPipeline().Return(realRedisClient.TxPipeline())

	// Redis lock/unlock expectations
	mocks.Redis.EXPECT().SetNX(gomock.Any(), lockKey, gomock.Any(), defaultLockTTL).
		Return(goRedis.NewBoolResult(true, nil))
	mocks.Redis.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{lockKey}, gomock.Any(), int(defaultLockTTL.Seconds())).
		Return(goRedis.NewCmdResult(int64(1), nil)).AnyTimes()
	mocks.Redis.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{lockKey}, gomock.Any()).
		Return(goRedis.NewCmdResult(int64(1), nil))

	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)

	migrationMap := map[int64]Migrate{
		1: {UP: func(d Datasource) error {
			// Only use SQL, NOT Redis
			_, err := d.SQL.Exec("CREATE TABLE test (id INT)")
			return err
		}},
	}

	// 1. checkAndCreateMigrationTable
	mocks.SQL.ExpectExec(createSQLGoFrMigrationsTable).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec(createSQLGoFrMigrationLocksTable).WillReturnResult(sqlmock.NewResult(0, 0))

	// 2. getLastMigration (pre-check)
	mocks.SQL.ExpectQuery(getLastSQLGoFrMigration).WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(0))

	// 3. lock (SQL side)
	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. getLastMigration under lock
	mocks.SQL.ExpectQuery(getLastSQLGoFrMigration).WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(0))

	// 5. beginTransaction
	mocks.SQL.ExpectBegin()

	// 6. The migration UP function calls SQL.Exec
	mocks.SQL.ExpectExec("CREATE TABLE test (id INT)").WillReturnResult(sqlmock.NewResult(0, 0))

	// 7. commitMigration — SQL was used so INSERT into gofr_migrations should happen
	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);").
		WithArgs(int64(1), "UP", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mocks.SQL.ExpectCommit()

	// 8. unlock (SQL side)
	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	Run(migrationMap, mockContainer)

	// Verify: Redis should NOT have a migration record since the migration only used SQL
	val := s.HGet("gofr_migrations", "1")
	assert.Empty(t, val, "Redis should NOT have migration record when migration only used SQL")

	// Verify all SQL expectations were met
	err := mocks.SQL.ExpectationsWereMet()
	assert.NoError(t, err, "all SQL expectations should be met")
}

// TestEndToEnd_ClickhouseOnly_MigrationUsesClickhouse tests the flow with only Clickhouse
// connected, where the migration uses Clickhouse — only Clickhouse should get a record.
func TestEndToEnd_ClickhouseOnly_MigrationUsesClickhouse(t *testing.T) {
	mockClickHouse := NewMockClickhouse(gomock.NewController(t))

	mockContainer, _ := container.NewMockContainer(t)
	mockContainer.SQL = nil
	mockContainer.Redis = nil
	mockContainer.Mongo = nil
	mockContainer.Cassandra = nil
	mockContainer.PubSub = nil
	mockContainer.ArangoDB = nil
	mockContainer.SurrealDB = nil
	mockContainer.DGraph = nil
	mockContainer.Elasticsearch = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil
	mockContainer.Oracle = nil
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)
	mockContainer.Clickhouse = mockClickHouse

	migrationMap := map[int64]Migrate{
		1: {UP: func(d Datasource) error {
			return d.Clickhouse.Exec(t.Context(), "CREATE TABLE metrics (id UInt64) ENGINE = MergeTree()")
		}},
	}

	// Pre-check
	mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
	mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil).Times(2)

	// Migration UP
	mockClickHouse.EXPECT().Exec(gomock.Any(), "CREATE TABLE metrics (id UInt64) ENGINE = MergeTree()").Return(nil)

	// commitMigration: Clickhouse was used, so INSERT should happen
	mockClickHouse.EXPECT().Exec(gomock.Any(), insertChGoFrMigrationRow, int64(1),
		"UP", gomock.Any(), gomock.Any()).Return(nil)

	Run(migrationMap, mockContainer)
}

// TestEndToEnd_ClickhouseOnly_MigrationDoesNotUseClickhouse verifies that
// when a migration does NOT use the connected Clickhouse, no record is written.
func TestEndToEnd_ClickhouseOnly_MigrationDoesNotUseClickhouse(t *testing.T) {
	mockClickHouse := NewMockClickhouse(gomock.NewController(t))

	mockContainer, _ := container.NewMockContainer(t)
	mockContainer.SQL = nil
	mockContainer.Redis = nil
	mockContainer.Mongo = nil
	mockContainer.Cassandra = nil
	mockContainer.PubSub = nil
	mockContainer.ArangoDB = nil
	mockContainer.SurrealDB = nil
	mockContainer.DGraph = nil
	mockContainer.Elasticsearch = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil
	mockContainer.Oracle = nil
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)
	mockContainer.Clickhouse = mockClickHouse

	migrationMap := map[int64]Migrate{
		1: {UP: func(_ Datasource) error {
			// Does NOT use any datasource
			return nil
		}},
	}

	// Pre-check
	mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
	mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil).Times(2)

	// commitMigration: Clickhouse was NOT used, so INSERT should NOT happen

	Run(migrationMap, mockContainer)

	assert.True(t, mockClickHouse.ctrl.Satisfied(), "no unexpected Clickhouse calls should have been made")
}

// TestEndToEnd_MultiDB_OnlySQLUsed verifies that when SQL+Clickhouse are connected
// but only SQL is used, only SQL gets the migration record.
func TestEndToEnd_MultiDB_OnlySQLUsed(t *testing.T) {
	mockClickHouse := NewMockClickhouse(gomock.NewController(t))

	mockContainer, mocks := container.NewMockContainer(t)
	mockContainer.Redis = nil
	mockContainer.Mongo = nil
	mockContainer.Cassandra = nil
	mockContainer.PubSub = nil
	mockContainer.ArangoDB = nil
	mockContainer.SurrealDB = nil
	mockContainer.DGraph = nil
	mockContainer.Elasticsearch = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil
	mockContainer.Oracle = nil
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)
	mockContainer.Clickhouse = mockClickHouse

	migrationMap := map[int64]Migrate{
		1: {UP: func(d Datasource) error {
			_, err := d.SQL.Exec("CREATE TABLE users (id INT)")
			return err
		}},
	}

	// checkAndCreateMigrationTable (SQL + Clickhouse)
	mocks.SQL.ExpectExec(createSQLGoFrMigrationsTable).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec(createSQLGoFrMigrationLocksTable).WillReturnResult(sqlmock.NewResult(0, 0))
	mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)

	// getLastMigration (pre-check) — both DBs queried
	mocks.SQL.ExpectQuery(getLastSQLGoFrMigration).WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(0))
	mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)

	// lock (SQL only — Clickhouse delegates to base)
	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// getLastMigration (under lock)
	mocks.SQL.ExpectQuery(getLastSQLGoFrMigration).WillReturnRows(sqlmock.NewRows([]string{"MAX"}).AddRow(0))
	mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)

	// beginTransaction
	mocks.SQL.ExpectBegin()

	// Migration UP — uses SQL only
	mocks.SQL.ExpectExec("CREATE TABLE users (id INT)").WillReturnResult(sqlmock.NewResult(0, 0))

	// commitMigration — SQL was used so INSERT happens, Clickhouse was NOT used so no INSERT
	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);").
		WithArgs(int64(1), "UP", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mocks.SQL.ExpectCommit()

	// unlock
	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?").
		WithArgs("gofr_migrations_lock", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	Run(migrationMap, mockContainer)

	err := mocks.SQL.ExpectationsWereMet()
	require.NoError(t, err)
	assert.True(t, mockClickHouse.ctrl.Satisfied(), "Clickhouse should not have received INSERT for migration record")
}
