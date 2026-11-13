package s3

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/mock/gomock"
)

// Calls should fail without panicking even before the app has installed a logger.
func TestFileOperationsBeforeConnectReturnError(t *testing.T) {
	f, ok := New(&Config{BucketName: "test-bucket"}).(*FileSystem)
	if !ok {
		t.Fatal("New should return *FileSystem")
	}

	assertDisconnectedOperations(t, f)
}

// Exercise the actual Connect failure rather than only constructing an
// uninitialized FileSystem. The missing profile fails before a client is set up.
func TestFileOperationsAfterFailedConnectReturnError(t *testing.T) {
	t.Setenv("AWS_PROFILE", "gofr-s3-test-nonexistent-profile")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing-credentials"))

	f, ok := New(&Config{BucketName: "test-bucket", Region: "us-east-1"}).(*FileSystem)
	if !ok {
		t.Fatal("New should return *FileSystem")
	}

	logger := NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()
	logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	f.UseLogger(logger)

	f.Connect()
	if f.conn != nil {
		t.Fatal("expected Connect to fail and leave the S3 client uninitialized")
	}

	assertDisconnectedOperations(t, f)
}

func assertDisconnectedOperations(t *testing.T, f *FileSystem) {
	t.Helper()

	// Observe the same FileLog path consumed by GoFr logging/metrics.
	var logs []FileLog
	logger := NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug(gomock.Any()).Do(func(args ...any) {
		fl, ok := args[0].(*FileLog)
		if !ok {
			t.Fatalf("expected *FileLog, got %T", args[0])
		}
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
			if !errors.Is(err, errS3ClientNotInitialized) {
				t.Fatalf("expected errS3ClientNotInitialized, got %v", err)
			}

			if len(logs) != before+1 {
				t.Fatalf("expected one operation log, got %d", len(logs)-before)
			}

			got := logs[len(logs)-1]
			if got.Operation != tt.operation {
				t.Errorf("operation = %q, want %q", got.Operation, tt.operation)
			}
			if got.Status == nil || *got.Status != statusErr {
				t.Errorf("status = %v, want %q", got.Status, statusErr)
			}
			if got.Message == nil || *got.Message != errS3ClientNotInitialized.Error() {
				t.Errorf("message = %v, want %q", got.Message, errS3ClientNotInitialized.Error())
			}
		})
	}
}
