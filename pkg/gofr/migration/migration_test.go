package migration

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
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
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)

		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
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
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)

		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
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

	// checkAndCreateMigrationTable is called first
	mockClickHouse.EXPECT().Exec(gomock.Any(), gomock.Any()).Return(nil)

	// Then getLastMigration is called - returns 0, so migration 0 is skipped
	mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).DoAndReturn(
		func(_, dest, _ any, _ ...any) error {
			v := reflect.ValueOf(dest).Elem()
			// Create a new slice of the same type as the destination
			slice := reflect.MakeSlice(v.Type(), 1, 1)
			// Set the first element's Timestamp field to 0
			slice.Index(0).FieldByName("Timestamp").SetInt(0)
			v.Set(slice)

			return nil
		})

	Run(migrationMap, mockContainer)
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
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)

		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
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

	// 4. beginTransaction
	mocks.SQL.ExpectBegin()

	// 5. commitMigration fails
	testErr := errGenericCommit

	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mocks.SQL.ExpectCommit().WillReturnError(testErr)

	// 6. rollback in runMigrations
	mocks.SQL.ExpectRollback()

	// 7. unlock in defer
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
	mockLogger.EXPECT().Infof(gomock.Any()).AnyTimes()

	Run(migrationMap, mockContainer)
}
