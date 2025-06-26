package ftp

import (
	"context"
	"io"
	"time"

	"github.com/jlaffaye/ftp"
)

// Logger interface is used by ftp package to log information about query execution.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
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
