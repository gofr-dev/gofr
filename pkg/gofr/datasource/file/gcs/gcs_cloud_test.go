package gcs_test

import (
	"testing"

	"gofr.dev/pkg/gofr/datasource/file"

	"gofr.dev/pkg/gofr/datasource/file/gcs"
)

// TestNew_ReturnsCloudFileSystem verifies that New() returns a value that satisfies
// file.CloudFileSystem at compile time (implicit) and at runtime via AsCloud.
func TestNew_ReturnsCloudFileSystem(t *testing.T) {
	cfg := &gcs.Config{BucketName: "test-bucket"}

	cfs := gcs.New(cfg)

	if cfs == nil {
		t.Fatal("expected non-nil CloudFileSystem from New()")
	}

	// AsCloud must succeed because New() explicitly declares CloudFileSystem.
	if _, ok := file.AsCloud(cfs); !ok {
		t.Fatal("AsCloud should succeed for a value returned by gcs.New()")
	}
}
