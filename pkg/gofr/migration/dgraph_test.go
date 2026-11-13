//go:build !gofr_nodgraph

package migration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dgraph-io/dgo/v210/protos/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

var errDgraph = errors.New("dgraph error")

// fakeDgraphTxn is a minimal dgraphTxn used to drive commitMigration in tests.
type fakeDgraphTxn struct {
	mutateErr  error
	commitErr  error
	discardErr error
	setJSON    []byte
	mutated    bool
	committed  bool
	discarded  bool
}

func (f *fakeDgraphTxn) Mutate(_ context.Context, mu *api.Mutation) (*api.Response, error) {
	f.mutated = true
	f.setJSON = mu.SetJson

	return nil, f.mutateErr
}

func (f *fakeDgraphTxn) Commit(context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeDgraphTxn) Discard(context.Context) error {
	f.discarded = true
	return f.discardErr
}

func dgraphSetup(t *testing.T) (migrator, *container.MockDgraph, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)
	mockDGraph := mocks.DGraph

	ds := Datasource{DGraph: mockContainer.DGraph}

	dgraphDB := dgraphDS{client: mockDGraph}
	migratorWithDGraph := dgraphDB.apply(&ds)

	mockContainer.DGraph = mockDGraph

	return migratorWithDGraph, mockDGraph, mockContainer
}

func Test_DGraphCheckAndCreateMigrationTable(t *testing.T) {
	migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)

	mockDGraph.EXPECT().ApplySchema(gomock.Any(), dgraphSchema).Return(nil)

	err := migratorWithDGraph.checkAndCreateMigrationTable(mockContainer)

	require.NoError(t, err, "Test_DGraphCheckAndCreateMigrationTable Failed!")
}

func Test_DGraphGetAppliedMigrations(t *testing.T) {
	testCases := []struct {
		desc      string
		mockResp  any
		queryErr  error
		skipInner bool
		inner     map[int64]struct{}
		innerErr  error
		expected  map[int64]struct{}
		expErr    string
	}{
		{
			desc:     "backend versions merged with inner versions",
			mockResp: &api.Response{Json: []byte(`{"migrations":[{"version":1},{"version":3},{"version":4}]}`)},
			inner:    map[int64]struct{}{2: {}},
			expected: map[int64]struct{}{1: {}, 2: {}, 3: {}, 4: {}},
		},
		{
			desc:     "empty response returns only inner set",
			mockResp: &api.Response{Json: []byte(`{"migrations":[]}`)},
			inner:    map[int64]struct{}{2: {}},
			expected: map[int64]struct{}{2: {}},
		},
		{
			desc:      "query error",
			queryErr:  context.DeadlineExceeded,
			skipInner: true,
			expErr:    context.DeadlineExceeded.Error(),
		},
		{
			desc:      "invalid JSON",
			mockResp:  &api.Response{Json: []byte(`{invalid`)},
			skipInner: true,
			expErr:    "dgraph: invalid character",
		},
		{
			desc:     "inner migrator error",
			mockResp: &api.Response{Json: []byte(`{"migrations":[{"version":3}]}`)},
			innerErr: errDgraph,
			expErr:   errDgraph.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockContainer, mocks := container.NewMockContainer(t)
			mockMigrator := NewMockmigrator(ctrl)

			m := dgraphMigrator{dgraphDS: dgraphDS{client: mocks.DGraph}, migrator: mockMigrator}

			mocks.DGraph.EXPECT().Query(gomock.Any(), getMigrationVersionsQuery).Return(tc.mockResp, tc.queryErr)

			if !tc.skipInner {
				mockMigrator.EXPECT().getAppliedMigrations(mockContainer).Return(tc.inner, tc.innerErr)
			}

			resp, err := m.getAppliedMigrations(mockContainer)

			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
				assert.Nil(t, resp)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, resp)
		})
	}
}

func Test_DGraphCommitMigration(t *testing.T) {
	td := transactionData{
		StartTime:       time.Now(),
		MigrationNumber: 10,
		UsedDatasources: map[string]bool{dsDGraph: true},
	}

	t.Run("success commits the record", func(t *testing.T) {
		migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)
		txn := &fakeDgraphTxn{}
		mockDGraph.EXPECT().NewTxn().Return(txn)

		err := migratorWithDGraph.commitMigration(mockContainer, &td)

		require.NoError(t, err)
		assert.True(t, txn.mutated && txn.committed && txn.discarded)
		// The record must be typed as Migration and carry the version so
		// getAppliedMigrations can find it on subsequent runs.
		assert.Contains(t, string(txn.setJSON), `"dgraph.type":"Migration"`)
		assert.Contains(t, string(txn.setJSON), `"migrations.version":10`)
	})

	t.Run("mutate error is returned without commit", func(t *testing.T) {
		migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)
		txn := &fakeDgraphTxn{mutateErr: context.DeadlineExceeded}
		mockDGraph.EXPECT().NewTxn().Return(txn)

		err := migratorWithDGraph.commitMigration(mockContainer, &td)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.False(t, txn.committed)
		assert.True(t, txn.discarded)
	})

	t.Run("commit error is returned", func(t *testing.T) {
		migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)
		txn := &fakeDgraphTxn{commitErr: context.DeadlineExceeded}
		mockDGraph.EXPECT().NewTxn().Return(txn)

		err := migratorWithDGraph.commitMigration(mockContainer, &td)

		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("invalid transaction type", func(t *testing.T) {
		migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)
		mockDGraph.EXPECT().NewTxn().Return("not a txn")

		err := migratorWithDGraph.commitMigration(mockContainer, &td)

		require.ErrorIs(t, err, errInvalidDgraphTxn)
	})

	t.Run("typed-nil transaction does not panic", func(t *testing.T) {
		migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)
		mockDGraph.EXPECT().NewTxn().Return((*fakeDgraphTxn)(nil))

		err := migratorWithDGraph.commitMigration(mockContainer, &td)

		require.ErrorIs(t, err, errInvalidDgraphTxn)
	})

	t.Run("skips record when dgraph not used", func(t *testing.T) {
		migratorWithDGraph, _, mockContainer := dgraphSetup(t)
		unused := transactionData{StartTime: time.Now(), MigrationNumber: 10}

		err := migratorWithDGraph.commitMigration(mockContainer, &unused)

		require.NoError(t, err)
	})
}

