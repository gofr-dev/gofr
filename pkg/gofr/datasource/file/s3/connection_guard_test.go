package s3

import (
	"context"
	"errors"
	"os"
	"testing"
)

// A newly created file store is in the same uninitialized state as a store
// whose Connect failed before assigning its client.
func TestFileOperationsBeforeConnectReturnError(t *testing.T) {
	f, ok := New(&Config{BucketName: "test-bucket"}).(*FileSystem)
	if !ok {
		t.Fatal("New should return *FileSystem")
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{"Create", func() error { _, err := f.Create("a.txt"); return err }},
		{"CreateWithOptions", func() error { _, err := f.CreateWithOptions(context.Background(), "a.txt", nil); return err }},
		{"Mkdir", func() error { return f.Mkdir("foo", os.ModePerm) }},
		{"MkdirAll", func() error { return f.MkdirAll("foo/bar", os.ModePerm) }},
		{"Open", func() error { _, err := f.Open("a.txt"); return err }},
		{"OpenFile", func() error { _, err := f.OpenFile("a.txt", os.O_RDONLY, os.ModePerm); return err }},
		{"ReadDir", func() error { _, err := f.ReadDir("."); return err }},
		{"Remove", func() error { return f.Remove("a.txt") }},
		{"RemoveAll", func() error { return f.RemoveAll("foo") }},
		{"Rename", func() error { return f.Rename("a.txt", "b.txt") }},
		{"Stat", func() error { _, err := f.Stat("a.txt"); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); !errors.Is(err, ErrNotConnected) {
				t.Fatalf("expected ErrNotConnected, got %v", err)
			}
		})
	}
}
