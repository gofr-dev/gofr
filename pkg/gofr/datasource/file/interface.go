package file

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
)

// File represents a file in the filesystem.
type File interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt

	ReadAll() (RowReader, error)

	Name() string
	Size() int64
	ModTime() time.Time
	IsDir() bool
	Mode() os.FileMode
	Sys() any
}

// FileInfo : Any simulated or real file should implement this interface.
//
//nolint:revive // let's consider file.FileInfo doesn't sound repetitive
type FileInfo interface {
	Name() string       // base name of the file
	Size() int64        // length in bytes for regular files; system-dependent for others
	ModTime() time.Time // modification time
	Mode() os.FileMode  // file mode bits
	IsDir() bool        // abbreviation for Mode().IsDir()
}

type RowReader interface {
	Next() bool
	Scan(any) error
}

// StorageProvider abstracts low-level storage operations (stateless).
// This is the interface that each provider (GCS, S3, FTP, SFTP) must implement.
type StorageProvider interface {
	Connect(ctx context.Context) error

	NewReader(ctx context.Context, name string) (io.ReadCloser, error)
	NewRangeReader(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error)
	NewWriter(ctx context.Context, name string) io.WriteCloser

	DeleteObject(ctx context.Context, name string) error
	CopyObject(ctx context.Context, src, dst string) error
	StatObject(ctx context.Context, name string) (*ObjectInfo, error)

	ListObjects(ctx context.Context, prefix string) ([]string, error)
	ListDir(ctx context.Context, prefix string) (objects []ObjectInfo, prefixes []string, err error)
}

// ObjectInfo represents cloud storage object metadata.
type ObjectInfo struct {
	Name         string
	Size         int64
	ContentType  string
	LastModified time.Time
	IsDir        bool
}

// Removed duplicate FileSystem interface definition. Use the one in common_fs.go.

var (
	ErrOutOfRange   = errors.New("out of range")
	ErrFileNotFound = os.ErrNotExist
)

const (
	StatusSuccess   = "SUCCESS"
	StatusError     = "ERROR"
	MsgWriterClosed = "Writer closed successfully"
	MsgReaderClosed = "Reader closed successfully"
)
