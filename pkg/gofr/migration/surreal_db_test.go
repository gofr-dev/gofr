package migration

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

var errSurrealInner = errors.New("surreal inner migrator error")

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

func Test_SurrealGetAppliedMigrations(t *testing.T) {
	testCases := []struct {
		desc        string
		result      []any
		queryErr    error
		baseApplied map[int64]struct{}
		baseErr     error
		expApplied  map[int64]struct{}
		expErr      error
	}{
		{
			desc: "surreal versions are unioned with inner migrator versions",
			result: []any{
				map[string]any{"version": int64(1)},
				map[string]any{"version": int64(3)},
				map[string]any{"version": int64(4)},
			},
			baseApplied: map[int64]struct{}{2: {}},
			expApplied:  map[int64]struct{}{1: {}, 2: {}, 3: {}, 4: {}},
		},
		// The SurrealDB driver decodes a `number` column as int for real connections,
		// but may yield int64/float64 depending on value and driver version.
		{
			desc: "version types are normalized",
			result: []any{
				map[string]any{"version": int(2147483600)},
				map[string]any{"version": int64(20260717211450)},
				map[string]any{"version": uint64(20260717211451)},
				map[string]any{"version": float64(1)},
			},
			baseApplied: map[int64]struct{}{},
			expApplied:  map[int64]struct{}{2147483600: {}, 20260717211450: {}, 20260717211451: {}, 1: {}},
		},
		{
			desc:        "non map row is ignored",
			result:      []any{"bad"},
			baseApplied: map[int64]struct{}{3: {}},
			expApplied:  map[int64]struct{}{3: {}},
		},
		{
			desc:        "empty result returns only inner versions",
			result:      []any{},
			baseApplied: map[int64]struct{}{5: {}},
			expApplied:  map[int64]struct{}{5: {}},
		},
		{desc: "surreal query error", queryErr: context.DeadlineExceeded, expErr: context.DeadlineExceeded},
		{
			desc:    "inner migrator error",
			result:  []any{map[string]any{"version": int64(2)}},
			baseErr: errSurrealInner,
			expErr:  errSurrealInner,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockContainer, mocks := container.NewMockContainer(t)
			mockMigrator := NewMockmigrator(ctrl)

			m := surrealMigrator{SurrealDB: surrealDS{client: mocks.SurrealDB}, migrator: mockMigrator}

			mocks.SurrealDB.EXPECT().Query(gomock.Any(), getSurrealDBGoFrMigrationVersions, nil).Return(tc.result, tc.queryErr)

			if tc.queryErr == nil {
				mockMigrator.EXPECT().getAppliedMigrations(mockContainer).Return(tc.baseApplied, tc.baseErr)
			}

			applied, err := m.getAppliedMigrations(mockContainer)

			assert.Equal(t, tc.expApplied, applied)

			if tc.expErr != nil {
				require.ErrorIs(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Test_SurrealMigrationTableQueriesAreIdempotent guards the fix for #3725: every DEFINE
// statement used to create the migration table must use "IF NOT EXISTS" so that
// re-running an app against an existing gofr_migrations table does not fail.
func Test_SurrealMigrationTableQueriesAreIdempotent(t *testing.T) {
	for _, q := range getMigrationTableQueries() {
		assert.Contains(t, q, "IF NOT EXISTS", "migration query must be idempotent: %q", q)
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
		UsedDatasources: map[string]bool{dsSurrealDB: true},
	}

	for i, tc := range testCases {
		bindVars := map[string]any{
			"version":    td.MigrationNumber,
			"method":     "UP",
			"start_time": td.StartTime,
			"duration":   time.Since(td.StartTime).Milliseconds(),
		}

		mockSurreal.EXPECT().Query(gomock.Any(), insertSurrealDBGoFrMigrationRow, bindVars).Return([]any{}, tc.err)

		err := migratorWithSurreal.commitMigration(mockContainer, &td)

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

func Test_SurrealCommitMigration_SkipsWhenNotUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockSurreal := NewMockSurrealDB(ctrl)
	mockMigrator := NewMockmigrator(ctrl)

	m := surrealMigrator{SurrealDB: mockSurreal, migrator: mockMigrator}

	c, _ := container.NewMockContainer(t)

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
		UsedDatasources: map[string]bool{},
	}

	mockMigrator.EXPECT().commitMigration(c, &data).Return(nil)

	err := m.commitMigration(c, &data)
	assert.NoError(t, err)
}

func Test_SurrealCheckAndCreateMigrationTable_QueryError(t *testing.T) {
	migratorWithSurreal, mockSurreal, mockContainer := surrealSetup(t)

	mockSurreal.EXPECT().Query(gomock.Any(), getMigrationTableQueries()[0], nil).Return(nil, context.DeadlineExceeded)

	err := migratorWithSurreal.checkAndCreateMigrationTable(mockContainer)

	require.ErrorIs(t, err, errExecuteQuery)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func Test_SurrealVersionToInt64(t *testing.T) {
	testCases := []struct {
		desc    string
		version any
		exp     int64
	}{
		{desc: "uint64 overflowing int64", version: uint64(math.MaxInt64) + 1, exp: 0},
		{desc: "unsupported type", version: "12", exp: 0},
		{desc: "nil value", version: nil, exp: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.exp, surrealVersionToInt64(tc.version))
		})
	}
}

func Test_SurrealMigratorDelegation(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockContainer, _ := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	m := surrealMigrator{migrator: mockMigrator}
	data := transactionData{MigrationNumber: 4}

	mockMigrator.EXPECT().rollback(mockContainer, &data)
	mockLogger.EXPECT().Fatalf("migration %v failed and rolled back", int64(4))
	mockMigrator.EXPECT().lock(gomock.Any(), gomock.Any(), mockContainer, "owner-1").Return(context.Canceled)
	mockMigrator.EXPECT().unlock(mockContainer, "owner-1").Return(context.Canceled)

	m.rollback(mockContainer, &data)

	require.ErrorIs(t, m.lock(t.Context(), func() {}, mockContainer, "owner-1"), context.Canceled)
	require.ErrorIs(t, m.unlock(mockContainer, "owner-1"), context.Canceled)
	assert.Equal(t, "SurrealDB", m.name())
}
