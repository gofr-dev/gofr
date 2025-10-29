package file

import (
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

var (
	ErrFileClosed        = errors.New("File is closed")
	ErrOutOfRange        = errors.New("out of range")
	ErrTooLarge          = errors.New("too large")
	ErrFileNotFound      = os.ErrNotExist
	ErrFileExists        = os.ErrExist
	ErrDestinationExists = os.ErrExist
)

const (
	StatusSuccess   = "SUCCESS"
	StatusError     = "ERROR"
	MsgWriterClosed = "Writer closed successfully"
	MsgReaderClosed = "Reader closed successfully"
)

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
