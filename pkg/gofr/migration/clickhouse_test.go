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

func clickHouseSetup(t *testing.T) (migrator, *MockClickhouse, *container.Container) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mockContainer, _ := container.NewMockContainer(t)

	mockClickhouse := NewMockClickhouse(ctrl)

	ds := Datasource{Clickhouse: mockClickhouse}

	ch := clickHouseDS{Clickhouse: mockClickhouse}
	mg := ch.apply(ds)

	mockContainer.Clickhouse = mockClickhouse

	return mg, mockClickhouse, mockContainer
}

func Test_ClickHouseCheckAndCreateMigrationTable(t *testing.T) {
	mg, mockClickhouse, mockContainer := clickHouseSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", sql.ErrConnDone},
	}

	for i, tc := range testCases {
		mockClickhouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(tc.err)

		err := mg.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ClickHouseGetLastMigration(t *testing.T) {
	mg, mockClickhouse, mockContainer := clickHouseSetup(t)

	testCases := []struct {
		desc string
		err  error
		resp int64
	}{
		{"no error", nil, 0},
		{"connection failed", sql.ErrConnDone, 0},
	}

	for i, tc := range testCases {
		mockClickhouse.EXPECT().Select(gomock.Any(), gomock.Any(), getLastChGoFrMigration).Return(tc.err)

		resp := mg.getLastMigration(mockContainer)

		assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ClickHouseCommitMigration(t *testing.T) {
	mg, mockClickhouse, mockContainer := clickHouseSetup(t)

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
		mockClickhouse.EXPECT().Exec(gomock.Any(), insertChGoFrMigrationRow, td.MigrationNumber,
			"UP", td.StartTime, gomock.Any()).Return(tc.err)

		err := mg.commitMigration(mockContainer, td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ClickHouseBeginTransaction(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		mg, _, mockContainer := clickHouseSetup(t)
		mg.beginTransaction(mockContainer)
	})

	assert.Contains(t, logs, "Clickhouse Migrator begin successfully")
}

func Test_ClickHouseRollback(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		mg, _, mockContainer := clickHouseSetup(t)
		mg.rollback(mockContainer, transactionData{MigrationNumber: 0})
	})

	assert.Contains(t, logs, "Migration 0 failed")
}
