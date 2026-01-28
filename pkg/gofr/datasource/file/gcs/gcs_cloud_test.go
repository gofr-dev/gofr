package gcs_test

import (
	"testing"

	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/file/gcs"
)

func TestNewCloudFileSystem_ReturnsCloudInterface(t *testing.T) {
	cfg := &gcs.Config{BucketName: "test-bucket"}
	cfs, err := gcs.NewCloudFileSystem(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfs == nil {
		t.Fatalf("expected CloudFileSystem, got nil")
	}

	// Also ensure AsCloud helper works
	if _, ok := file.AsCloud(cfs); !ok {
		t.Fatalf("AsCloud should succeed for a cloud provider")
	}
}
