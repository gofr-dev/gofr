package dgraph

import (
	"context"
	"errors"
	"testing"

	"github.com/dgraph-io/dgo/v210/protos/api"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

var (
	errCommitFailed  = errors.New("commit failed")
	errDiscardFailed = errors.New("discard failed")
)

// recordingTxn counts what Mutate does to the transaction it opens.
//
// A gomock Txn cannot answer "was Commit called?" in this package: setupDB runs ctrl.Finish()
// when it returns, which marks the controller finished before the test body starts, so the
// t.Cleanup verification gomock.NewController installs short-circuits (mock v0.6.0
// controller.go:268) and an unmet expectation is never reported. Counting here asserts on what
// happened rather than on the mock library's bookkeeping.
type recordingTxn struct {
	mutateErr  error
	commitErr  error
	discardErr error

	mutations int
	commits   int
	discards  int
}

func (r *recordingTxn) Mutate(_ context.Context, _ *api.Mutation) (*api.Response, error) {
	r.mutations++

	if r.mutateErr != nil {
		return nil, r.mutateErr
	}

	return &api.Response{Json: []byte(`{}`)}, nil
}

func (r *recordingTxn) Commit(context.Context) error {
	r.commits++

	return r.commitErr
}

func (r *recordingTxn) Discard(context.Context) error {
	r.discards++

	return r.discardErr
}

func (r *recordingTxn) BestEffort() Txn { return r }

func (*recordingTxn) Query(context.Context, string) (*api.Response, error) { return nil, nil }

func (*recordingTxn) QueryRDF(context.Context, string) (*api.Response, error) { return nil, nil }

func (*recordingTxn) QueryWithVars(context.Context, string, map[string]string) (*api.Response, error) {
	return nil, nil
}

func (*recordingTxn) QueryRDFWithVars(context.Context, string,
	map[string]string) (*api.Response, error) {
	return nil, nil
}

func (*recordingTxn) Do(context.Context, *api.Request) (*api.Response, error) { return nil, nil }

func setupWithTxn(t *testing.T, txn Txn) (*Client, *MockLogger) {
	t.Helper()

	ctrl := gomock.NewController(t)

	logger := NewMockLogger(ctrl)
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()
	logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Log(gomock.Any()).AnyTimes()
	logger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	metrics := NewMockMetrics(ctrl)
	metrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	client := New(Config{Host: "localhost", Port: "9080"})
	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-dgraph"))

	dgraphClient := NewMockDgraphClient(ctrl)
	dgraphClient.EXPECT().NewTxn().Return(txn).AnyTimes()
	client.client = dgraphClient

	return client, logger
}

func Test_Mutate_TransactionHandling(t *testing.T) {
	tests := []struct {
		name         string
		commitNow    bool
		txn          recordingTxn
		wantErr      error
		wantResp     bool
		wantCommits  int
		wantDiscards int
	}{
		{
			// The defect: dgo only finishes the transaction when CommitNow is set, so without
			// an explicit Commit the write was staged and abandoned, silently.
			name:         "commits when CommitNow is not set",
			wantResp:     true,
			wantCommits:  1,
			wantDiscards: 1,
		},
		{
			// dgo has already finished the transaction; Commit again returns ErrFinished.
			name:         "does not commit again when CommitNow is set",
			commitNow:    true,
			wantResp:     true,
			wantCommits:  0,
			wantDiscards: 1,
		},
		{
			name:         "commit failure is returned",
			txn:          recordingTxn{commitErr: errCommitFailed},
			wantErr:      errCommitFailed,
			wantCommits:  1,
			wantDiscards: 1,
		},
		{
			name:         "mutate failure is returned without committing",
			txn:          recordingTxn{mutateErr: errMutationFailed},
			wantErr:      errMutationFailed,
			wantCommits:  0,
			wantDiscards: 1,
		},
		{
			// A failing Discard must not turn a committed write into an error.
			name:         "discard failure does not fail a committed write",
			txn:          recordingTxn{discardErr: errDiscardFailed},
			wantResp:     true,
			wantCommits:  1,
			wantDiscards: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			txn := tc.txn
			client, _ := setupWithTxn(t, &txn)

			resp, err := client.Mutate(t.Context(), &api.Mutation{
				SetJson:   []byte(`{"name":"GoFr"}`),
				CommitNow: tc.commitNow,
			})

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantResp, resp != nil, "response")
			require.Equal(t, 1, txn.mutations, "Mutate calls")
			require.Equal(t, tc.wantCommits, txn.commits, "Commit calls")
			require.Equal(t, tc.wantDiscards, txn.discards, "Discard calls")
		})
	}
}
