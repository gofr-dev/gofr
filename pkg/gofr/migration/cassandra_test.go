package migration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{"connection failed", sql.ErrConnDone, -1},
	}

	var lastMigration []int64

	for i, tc := range testCases {
		mockCassandra.EXPECT().QueryWithCtx(gomock.Any(), &lastMigration, getLastCassandraGoFrMigration).Return(tc.err)

		resp, err := migratorWithCassandra.getLastMigration(mockContainer)

		assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)

		if tc.err != nil {
			assert.ErrorContains(t, err, tc.err.Error(), "TEST[%v]\n %v Failed! ", i, tc.desc)
		} else {
			assert.NoError(t, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
		}
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
		UsedDatasources: map[string]bool{dsCassandra: true},
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

func Test_CassandraCommitMigration_SkipsWhenNotUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCass := container.NewMockCassandraWithContext(ctrl)
	mockMigrator := NewMockmigrator(ctrl)
	m := cassandraMigrator{CassandraWithContext: mockCass, migrator: mockMigrator}

	c, _ := container.NewMockContainer(t)

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
		UsedDatasources: map[string]bool{},
	}

	mockMigrator.EXPECT().commitMigration(c, data).Return(nil)

	err := m.commitMigration(c, data)
	assert.NoError(t, err)
}

func Test_CassandraGetLastMigration_Chained(t *testing.T) {
	testCases := []struct {
		desc       string
		versions   []int64
		baseResp   int64
		baseErr    error
		expVersion int64
		expErr     error
	}{
		{
			desc:       "highest cassandra version is picked",
			versions:   []int64{3, 9, 5},
			baseResp:   4,
			expVersion: 9,
		},
		{
			desc:       "base version greater than cassandra",
			versions:   []int64{2},
			baseResp:   6,
			expVersion: 6,
		},
		{
			desc:       "base migrator error",
			versions:   []int64{2},
			baseErr:    sql.ErrConnDone,
			expVersion: -1,
			expErr:     sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockContainer, mocks := container.NewMockContainer(t)
			mockMigrator := NewMockmigrator(ctrl)

			m := cassandraMigrator{CassandraWithContext: mocks.Cassandra, migrator: mockMigrator}

			mocks.Cassandra.EXPECT().QueryWithCtx(gomock.Any(), gomock.Any(), getLastCassandraGoFrMigration).
				DoAndReturn(func(_ context.Context, dest any, _ string, _ ...any) error {
					*(dest.(*[]int64)) = tc.versions

					return nil
				})
			mockMigrator.EXPECT().getLastMigration(mockContainer).Return(tc.baseResp, tc.baseErr)

			resp, err := m.getLastMigration(mockContainer)

			assert.Equal(t, tc.expVersion, resp)
			assert.Equal(t, tc.expErr, err)
		})
	}
}

func Test_CassandraMigratorDelegation(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockContainer, _ := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	m := cassandraMigrator{migrator: mockMigrator}
	data := transactionData{MigrationNumber: 4}

	mockMigrator.EXPECT().rollback(mockContainer, data)
	mockLogger.EXPECT().Fatalf("migration %v failed and rolled back", int64(4))
	mockMigrator.EXPECT().lock(gomock.Any(), gomock.Any(), mockContainer, "owner-1").Return(sql.ErrConnDone)
	mockMigrator.EXPECT().unlock(mockContainer, "owner-1").Return(sql.ErrConnDone)

	m.rollback(mockContainer, data)

	require.ErrorIs(t, m.lock(t.Context(), func() {}, mockContainer, "owner-1"), sql.ErrConnDone)
	require.ErrorIs(t, m.unlock(mockContainer, "owner-1"), sql.ErrConnDone)
	assert.Equal(t, "Cassandra", m.name())
}
