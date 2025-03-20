package migration

import (
	"context"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func surrealSetup(t *testing.T) (migrator, *container.MockSurrealDB, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	mockSurreal := mocks.SurrealDB

	ds := Datasource{SurrealDB: mockSurreal}

	surrealDB := surrealDS{client: mockSurreal}
	migratorWithSurreal := surrealDB.apply(&ds)

	mockContainer.SurrealDB = mockSurreal

	return migratorWithSurreal, mockSurreal, mockContainer
}

func Test_SurrealCheckAndCreateMigrationTable(t *testing.T) {
	migratorWithSurreal, mockSurreal, mockContainer := surrealSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"table already exists", nil},
	}

	for i, tc := range testCases {
		mockSurreal.EXPECT().Query(context.Background(), checkAndCreateSurrealDBMigrationTable, nil).Return([]any{}, tc.err)

		err := migratorWithSurreal.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_SurrealGetLastMigration(t *testing.T) {
	migratorWithSurreal, mockSurreal, mockContainer := surrealSetup(t)

	testCases := []struct {
		desc string
		err  error
		resp int64
	}{
		{"no error", nil, 1},
		{"query failed", context.DeadlineExceeded, 0},
	}

	for i, tc := range testCases {
		mockSurreal.EXPECT().Query(context.Background(), getLastSurrealDBGoFrMigration, nil).Return([]any{
			map[string]any{"version": float64(tc.resp)},
		}, tc.err)

		resp := migratorWithSurreal.getLastMigration(mockContainer)

		assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_SurrealCommitMigration(t *testing.T) {
	migratorWithSurreal, mockSurreal, mockContainer := surrealSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"insert failed", context.DeadlineExceeded},
	}

	timeNow := time.Now()

	td := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
	}

	for i, tc := range testCases {
		bindVars := map[string]any{
			"version":    td.MigrationNumber,
			"method":     "UP",
			"start_time": td.StartTime,
			"duration":   time.Since(td.StartTime).Milliseconds(),
		}

		mockSurreal.EXPECT().Query(context.Background(), insertSurrealDBGoFrMigrationRow, bindVars).Return([]any{}, tc.err)

		err := migratorWithSurreal.commitMigration(mockContainer, td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_SurrealBeginTransaction(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migratorWithSurreal, _, mockContainer := surrealSetup(t)
		migratorWithSurreal.beginTransaction(mockContainer)
	})

	assert.Contains(t, logs, "surrealDB migrator begin successfully")
}

func TestSurrealDS_Query(t *testing.T) {
	_, mocks := container.NewMockContainer(t)

	mockSurreal := mocks.SurrealDB

	ds := Datasource{SurrealDB: mockSurreal}

	query := "SELECT * FROM table"
	vars := map[string]any{"key": "value"}
	expectedResult := []any{"result"}
	mockSurreal.EXPECT().Query(mock.Anything, query, vars).Return(expectedResult, nil)

	result, err := ds.SurrealDB.Query(context.Background(), query, vars)

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
}
