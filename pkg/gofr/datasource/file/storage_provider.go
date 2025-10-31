package file

import (
	"context"
	"io"
	"time"
)

// StorageProvider is the unified interface for all cloud storage providers (GCS, S3, FTP, SFTP).
// It abstracts low-level storage operations, allowing common implementations for directory operations,
// metadata handling, and observability.
//
// All providers (GCS, S3, FTP, SFTP) implement this interface to integrate with GoFr's file system.
type StorageProvider interface {
	Connect(ctx context.Context) error
	Health(ctx context.Context) error
	Close() error

	NewReader(ctx context.Context, name string) (io.ReadCloser, error)
	NewRangeReader(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error)
	NewWriter(ctx context.Context, name string) io.WriteCloser

	DeleteObject(ctx context.Context, name string) error
	CopyObject(ctx context.Context, src, dst string) error
	StatObject(ctx context.Context, name string) (*ObjectInfo, error)

	ListObjects(ctx context.Context, prefix string) ([]string, error)
	ListDir(ctx context.Context, prefix string) ([]ObjectInfo, []string, error)
}

// ObjectInfo represents unified metadata for any storage object (file or directory).
// It abstracts provider-specific attributes into a common structure.
type ObjectInfo struct {
	Name         string
	Size         int64 // Size in bytes
	ContentType  string
	LastModified time.Time
	IsDir        bool
	Metadata     map[string]string
}
