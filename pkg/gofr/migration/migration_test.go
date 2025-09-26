package migration

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
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

		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)
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

		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)
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

	assert.Contains(t, logs, "skipping migration 0")
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

		mockClickHouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(nil)
		mockClickHouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(nil)
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
