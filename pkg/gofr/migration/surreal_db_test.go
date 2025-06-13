package migration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

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
		mockSurreal.EXPECT().Query(gomock.Any(), gomock.Any(), nil).Return([]any{}, tc.err).MaxTimes(8)

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
		mockSurreal.EXPECT().Query(gomock.Any(), getLastSurrealDBGoFrMigration, nil).Return([]any{
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

		mockSurreal.EXPECT().Query(gomock.Any(), insertSurrealDBGoFrMigrationRow, bindVars).Return([]any{}, tc.err)

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
	_, mockSurreal, _ := surrealSetup(t)

	query := "SELECT * FROM table"
	vars := map[string]any{"key": "value"}
	expectedResult := []any{"result"}
	mockSurreal.EXPECT().Query(t.Context(), query, vars).Return(expectedResult, nil)

	surreal := surrealDS{client: mockSurreal}
	result, err := surreal.Query(t.Context(), query, vars)

	require.NoError(t, err)
	assert.Equal(t, expectedResult, result)
}

func TestSurrealDS_CreateNamespace(t *testing.T) {
	_, mockSurreal, _ := surrealSetup(t)

	namespace := "test_namespace"
	mockSurreal.EXPECT().CreateNamespace(t.Context(), namespace).Return(nil)

	surreal := surrealDS{client: mockSurreal}
	err := surreal.CreateNamespace(t.Context(), namespace)

	assert.NoError(t, err)
}

func TestSurrealDS_CreateDatabase(t *testing.T) {
	_, mockSurreal, _ := surrealSetup(t)

	database := "test_database"
	mockSurreal.EXPECT().CreateDatabase(t.Context(), database).Return(nil)

	surreal := surrealDS{client: mockSurreal}
	err := surreal.CreateDatabase(t.Context(), database)

	assert.NoError(t, err)
}

func TestSurrealDS_DropNamespace(t *testing.T) {
	_, mockSurreal, _ := surrealSetup(t)

	namespace := "test_namespace"
	mockSurreal.EXPECT().DropNamespace(t.Context(), namespace).Return(nil)

	surreal := surrealDS{client: mockSurreal}
	err := surreal.DropNamespace(t.Context(), namespace)

	assert.NoError(t, err)
}

func TestSurrealDS_DropDatabase(t *testing.T) {
	_, mockSurreal, _ := surrealSetup(t)

	database := "test_database"
	mockSurreal.EXPECT().DropDatabase(t.Context(), database).Return(nil)

	surreal := surrealDS{client: mockSurreal}
	err := surreal.DropDatabase(t.Context(), database)

	assert.NoError(t, err)
}
