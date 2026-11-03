package s3

import (
	"context"
	"errors"

	"gofr.dev/pkg/gofr/datasource/file"
)

// Deprecated shims kept so code written against earlier releases still compiles.
// They will be removed in a future major release.

// S3File is the handle returned by Open and Create.
//
// Deprecated: the S3 file store now returns the shared file.CommonFile; use the file.File
// interface instead. S3File is an alias for it, so type assertions keep matching.
//
//nolint:revive // s3.S3File is repetitive, but it is an existing exported name.
type S3File = file.CommonFile

// FileSystem is the S3 file store type returned by New.
//
// Deprecated: use the file.CloudFileSystem interface that New returns.
type FileSystem = fileSystem

// Logger is the logging interface the S3 package accepted in earlier releases.
//
// Deprecated: the S3 package now uses datasource.Logger. This interface is no longer used.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Warnf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
}

// Metrics is the metrics interface the S3 package accepted in earlier releases.
//
// Deprecated: the S3 package now uses the file package's metrics interface. This interface is no longer used.
type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}

var (
	// ErrOperationNotPermitted is no longer returned by the S3 package.
	//
	// Deprecated: kept for source compatibility only.
	ErrOperationNotPermitted = errors.New("operation not permitted")

	// ErrOutOfRange is returned for an out-of-range offset. It is the shared file.ErrOutOfRange.
	//
	// Deprecated: use file.ErrOutOfRange.
	ErrOutOfRange = file.ErrOutOfRange

	// ErrNilResponse is no longer returned by the S3 package.
	//
	// Deprecated: kept for source compatibility only.
	ErrNilResponse = errors.New("response retrieved is nil ")
)