func Test_DGraphBeginTransaction(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migratorWithDGraph, _, mockContainer := dgraphSetup(t)
		migratorWithDGraph.beginTransaction(mockContainer)
	})

	assert.Contains(t, logs, "Dgraph migrator begin successfully")
}

func Test_DGraphDS_ApplySchema(t *testing.T) {
	_, mockDGraph, _ := dgraphSetup(t)

	ds := dgraphDS{client: mockDGraph}
	ctx := t.Context()
	schema := "test schema"

	testCases := []struct {
		desc string
		err  error
	}{
		{"success", nil},
		{"schema error", context.DeadlineExceeded},
	}

	for i, tc := range testCases {
		mockDGraph.EXPECT().ApplySchema(ctx, schema).Return(tc.err)

		err := ds.ApplySchema(ctx, schema)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed!", i, tc.desc)
	}
}

func Test_DGraphDS_AddOrUpdateField(t *testing.T) {
	_, mockDGraph, _ := dgraphSetup(t)

	ds := dgraphDS{client: mockDGraph}
	ctx := t.Context()
	fieldName := "test"
	fieldType := "string"
	directives := "@index(exact)"

	testCases := []struct {
		desc string
		err  error
	}{
		{"success", nil},
		{"field error", context.DeadlineExceeded},
	}

	for i, tc := range testCases {
		mockDGraph.EXPECT().AddOrUpdateField(ctx, fieldName, fieldType, directives).Return(tc.err)

		err := ds.AddOrUpdateField(ctx, fieldName, fieldType, directives)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed!", i, tc.desc)
	}
}

func Test_DGraphDS_DropField(t *testing.T) {
	_, mockDGraph, _ := dgraphSetup(t)

	ds := dgraphDS{client: mockDGraph}
	ctx := t.Context()
	fieldName := "test"

	testCases := []struct {
		desc string
		err  error
	}{
		{"success", nil},
		{"drop error", context.DeadlineExceeded},
	}

	for i, tc := range testCases {
		mockDGraph.EXPECT().DropField(ctx, fieldName).Return(tc.err)

		err := ds.DropField(ctx, fieldName)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed!", i, tc.desc)
	}
}

func Test_DGraphCheckAndCreateMigrationTable_SchemaError(t *testing.T) {
	var err error

	logs := testutil.StdoutOutputForFunc(func() {
		migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)

		mockDGraph.EXPECT().ApplySchema(gomock.Any(), dgraphSchema).Return(errDgraph)

		err = migratorWithDGraph.checkAndCreateMigrationTable(mockContainer)
	})

	require.NoError(t, err)
	assert.Contains(t, logs, "Migration schema might already exist:")
}

func Test_DGraphCommitMigration_DiscardError(t *testing.T) {
	td := transactionData{
		StartTime:       time.Now(),
		MigrationNumber: 11,
		UsedDatasources: map[string]bool{dsDGraph: true},
	}

	var err error

	txn := &fakeDgraphTxn{discardErr: errDgraph}

	logs := testutil.StderrOutputForFunc(func() {
		migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)
		mockDGraph.EXPECT().NewTxn().Return(txn)

		err = migratorWithDGraph.commitMigration(mockContainer, &td)
	})

	require.NoError(t, err)
	assert.True(t, txn.committed)
	assert.Contains(t, logs, "dgraph: migration transaction discard failed: dgraph error")
}

func Test_DGraphMigratorDelegation(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockContainer, _ := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	m := dgraphMigrator{migrator: mockMigrator}
	data := transactionData{MigrationNumber: 4}

	mockMigrator.EXPECT().rollback(mockContainer, &data)
	mockLogger.EXPECT().Fatalf("Migration %v failed and rolled back", int64(4))
	mockMigrator.EXPECT().lock(gomock.Any(), gomock.Any(), mockContainer, "owner-1").Return(errDgraph)
	mockMigrator.EXPECT().unlock(mockContainer, "owner-1").Return(errDgraph)

	m.rollback(mockContainer, &data)

	require.ErrorIs(t, m.lock(t.Context(), func() {}, mockContainer, "owner-1"), errDgraph)
	require.ErrorIs(t, m.unlock(mockContainer, "owner-1"), errDgraph)
	assert.Equal(t, "DGraph", m.name())
}
