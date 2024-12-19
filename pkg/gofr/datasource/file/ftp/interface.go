package ftp

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/jlaffaye/ftp"
)

// FTPFile represents a file in the filesystem.
type FTPFile interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt

	ReadAll() (RowReader, error)
}

type FileInfo interface {
	Name() string       // base name of the file
	Size() int64        // length in bytes for regular files; system-dependent for others
	ModTime() time.Time // modification time
	Mode() os.FileMode  // file mode bits
	IsDir() bool        // abbreviation for Mode().IsDir()
}

type RowReader interface {
	Next() bool
	Scan(interface{}) error
}

// FTPFileSystem : Any simulated or real filesystem should implement this interface.
type FTPFileSystem interface {
	// Create creates a file in the filesystem, returning the file and an
	// error, if any happens.
	Create(name string) (FTPFile, error)

	// TODO - Lets make bucket constant for MkdirAll as well, we might create buckets from migrations
	// Mkdir creates a directory in the filesystem, return an error if any
	// happens.
	Mkdir(name string, perm os.FileMode) error

	// MkdirAll creates a directory path and all parents that does not exist
	// yet.
	MkdirAll(path string, perm os.FileMode) error

	// Open opens a file, returning it or an error, if any happens.
	Open(name string) (FTPFile, error)

	// OpenFile opens a file using the given flags and the given mode.
	OpenFile(name string, flag int, perm os.FileMode) (FTPFile, error)

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

// Logger interface is used by ftp package to log information about query execution.
type Logger interface {
	Debug(args ...interface{})
	Debugf(pattern string, args ...interface{})
	Logf(pattern string, args ...interface{})
	Errorf(pattern string, args ...interface{})
}

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}

// serverConn represents a connection to an FTP server.
type serverConn interface {
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
	FileSize(name string) (int64, error)
	CurrentDir() (string, error)
	ChangeDir(path string) error
	List(string) ([]*ftp.Entry, error)
	GetTime(path string) (time.Time, error)
}

// ftpResponse interface mimics the behavior of *ftp.Response returned on retrieval of file from FTP.
type ftpResponse interface {
	Read(buf []byte) (int, error)
	Close() error
	SetDeadline(t time.Time) error
}

type FileSystemProvider interface {
	FTPFileSystem

	// UseLogger sets the logger for the FileSystem client.
	UseLogger(logger interface{})

	// UseMetrics sets the metrics for the FileSystem client.
	UseMetrics(metrics interface{})

	// Connect establishes a connection to FileSystem and registers metrics using the provided configuration when the client was Created.
	Connect()
}
