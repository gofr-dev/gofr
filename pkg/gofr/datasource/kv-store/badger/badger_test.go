package badger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

func setupDB(t *testing.T) *Client {
	t.Helper()
	cl := New(Configs{DirPath: t.TempDir()})

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := NewMockLogger(ctrl)

	mockMetrics.EXPECT().NewHistogram("app_badger_stats", "Response time of Badger queries in microseconds.", gomock.Any())

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_badger_stats", gomock.Any(), "database", cl.configs.DirPath,
		"type", gomock.Any()).AnyTimes()

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	cl.UseLogger(mockLogger)
	cl.UseMetrics(mockMetrics)
	cl.Connect()

	return cl
}

func Test_ClientSet(t *testing.T) {
	cl := setupDB(t)

	err := cl.Set(context.Background(), "lkey", "lvalue")

	require.NoError(t, err)
}

func Test_ClientGet(t *testing.T) {
	cl := setupDB(t)

	err := cl.Set(context.Background(), "lkey", "lvalue")
	require.NoError(t, err)

	val, err := cl.Get(context.Background(), "lkey")

	require.NoError(t, err)
	assert.Equal(t, "lvalue", val)
}

func Test_ClientGetError(t *testing.T) {
	cl := setupDB(t)

	val, err := cl.Get(context.Background(), "lkey")

	require.EqualError(t, err, "Key not found")
	assert.Empty(t, val)
}

func Test_ClientDeleteSuccessError(t *testing.T) {
	cl := setupDB(t)

	err := cl.Delete(context.Background(), "lkey")

	require.NoError(t, err)
}

func Test_ClientHealthCheck(t *testing.T) {
	cl := setupDB(t)

	val, err := cl.HealthCheck(context.Background())

	require.NoError(t, err)
	assert.Contains(t, fmt.Sprint(val), "UP")
}

func Test_ClientConnectError(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o600))

	tests := []struct {
		desc    string
		dirPath string
	}{
		{desc: "path is a regular file", dirPath: filePath},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockMetrics := NewMockMetrics(ctrl)
			mockLogger := NewMockLogger(ctrl)

			mockMetrics.EXPECT().NewHistogram("app_badger_stats", "Response time of Badger queries in microseconds.", gomock.Any())
			mockLogger.EXPECT().Debugf("connecting to BadgerDB at %v", tc.dirPath)
			mockLogger.EXPECT().Errorf("error while connecting to BadgerDB: %v", gomock.Any())

			cl := New(Configs{DirPath: tc.dirPath})
			cl.UseLogger(mockLogger)
			cl.UseMetrics(mockMetrics)

			cl.Connect()

			assert.Nil(t, cl.db)
		})
	}
}

func Test_ClientUseTracer(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")

	tests := []struct {
		desc      string
		tracer    any
		expTracer trace.Tracer
	}{
		{desc: "valid tracer is set", tracer: tracer, expTracer: tracer},
		{desc: "invalid tracer is ignored", tracer: "tracer"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cl := New(Configs{DirPath: t.TempDir()})

			cl.UseTracer(tc.tracer)

			assert.Equal(t, tc.expTracer, cl.tracer)
		})
	}
}

func Test_ClientOperationsWithTracer(t *testing.T) {
	tests := []struct {
		desc      string
		operation func(t *testing.T, cl *Client) (string, error)
		expVal    string
		expErr    error
	}{
		{
			desc: "set and get with tracer",
			operation: func(t *testing.T, cl *Client) (string, error) {
				t.Helper()

				require.NoError(t, cl.Set(t.Context(), "key", "value"))

				return cl.Get(t.Context(), "key")
			},
			expVal: "value",
		},
		{
			desc: "delete with tracer",
			operation: func(t *testing.T, cl *Client) (string, error) {
				t.Helper()

				return "", cl.Delete(t.Context(), "key")
			},
		},
		{
			desc: "set with empty key fails in transaction",
			operation: func(t *testing.T, cl *Client) (string, error) {
				t.Helper()

				return "", cl.Set(t.Context(), "", "value")
			},
			expErr: badger.ErrEmptyKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cl := setupDB(t)
			cl.UseTracer(noop.NewTracerProvider().Tracer("test"))

			val, err := tc.operation(t, cl)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expVal, val)
		})
	}
}

func Test_ClientUseTransactionCommitConflict(t *testing.T) {
	tests := []struct {
		desc   string
		expErr error
	}{
		{desc: "commit fails when key is modified concurrently", expErr: badger.ErrConflict},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cl := setupDB(t)

			err := cl.useTransaction(func(txn *badger.Txn) error {
				// Reading the key registers it for conflict detection.
				_, _ = txn.Get([]byte("key"))

				// A concurrent committed write to the same key makes the outer commit conflict.
				require.NoError(t, cl.Set(t.Context(), "key", "other"))

				return txn.Set([]byte("key"), []byte("value"))
			})

			require.ErrorIs(t, err, tc.expErr)
		})
	}
}

func Test_ClientHealthCheckDown(t *testing.T) {
	tests := []struct {
		desc      string
		expStatus string
		expErr    error
	}{
		{desc: "database closed", expStatus: "DOWN", expErr: errStatusDown},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cl := setupDB(t)
			require.NoError(t, cl.db.Close())

			val, err := cl.HealthCheck(t.Context())

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expStatus, val.(*Health).Status)
			assert.Equal(t, cl.configs.DirPath, val.(*Health).Details["location"])
		})
	}
}
