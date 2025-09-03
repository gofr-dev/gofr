package migration

import (
	"bytes"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

func oracleSetup(t *testing.T) (migrator, *container.MockOracleDB, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	mockOracle := mocks.Oracle

	ds := Datasource{Oracle: mockOracle}

	oracleDB := oracleDS{Oracle: mockOracle}
	migrationWithOracle := oracleDB.apply(&ds)

	mockContainer.Oracle = mockOracle

	return migrationWithOracle, mockOracle, mockContainer
}

func Test_OracleCheckAndCreateMigrationTable(t *testing.T) {
	mg, mockOracle, mockContainer := oracleSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", sql.ErrConnDone},
	}

	for i, tc := range testCases {
		mockOracle.EXPECT().Exec(gomock.Any(), checkAndCreateOracleMigrationTable).Return(tc.err)
		err := mg.checkAndCreateMigrationTable(mockContainer)
		assert.Equal(t, tc.err, err, "TEST[%d]: %s failed", i, tc.desc)
	}
}

func Test_OracleGetLastMigration(t *testing.T) {
	mg, mockOracle, mockContainer := oracleSetup(t)

	testCases := []struct {
		desc string
		err  error
		resp int64
	}{
		{"no error", nil, 0},
		{"connection failed", sql.ErrConnDone, 0},
	}

	for i, tc := range testCases {
		mockOracle.EXPECT().Select(gomock.Any(), gomock.Any(), getLastOracleGoFrMigration).Return(tc.err)

		resp := mg.getLastMigration(mockContainer)
		assert.Equal(t, tc.resp, resp, "TEST[%d]: %s failed", i, tc.desc)
	}
}

func Test_OracleCommitMigration(t *testing.T) {
	mg, mockOracle, mockContainer := oracleSetup(t)
	timeNow := time.Now()
	td := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
	}
	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", sql.ErrConnDone},
	}

	for i, tc := range testCases {
		mockOracle.EXPECT().Exec(gomock.Any(), insertOracleGoFrMigrationRow,
			td.MigrationNumber, "UP", td.StartTime, gomock.Any()).Return(tc.err)

		err := mg.commitMigration(mockContainer, td)
		assert.Equal(t, tc.err, err, "TEST[%d]: %s failed", i, tc.desc)
	}
}

func Test_OracleBeginTransaction(t *testing.T) {
	logs := captureStdout(func() {
		mg, _, mockContainer := oracleSetup(t)
		mg.beginTransaction(mockContainer)
	})
	assert.Contains(t, logs, "OracleDB Migrator begin successfully")
}

// captureStdout helper to capture stdout during function execution.
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		outC <- buf.String()
	}()

	f()

	_ = w.Close()
	os.Stdout = old
	out := <-outC

	return out
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

	mockOracle.EXPECT().Exec(gomock.Any(), checkAndCreateOracleMigrationTable).Return(nil)
	mockOracle.EXPECT().Select(gomock.Any(), gomock.Any(), getLastOracleGoFrMigration).Return(nil)
	mockOracle.EXPECT().Exec(gomock.Any(), "CREATE TABLE test (id INT)").Return(nil)
	mockOracle.EXPECT().Exec(gomock.Any(), insertOracleGoFrMigrationRow, int64(1), "UP", gomock.Any(), gomock.Any()).Return(nil)

	Run(migrationMap, mockContainer)
}

func TestOracleMigration_FailCreateMigrationTable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockContainer, mocks := container.NewMockContainer(t)
	mockOracle := mocks.Oracle
	mockContainer.Oracle = mockOracle

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	mg := od.apply(&ds)

	mockOracle.EXPECT().Exec(gomock.Any(), checkAndCreateOracleMigrationTable).Return(sql.ErrConnDone)

	err := mg.checkAndCreateMigrationTable(mockContainer)
	assert.Equal(t, sql.ErrConnDone, err)
}

func TestOracleMigration_GetLastMigration_ReturnsZeroOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockContainer, mocks := container.NewMockContainer(t)
	mockOracle := mocks.Oracle
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

	mockContainer, mocks := container.NewMockContainer(t)
	mockOracle := mocks.Oracle
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

	mockContainer, mocks := container.NewMockContainer(t)
	mockOracle := mocks.Oracle
	mockContainer.Logger = logging.NewLogger(logging.DEBUG)
	mockContainer.Oracle = mockOracle

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	mg := od.apply(&ds)

	// Capture logs or just call method and rely on it not panicking.
	mg.beginTransaction(mockContainer)
}

func initializeOracleRunMocks(t *testing.T) (*container.MockOracleDB, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)
	mockOracle := mocks.Oracle

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
