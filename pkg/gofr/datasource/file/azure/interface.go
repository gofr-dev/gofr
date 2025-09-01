package azure

import (
	"context"
	"io"
)

// Logger interface is used by azure package to log information about query execution.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
}

// azureClient defines the interface that is used for gofr-azure datasource as well as mocks to
// streamline the testing as well as the implementation process.
type azureClient interface {
	// Share operations
	CreateDirectory(ctx context.Context, path string, options any) (any, error)
	DeleteDirectory(ctx context.Context, path string, options any) (any, error)
	ListFilesAndDirectoriesSegment(ctx context.Context, marker *string, options any) (any, error)

	// File operations
	CreateFile(ctx context.Context, path string, size int64, options any) (any, error)
	DeleteFile(ctx context.Context, path string, options any) (any, error)
	DownloadFile(ctx context.Context, options any) (any, error)
	UploadRange(ctx context.Context, offset int64, body io.ReadSeekCloser, options any) (any, error)
	GetProperties(ctx context.Context, options any) (any, error)
}

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
