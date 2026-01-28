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

// MetadataWriter is an optional extension for StorageProvider.
type MetadataWriter interface {
	NewWriterWithOptions(ctx context.Context, name string, opts *FileOptions) io.WriteCloser
}

// SignedURLProvider is an optional extension for StorageProvider.
type SignedURLProvider interface {
	SignedURL(ctx context.Context, name string, expiry time.Duration, opts *FileOptions) (string, error)
}

// ObjectInfo represents cloud storage object metadata.
type ObjectInfo struct {
	Name         string
	Size         int64
	ContentType  string
	LastModified time.Time
	IsDir        bool
}

// FileSystem : Any simulated or real filesystem should implement this interface.
//
//nolint:revive // let's consider file.FileSystem doesn't sound repetitive
type FileSystem interface {
	// Create creates a file in the filesystem, returning the file and an
	// error, if any happens.
	Create(name string) (File, error)

	// TODO - Lets make bucket constant for MkdirAll as well, we might create buckets from migrations
	// Mkdir creates a directory in the filesystem, return an error if any
	// happens.
	Mkdir(name string, perm os.FileMode) error

	// MkdirAll creates a directory path and all parents that does not exist
	// yet.
	MkdirAll(path string, perm os.FileMode) error

	// Open opens a file, returning it or an error, if any happens.
	Open(name string) (File, error)

	// OpenFile opens a file using the given flags and the given mode.
	OpenFile(name string, flag int, perm os.FileMode) (File, error)

	// Remove removes a file identified by name, returning an error, if any
	// happens.
	Remove(name string) error

	// RemoveAll removes a directory path and any children it contains. It
	// does not fail if the path does not exist (return nil).
	RemoveAll(path string) error

	// Rename renames a file.
	Rename(oldname, newname string) error

	// ReadDir returns a list of files/directories present in the directory.
	ReadDir(dir string) ([]FileInfo, error)

	// Stat returns the file/directory information in the directory.
	Stat(name string) (FileInfo, error)

	// ChDir changes the current directory.
	ChDir(dirname string) error

	// Getwd returns the path of the current directory.
	Getwd() (string, error)
}

// FileSystemProvider : Any simulated or real filesystem provider should implement this interface.
//
//nolint:revive // let's consider file.FileSystemProvider doesn't sound repetitive
type FileSystemProvider interface {
	FileSystem

	// UseLogger sets the logger for the FileSystem client.
	UseLogger(logger any)

	// UseMetrics sets the metrics for the FileSystem client.
	UseMetrics(metrics any)

	// Connect establishes a connection to FileSystem and registers metrics using the provided configuration when the client was Created.
	Connect()
}

// ============================================================
// OPTIONAL CAPABILITY INTERFACES
// ============================================================

// FileOptions represents optional file metadata for Create/Sign operations.
//
//nolint:revive // keep name FileOptions for clarity across packages
type FileOptions struct {
	ContentType        string            `json:"content_type,omitempty"`
	ContentDisposition string            `json:"content_disposition,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

var (
	// ErrSignedURLsNotSupported is returned when a provider does not implement signed URLs.
	ErrSignedURLsNotSupported = errors.New("signed URLs not supported by provider")
)

// AdvancedFileOperations extends FileSystem with metadata support.
type AdvancedFileOperations interface {
	CreateWithOptions(ctx context.Context, name string, opts *FileOptions) (File, error)
}

// SignedURLGenerator provides secure, time-limited URL generation.
type SignedURLGenerator interface {
	GenerateSignedURL(ctx context.Context, name string, expiry time.Duration, opts *FileOptions) (string, error)
}

// CloudFileSystem combines common cloud storage capabilities.
// This is a convenience interface for type assertions by consumers who need
// cloud-only features like metadata support and signed URLs. It intentionally
// does NOT expose any provider-specific concrete type so it remains non-breaking.
type CloudFileSystem interface {
	FileSystemProvider
	AdvancedFileOperations
	SignedURLGenerator
}

// AsCloud attempts to cast a FileSystemProvider to a CloudFileSystem.
// Returns the typed interface and true on success, otherwise false.
func AsCloud(fs FileSystemProvider) (CloudFileSystem, bool) {
	cfs, ok := fs.(CloudFileSystem)
	return cfs, ok
}

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
