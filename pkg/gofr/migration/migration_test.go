package migration

import (
	"database/sql"
	"testing"
	"time"

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
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)
	mockContainer.Clickhouse = mockClickHouse

	return mockClickHouse, mockContainer
}

func TestOracleMigration_RunMigrationSuccess(t *testing.T) {
	mockOracle, mockContainer := initializeOracleRunMocks(t)

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	_ = od.apply(&ds) // apply migrator wrapper.

	migrationMap := map[int64]Migrate{
		1: {UP: func(d Datasource) error {
			return d.Oracle.Exec(t.Context(), "CREATE TABLE test (id INT)")
		}},
	}

	mockOracle.EXPECT().Exec(gomock.Any(), CheckAndCreateOracleMigrationTable).Return(nil)
	mockOracle.EXPECT().Select(gomock.Any(), gomock.Any(), getLastOracleGoFrMigration).Return(nil)
	mockOracle.EXPECT().Exec(gomock.Any(), "CREATE TABLE test (id INT)").Return(nil)
	mockOracle.EXPECT().Exec(gomock.Any(), insertOracleGoFrMigrationRow, int64(1), "UP", gomock.Any(), gomock.Any()).Return(nil)

	Run(migrationMap, mockContainer)
}

func TestOracleMigration_FailCreateMigrationTable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOracle := NewMockOracle(ctrl)
	mockContainer, _ := container.NewMockContainer(t)
	mockContainer.Oracle = mockOracle

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	mg := od.apply(&ds)

	mockOracle.EXPECT().Exec(gomock.Any(), CheckAndCreateOracleMigrationTable).Return(sql.ErrConnDone)

	err := mg.checkAndCreateMigrationTable(mockContainer)
	assert.Equal(t, sql.ErrConnDone, err)
}

func TestOracleMigration_GetLastMigration_ReturnsZeroOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOracle := NewMockOracle(ctrl)
	mockContainer, _ := container.NewMockContainer(t)
	mockContainer.Oracle = mockOracle

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	mg := od.apply(&ds)

	mockOracle.EXPECT().Select(gomock.Any(), gomock.Any(), getLastOracleGoFrMigration).Return(sql.ErrConnDone)

	lastMigration := mg.getLastMigration(mockContainer)
	assert.Equal(t, int64(0), lastMigration)
}

func TestOracleMigration_CommitMigration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOracle := NewMockOracle(ctrl)
	mockContainer, _ := container.NewMockContainer(t)
	mockContainer.Oracle = mockOracle

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	mg := od.apply(&ds)

	td := transactionData{
		StartTime:       time.Now(),
		MigrationNumber: 42,
	}

	mockOracle.EXPECT().
		Exec(gomock.Any(), insertOracleGoFrMigrationRow,
			td.MigrationNumber, "UP", td.StartTime, gomock.Any()).
		Return(nil)

	err := mg.commitMigration(mockContainer, td)
	assert.NoError(t, err)
}

func TestOracleMigration_BeginTransaction_Logs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOracle := NewMockOracle(ctrl)
	mockContainer, _ := container.NewMockContainer(t)
	mockContainer.Logger = logging.NewLogger(logging.DEBUG)
	mockContainer.Oracle = mockOracle

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	mg := od.apply(&ds)

	// Capture logs or just call method and rely on it not panicking.
	mg.beginTransaction(mockContainer)
}

func initializeOracleRunMocks(t *testing.T) (*MockOracle, *container.Container) {
	t.Helper()

	mockOracle := NewMockOracle(gomock.NewController(t))
	mockContainer, _ := container.NewMockContainer(t)

	// Disable all other datasources by setting to nil.
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
	mockContainer.Clickhouse = nil

	// Initialize Oracle mock and Logger.
	mockContainer.Oracle = mockOracle
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)

	return mockOracle, mockContainer
}
