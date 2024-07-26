package ftp

import (
	"errors"
	"io"
	"os"
	"time"
)

// ServerConn represents a connection to an FTP server.
type ServerConn interface {
	Login(string, string) error
	Retr(string) (ftpResponse, error)
	RetrFrom(string, uint64) (ftpResponse, error)
	Stor(string, io.Reader) error
	StorFrom(string, io.Reader, uint64) error
	Rename(string, string) error
	Delete(string) error
	RemoveDirRecur(path string) error
	MakeDir(path string) error
	RemoveDir(path string) error
	Quit() error
}

// ftpResponse interface mimics the behavior of *ftp.Response returned on retrieval of file.
type ftpResponse interface {
	Read(buf []byte) (int, error)
	Close() error
	SetDeadline(t time.Time) error
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

var (
	ErrFileClosed        = errors.New("File is closed")
	ErrOutOfRange        = errors.New("out of range")
	ErrTooLarge          = errors.New("too large")
	ErrFileNotFound      = os.ErrNotExist
	ErrFileExists        = os.ErrExist
	ErrDestinationExists = os.ErrExist
)

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

// Logger interface is used by ftp package to log information about query execution.
// Developer Notes: Note that it's a reduced version of logging.Logger interface. We are not using that package to
// ensure that ftp package is not dependent on logging package. That way logging package should be easily able
// to import ftp package and provide a different "pretty" version for different log types defined here while
// avoiding the cyclical import issue. Idiomatically, interfaces should be defined by packages who are using it; unlike
// other languages. Also - accept interfaces, return concrete types.
type Logger interface {
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}
