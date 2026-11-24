package s3

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Calls should fail without panicking even before the app has installed a logger.
func TestFileOperationsBeforeConnectReturnError(t *testing.T) {
	f, ok := New(&Config{BucketName: "test-bucket"}).(*FileSystem)
	require.True(t, ok, "New should return *FileSystem")

	assertDisconnectedOperations(t, f)
}

// A call on a FileSystem that has no logger yet must return the error, not panic
// in the deferred sendOperationStats. New installs no logger, so this exercises
// the nil-logger branch directly.
func TestFileOperationBeforeLoggerReturnsErrorWithoutPanic(t *testing.T) {
	f, ok := New(&Config{BucketName: "test-bucket"}).(*FileSystem)
	require.True(t, ok, "New should return *FileSystem")
	require.Nil(t, f.logger, "New must not install a logger")

	require.NotPanics(t, func() {
		_, err := f.Create("a.txt")
		require.ErrorIs(t, err, errS3ClientNotInitialized)
	})
}

// Exercise the actual Connect failure rather than only constructing an
// uninitialized FileSystem. The missing profile fails before a client is set up.
func TestFileOperationsAfterFailedConnectReturnError(t *testing.T) {
	t.Setenv("AWS_PROFILE", "gofr-s3-test-nonexistent-profile")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing-credentials"))

	f, ok := New(&Config{BucketName: "test-bucket", Region: "us-east-1"}).(*FileSystem)
	require.True(t, ok, "New should return *FileSystem")

	logger := NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()
	logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	f.UseLogger(logger)

	f.Connect()

	require.Nil(t, f.conn, "expected Connect to fail and leave the S3 client uninitialized")

	assertDisconnectedOperations(t, f)
}

func assertDisconnectedOperations(t *testing.T, f *FileSystem) {
	t.Helper()

	// Observe the same FileLog path consumed by GoFr logging.
	var logs []FileLog

	logger := NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug(gomock.Any()).Do(func(args ...any) {
		fl, ok := args[0].(*FileLog)
		require.True(t, ok, "expected *FileLog, got %T", args[0])

		logs = append(logs, *fl)
	}).AnyTimes()
	f.UseLogger(logger)

	tests := []struct {
		name      string
		operation string
		run       func() error
	}{
		{"Create", "CREATE FILE", func() error { _, err := f.Create("a.txt"); return err }},
		{"CreateWithOptions", "CREATE FILE WITH OPTIONS", func() error {
			_, err := f.CreateWithOptions(context.Background(), "a.txt", nil)
			return err
		}},
		{"Mkdir", "MKDIR", func() error { return f.Mkdir("foo", os.ModePerm) }},
		{"MkdirAll", "MKDIR", func() error { return f.MkdirAll("foo/bar", os.ModePerm) }},
		{"Open", "OPEN FILE", func() error { _, err := f.Open("a.txt"); return err }},
		{"OpenFile", "OPEN FILE", func() error {
			_, err := f.OpenFile("a.txt", os.O_RDONLY, os.ModePerm)
			return err
		}},
		{"ReadDir", "READDIR", func() error { _, err := f.ReadDir("."); return err }},
		{"Remove", "REMOVE FILE", func() error { return f.Remove("a.txt") }},
		{"RemoveAll directory", "REMOVEALL", func() error { return f.RemoveAll("foo") }},
		{"RemoveAll file", "REMOVE FILE", func() error { return f.RemoveAll("a.txt") }},
		{"Rename", "RENAME", func() error { return f.Rename("a.txt", "b.txt") }},
		{"Stat", "STAT", func() error { _, err := f.Stat("a.txt"); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(logs)

			err := tt.run()
			require.ErrorIs(t, err, errS3ClientNotInitialized)

			require.Len(t, logs, before+1, "expected exactly one operation log")

			got := logs[len(logs)-1]
			assert.Equal(t, tt.operation, got.Operation)

			require.NotNil(t, got.Status)
			assert.Equal(t, statusErr, *got.Status)

			require.NotNil(t, got.Message)
			assert.Equal(t, errS3ClientNotInitialized.Error(), *got.Message)
		})
	}
}
