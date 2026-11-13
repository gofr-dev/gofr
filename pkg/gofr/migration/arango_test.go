package migration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

var errArango = errors.New("arango error")

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func arangoSetup(t *testing.T) (migrator, *container.MockArangoDBProvider, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	mockArango := mocks.ArangoDB

	ds := Datasource{ArangoDB: mockContainer.ArangoDB}

	arangoDB := arangoDS{client: mockArango}
	migratorWithArango := arangoDB.apply(&ds)

	mockContainer.ArangoDB = mockArango

	return migratorWithArango, mockArango, mockContainer
}

func Test_ArangoCheckAndCreateMigrationTable(t *testing.T) {
	migratorWithArango, mockArango, mockContainer := arangoSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"collection already exists", nil},
	}

	for i, tc := range testCases {
		mockArango.EXPECT().CreateCollection(gomock.Any(), arangoMigrationDB, arangoMigrationCollection, false).Return(tc.err)

		err := migratorWithArango.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ArangoGetAppliedMigrations(t *testing.T) {
	testCases := []struct {
		desc        string
		versions    []int64
		queryErr    error
		baseApplied map[int64]struct{}
		baseErr     error
		expApplied  map[int64]struct{}
		expErr      error
	}{
		{
			desc:        "arango versions are unioned with inner migrator versions",
			versions:    []int64{1, 3, 4},
			baseApplied: map[int64]struct{}{2: {}},
			expApplied:  map[int64]struct{}{1: {}, 2: {}, 3: {}, 4: {}},
		},
		{
			desc:        "empty collection returns only inner versions",
			versions:    []int64{},
			baseApplied: map[int64]struct{}{5: {}},
			expApplied:  map[int64]struct{}{5: {}},
		},
		{desc: "arango query error", queryErr: context.DeadlineExceeded, expErr: context.DeadlineExceeded},
		{desc: "inner migrator error", versions: []int64{2}, baseErr: errArango, expErr: errArango},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockContainer, mocks := container.NewMockContainer(t)
			mockMigrator := NewMockmigrator(ctrl)

			m := arangoMigrator{ArangoDB: arangoDS{client: mocks.ArangoDB}, migrator: mockMigrator}

			mocks.ArangoDB.EXPECT().Query(gomock.Any(), arangoMigrationDB, getArangoMigrationVersions, nil, gomock.Any()).
				DoAndReturn(func(_ context.Context, _, _ string, _ map[string]any, result any, _ ...map[string]any) error {
					if tc.queryErr != nil {
						return tc.queryErr
					}

					*(result.(*[]int64)) = tc.versions

					return nil
				})

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

func Test_ArangoCommitMigration(t *testing.T) {
	migratorWithArango, mockArango, mockContainer := arangoSetup(t)

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
		UsedDatasources: map[string]bool{dsArangoDB: true},
	}

	for i, tc := range testCases {
		bindVars := map[string]any{
			"version":    td.MigrationNumber,
			"method":     "UP",
			"start_time": td.StartTime,
			"duration":   time.Since(td.StartTime).Milliseconds(),
		}

		mockArango.EXPECT().Query(gomock.Any(), arangoMigrationDB, insertArangoMigrationRecord, bindVars, gomock.Any()).Return(tc.err)

		err := migratorWithArango.commitMigration(mockContainer, &td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ArangoBeginTransaction(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migratorWithArango, _, mockContainer := arangoSetup(t)
		migratorWithArango.beginTransaction(mockContainer)
	})

	assert.Contains(t, logs, "ArangoDB migrator begin successfully")
}

func Test_ArangoDSDelegatesToClient(t *testing.T) {
	testCases := []struct {
		desc     string
		mockCall func(m *container.MockArangoDBProvider)
		call     func(ds arangoDS) error
		expErr   error
	}{
		{
			desc: "CreateDB",
			mockCall: func(m *container.MockArangoDBProvider) {
				m.EXPECT().CreateDB(gomock.Any(), "db").Return(errArango)
			},
			call:   func(ds arangoDS) error { return ds.CreateDB(t.Context(), "db") },
			expErr: errArango,
		},
		{
			desc: "DropDB",
			mockCall: func(m *container.MockArangoDBProvider) {
				m.EXPECT().DropDB(gomock.Any(), "db").Return(nil)
			},
			call: func(ds arangoDS) error { return ds.DropDB(t.Context(), "db") },
		},
		{
			desc: "DropCollection",
			mockCall: func(m *container.MockArangoDBProvider) {
				m.EXPECT().DropCollection(gomock.Any(), "db", "coll").Return(errArango)
			},
			call:   func(ds arangoDS) error { return ds.DropCollection(t.Context(), "db", "coll") },
			expErr: errArango,
		},
		{
			desc: "CreateGraph",
			mockCall: func(m *container.MockArangoDBProvider) {
				m.EXPECT().CreateGraph(gomock.Any(), "db", "graph", "edges").Return(nil)
			},
			call: func(ds arangoDS) error { return ds.CreateGraph(t.Context(), "db", "graph", "edges") },
		},
		{
			desc: "DropGraph",
			mockCall: func(m *container.MockArangoDBProvider) {
				m.EXPECT().DropGraph(gomock.Any(), "db", "graph").Return(errArango)
			},
			call:   func(ds arangoDS) error { return ds.DropGraph(t.Context(), "db", "graph") },
			expErr: errArango,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			_, mocks := container.NewMockContainer(t)

			tc.mockCall(mocks.ArangoDB)

			err := tc.call(arangoDS{client: mocks.ArangoDB})

			assert.Equal(t, tc.expErr, err)
		})
	}
}

func Test_ArangoCheckAndCreateMigrationTable_CollectionError(t *testing.T) {
	testCases := []struct {
		desc      string
		createErr error
		expLog    string
	}{
		{
			desc:      "collection creation error is logged and ignored",
			createErr: errArango,
			expLog:    "Migration collection might already exist:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var err error

			logs := testutil.StdoutOutputForFunc(func() {
				migratorWithArango, mockArango, mockContainer := arangoSetup(t)

				mockArango.EXPECT().CreateCollection(gomock.Any(), arangoMigrationDB, arangoMigrationCollection, false).
					Return(tc.createErr)

				err = migratorWithArango.checkAndCreateMigrationTable(mockContainer)
			})

			require.NoError(t, err)
			assert.Contains(t, logs, tc.expLog)
		})
	}
}

func Test_ArangoMigratorDelegation(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockContainer, _ := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	m := arangoMigrator{migrator: mockMigrator}
	data := transactionData{MigrationNumber: 4}

	mockMigrator.EXPECT().rollback(mockContainer, &data)
	mockLogger.EXPECT().Fatalf("Migration %v failed and rolled back", int64(4))
	mockMigrator.EXPECT().lock(gomock.Any(), gomock.Any(), mockContainer, "owner-1").Return(errArango)
	mockMigrator.EXPECT().unlock(mockContainer, "owner-1").Return(errArango)

	m.rollback(mockContainer, &data)

	require.ErrorIs(t, m.lock(t.Context(), func() {}, mockContainer, "owner-1"), errArango)
	require.ErrorIs(t, m.unlock(mockContainer, "owner-1"), errArango)
	assert.Equal(t, "ArangoDB", m.name())
}
