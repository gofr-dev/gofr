package ftp

import (
	"io"

	"github.com/jlaffaye/ftp"
)

// FTPClient interface defines the methods needed for FTP operations.
type FTPClient interface {
	Dial(string, ...ftp.DialOption) (Conn, error)
}

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
