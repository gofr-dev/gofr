package s3

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// guardName is an object name no local file shadows, so Open cannot fall back to the
// local file system and every call has to reach the S3 adapter.
const guardName = "gofr-s3-guard-missing.txt"

// Calls made before Connect must return errS3ClientNotInitialized instead of panicking
// on a nil client, even when GoFr has not installed a logger yet.
func TestFileOperationsBeforeConnectReturnError(t *testing.T) {
	fs, ok := New(&Config{BucketName: testBucket}).(*fileSystem)
	require.True(t, ok, "New should return *fileSystem")
	require.Nil(t, fs.Logger, "New must not install a logger")

	assertDisconnectedOperations(t, fs)
}

// A Connect that fails leaves the client unset; calls must still return the error.
func TestFileOperationsAfterFailedConnectReturnError(t *testing.T) {
	srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })

	fs, ok := New(testConfig(srv.URL)).(*fileSystem)
	require.True(t, ok, "New should return *fileSystem")

	fs.UseLogger(&testLogger{})
	fs.UseMetrics(&testMetrics{})

	// Stops the retry goroutine at its first tick so it does not outlive the test.
	defer fs.SetDisableRetry(true)

	fs.Connect()

	require.False(t, fs.IsConnected(), "expected Connect to fail against a 403 bucket")

	assertDisconnectedOperations(t, fs)
}

func assertDisconnectedOperations(t *testing.T, fs *fileSystem) {
	t.Helper()

	// writeThrough creates a file and closes it: an S3 upload only reports its error
	// once the body is written, so the error may come from Create or from Close.
	writeThrough := func(create func() (io.WriteCloser, error)) error {
		f, err := create()
		if err != nil {
			return err
		}

		return f.Close()
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{"Create", func() error {
			return writeThrough(func() (io.WriteCloser, error) { return fs.Create(guardName) })
		}},
		{"CreateWithOptions", func() error {
			return writeThrough(func() (io.WriteCloser, error) {
				return fs.CreateWithOptions(context.Background(), guardName, nil)
			})
		}},
		{"Mkdir", func() error { return fs.Mkdir("foo", os.ModePerm) }},
		{"MkdirAll", func() error { return fs.MkdirAll("foo/bar", os.ModePerm) }},
		{"Open", func() error { _, err := fs.Open(guardName); return err }},
		{"OpenFile", func() error { _, err := fs.OpenFile(guardName, os.O_RDONLY, os.ModePerm); return err }},
		{"ReadDir", func() error { _, err := fs.ReadDir("."); return err }},
		{"Remove", func() error { return fs.Remove(guardName) }},
		{"RemoveAll", func() error { return fs.RemoveAll("foo") }},
		{"Rename", func() error { return fs.Rename(guardName, "b.txt") }},
		{"Stat", func() error { _, err := fs.Stat(guardName); return err }},
		{"GenerateSignedURL", func() error {
			_, err := fs.GenerateSignedURL(context.Background(), guardName, 0, nil)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error

			require.NotPanics(t, func() { err = tt.run() })
			require.ErrorIs(t, err, errS3ClientNotInitialized)
		})
	}
}
