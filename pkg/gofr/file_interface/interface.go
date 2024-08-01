package _interface

import (
	"io"
	"os"
)

// FileSystem : Any simulated or real filesystem should implement this interface.
type FileSystem interface {
	// Create creates a file in the filesystem, returning the file and an
	// error, if any happens.
	Create(name string) (File, error)

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
}

type FileSystemProvider interface {
	FileSystem

	// UseLogger sets the logger for the FileSystem client.
	UseLogger(logger interface{})

	// UseMetrics sets the metrics for the FileSystem client.
	UseMetrics(metrics interface{})

	// Connect establishes a connection to FileSystem and registers metrics
	// using the provided configuration when the client was Created.
	Connect()
}

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

type RowReader interface {
	Next() bool
	Scan(interface{}) error
}
