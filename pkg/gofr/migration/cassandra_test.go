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

func cassandraSetup(t *testing.T) (migrator, *container.MockCassandra, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	mockCassandra := mocks.Cassandra

	ds := Datasource{Cassandra: mockContainer.Cassandra}

	ch := cassandraDS{Cassandra: mockCassandra}
	mg := ch.apply(&ds)

	mockContainer.Cassandra = mockCassandra

	return mg, mockCassandra, mockContainer
}

func Test_CassandraCheckAndCreateMigrationTable(t *testing.T) {
	mg, mockCassandra, mockContainer := cassandraSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", sql.ErrConnDone},
	}

	for i, tc := range testCases {
		mockCassandra.EXPECT().Exec(CheckAndCreateCassandraMigrationTable).Return(tc.err)

		err := mg.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_CassandraGetLastMigration(t *testing.T) {
	mg, mockCassandra, mockContainer := cassandraSetup(t)

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
		mockCassandra.EXPECT().Query(&lastMigration, getLastCassandraGoFrMigration).Return(tc.err)

		resp := mg.getLastMigration(mockContainer)

		assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_CassandraCommitMigration(t *testing.T) {
	mg, mockCassandra, mockContainer := cassandraSetup(t)

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
		mockCassandra.EXPECT().Exec(insertCassandraGoFrMigrationRow, td.MigrationNumber,
			"UP", td.StartTime, gomock.Any()).Return(tc.err)

		err := mg.commitMigration(mockContainer, td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_CassandraBeginTransaction(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		mg, _, mockContainer := cassandraSetup(t)
		mg.beginTransaction(mockContainer)
	})

	assert.Contains(t, logs, "Cassandra Migrator begin successfully")
}
