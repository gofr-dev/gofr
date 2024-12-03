package file

import (
	"errors"
	"io"
	"io/fs"
	"os"
)

var (
	ErrFileClosed        = errors.New("file is closed")
	ErrOutOfRange        = errors.New("out of range")
	ErrTooLarge          = errors.New("too large")
	ErrFileNotFound      = os.ErrNotExist
	ErrFileExists        = os.ErrExist
	ErrDestinationExists = os.ErrExist
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

type RowReader interface {
	Next() bool
	Scan(interface{}) error
}

// FileSystem : Any simulated or real filesystem should implement this interface.
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
	ReadDir(dir string) ([]fs.FileInfo, error)

	// Stat returns the file/directory information in the directory.
	Stat(name string) (fs.FileInfo, error)

	// ChDir changes the current directory.
	ChDir(dirname string) error

	// Getwd returns the path of the current directory.
	Getwd() (string, error)
}
