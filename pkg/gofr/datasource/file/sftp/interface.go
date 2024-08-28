package sftp

import (
	"context"
	"os"

	"github.com/pkg/sftp"
)

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

type sftpClient interface {
	Create(path string) (*sftp.File, error)
	Mkdir(path string) error
	MkdirAll(path string) error
	Open(path string) (*sftp.File, error)
	OpenFile(path string, f int) (*sftp.File, error)
	Remove(path string) error
	RemoveAll(path string) error
	Rename(oldname, newname string) error
	ReadDir(p string) ([]os.FileInfo, error)
	Stat(p string) (os.FileInfo, error)
	Getwd() (string, error)
}
