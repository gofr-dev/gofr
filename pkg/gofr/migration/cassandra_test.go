package migration

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func cassandraSetup(t *testing.T) (migrator, *container.MockCassandraWithContext, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	mockCassandra := mocks.Cassandra

	ds := Datasource{Cassandra: mockContainer.Cassandra}

	cassandraDB := cassandraDS{CassandraWithContext: mockCassandra}
	migratorWithCassandra := cassandraDB.apply(&ds)

	mockContainer.Cassandra = mockCassandra

	return migratorWithCassandra, mockCassandra, mockContainer
}

func Test_CassandraCheckAndCreateMigrationTable(t *testing.T) {
	migratorWithCassandra, mockCassandra, mockContainer := cassandraSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", sql.ErrConnDone},
	}

	for i, tc := range testCases {
		mockCassandra.EXPECT().ExecWithCtx(gomock.Any(), checkAndCreateCassandraMigrationTable).Return(tc.err)

		err := migratorWithCassandra.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_CassandraGetLastMigration(t *testing.T) {
	migratorWithCassandra, mockCassandra, mockContainer := cassandraSetup(t)

	testCases := []struct {
		desc string
		err  error
		resp int64
	}{
		{"no error", nil, 0},
		{"connection failed", sql.ErrConnDone, 0},
	}

	var lastMigration []int64

	for i, tc := range testCases {
		mockCassandra.EXPECT().QueryWithCtx(gomock.Any(), &lastMigration, getLastCassandraGoFrMigration).Return(tc.err)

		resp := migratorWithCassandra.getLastMigration(mockContainer)

		assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_CassandraCommitMigration(t *testing.T) {
	migratorWithCassandra, mockCassandra, mockContainer := cassandraSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", sql.ErrConnDone},
	}

	timeNow := time.Now()

	td := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
	}

	for i, tc := range testCases {
		mockCassandra.EXPECT().ExecWithCtx(gomock.Any(), insertCassandraGoFrMigrationRow, td.MigrationNumber,
			"UP", td.StartTime, gomock.Any()).Return(tc.err)

		err := migratorWithCassandra.commitMigration(mockContainer, td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_CassandraBeginTransaction(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migratorWithCassandra, _, mockContainer := cassandraSetup(t)
		migratorWithCassandra.beginTransaction(mockContainer)
	})

	assert.Contains(t, logs, "cassandra migrator begin successfully")
}
