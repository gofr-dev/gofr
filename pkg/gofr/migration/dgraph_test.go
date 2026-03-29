// --- Additional tests for Dgraph migration transaction ---
type fakeNotTxn struct{}

func Test_DGraphCommitMigration_InvalidTxn(t *testing.T) {
	migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)

	td := transactionData{
		StartTime:       time.Now(),
		MigrationNumber: 42,
	}

	// NewTxn returns a type that does NOT implement dgraphTxn
	mockDGraph.EXPECT().NewTxn().Return(&fakeNotTxn{})

	err := migratorWithDGraph.commitMigration(mockContainer, td)

	assert.Equal(t, errInvalidDgraphTxn, err)
}


type mockDgraphTxn2 struct {
	mutateErr   error
	commitErr   error
	discarded   bool
	commitCalled bool
}

func (m *mockDgraphTxn2) Mutate(context.Context, *api.Mutation) (*api.Response, error) {
	return nil, m.mutateErr
}
func (m *mockDgraphTxn2) Commit(context.Context) error {
	m.commitCalled = true
	return m.commitErr
}
func (m *mockDgraphTxn2) Discard(context.Context) error {
	m.discarded = true
	return nil
}

func Test_DGraphCommitMigration_MutateError(t *testing.T) {
	migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)

	td := transactionData{
		StartTime:       time.Now(),
		MigrationNumber: 43,
	}

	txn := &mockDgraphTxn2{mutateErr: errors.New("mutation failed")}
	// NewTxn returns our mock txn
	mockDGraph.EXPECT().NewTxn().Return(txn)

	err := migratorWithDGraph.commitMigration(mockContainer, td)

	assert.EqualError(t, err, "mutation failed")
	assert.True(t, txn.discarded, "discard should be called on mutation error")
	assert.False(t, txn.commitCalled, "commit should NOT be called on mutation error")
}

func Test_DGraphCommitMigration_CommitError(t *testing.T) {
	migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)

	td := transactionData{
		StartTime:       time.Now(),
		MigrationNumber: 44,
	}

	txn := &mockDgraphTxn2{commitErr: errors.New("commit failed")}
	// NewTxn returns our mock txn
	mockDGraph.EXPECT().NewTxn().Return(txn)

	err := migratorWithDGraph.commitMigration(mockContainer, td)

	assert.EqualError(t, err, "commit failed")
	assert.True(t, txn.discarded, "discard should be called on commit error")
	assert.True(t, txn.commitCalled, "commit should be called")
}
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

type mockDgraphTxn struct {
	mutateErr  error
	commitErr  error
	commitDone bool
	discarded  bool
}

func (m *mockDgraphTxn) Mutate(context.Context, *api.Mutation) (*api.Response, error) {
	return nil, m.mutateErr
}

func (m *mockDgraphTxn) Commit(context.Context) error {
	m.commitDone = true

	return m.commitErr
}

func (m *mockDgraphTxn) Discard(context.Context) error {
	m.discarded = true

	return nil
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

func Test_DGraphGetLastMigration(t *testing.T) {
	migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)

	testCases := []struct {
		desc     string
		err      error
		mockResp map[string]any
		expected int64
	}{
		{
			desc: "success",
			err:  nil,
			mockResp: map[string]any{
				"migrations": []map[string]any{
					{
						"version": float64(10),
					},
				},
			},
			expected: 10,
		},
		{
			desc:     "query error",
			err:      context.DeadlineExceeded,
			mockResp: nil,
			expected: -1,
		},
		{
			desc:     "empty response",
			err:      nil,
			mockResp: map[string]any{},
			expected: 0,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Set up mock expectation for the main query
			mockDGraph.EXPECT().Query(gomock.Any(), getLastMigrationQuery).
				Return(tc.mockResp, tc.err)

			resp, err := migratorWithDGraph.getLastMigration(mockContainer)

			assert.Equal(t, tc.expected, resp, "TEST[%v] Failed!", i)

			if tc.err != nil {
				assert.ErrorContains(t, err, tc.err.Error(), "TEST[%v] Failed!", i)
			} else {
				assert.NoError(t, err, "TEST[%v] Failed!", i)
			}
		})
	}
}

func Test_DGraphCommitMigration(t *testing.T) {
	migratorWithDGraph, mockDGraph, mockContainer := dgraphSetup(t)

	timeNow := time.Now()

	testCases := []struct {
		desc      string
		mutateErr error
		commitErr error
		err       error
	}{
		{"success", nil, nil, nil},
		{"mutation failed", context.DeadlineExceeded, nil, context.DeadlineExceeded},
		{"commit failed", nil, context.Canceled, context.Canceled},
	}

	td := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
	}

	for i, tc := range testCases {
		tx := &mockDgraphTxn{mutateErr: tc.mutateErr, commitErr: tc.commitErr}
		mockDGraph.EXPECT().NewTxn().Return(tx)

		err := migratorWithDGraph.commitMigration(mockContainer, td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed!", i, tc.desc)
		assert.True(t, tx.discarded, "TEST[%v]\n %v Failed!", i, tc.desc)

		if tc.mutateErr == nil {
			assert.True(t, tx.commitDone, "TEST[%v]\n %v Failed!", i, tc.desc)
		} else {
			assert.False(t, tx.commitDone, "TEST[%v]\n %v Failed!", i, tc.desc)
		}
	}
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
