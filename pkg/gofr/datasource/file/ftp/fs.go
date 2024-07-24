package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

type Conn struct {
	*ftp.ServerConn
}

// Retr wraps the ftp retrieve method to return a ftpResponse interface type
func (c *Conn) Retr(path string) (ftpResponse, error) {
	return c.ServerConn.Retr(path)
}

func (c *Conn) RetrFrom(path string, offset uint64) (ftpResponse, error) {
	return c.ServerConn.RetrFrom(path, offset)
}

var (
	errEmptyFilename           = errors.New("empty filename")
	errEmptyPath               = errors.New("empty path")
	errEmptyDirectory          = errors.New("empty directory")
	errInvalidArg              = errors.New("invalid filename/directory")
	directoryAlreadyExistError = "550 Create directory operation failed."
)

// ftpFileSystem represents a file system interface over FTP.
type ftpFileSystem struct {
	*ftpFile
	conn   ServerConn
	config *Config
	logger Logger
}

// Config represents the FTP configuration.
type Config struct {
	Host      string // FTP server hostname
	User      string // FTP username
	Password  string // FTP password
	Port      int    // FTP port
	RemoteDir string // Remote directory path. Base Path for all FTP Operations.
}

// New initializes a new instance of ftpFileSystem with provided configuration.
func New(config *Config) FileSystem {
	return &ftpFileSystem{config: config}
}

// UseLogger sets the logger interface for the FTP file system.
func (f *ftpFileSystem) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the metrics for the ftpFileSystem client which asserts the Metrics interface.
// Currently not implemented.
func (*ftpFileSystem) UseMetrics(_ interface{}) {

}

// Connect establishes a connection to the FTP server and logs in.
func (f *ftpFileSystem) Connect() {
	ftpServer := fmt.Sprintf("%v:%v", f.config.Host, f.config.Port)

	const dialTimeout = 5 * time.Second

	conn, err := ftp.Dial(ftpServer, ftp.DialWithTimeout(dialTimeout))
	if err != nil {
		f.logger.Errorf("Failed to connect to FTP server: %v", err)
		return
	}

	f.conn = &Conn{conn}

	err = conn.Login(f.config.User, f.config.Password)
	if err != nil {
		f.logger.Errorf("Failed to login: %v", err)
	} else {
		f.logger.Logf("Login Successful")
	}
}

// Create creates an empty file on the FTP server.
func (f *ftpFileSystem) Create(name string) (File, error) {
	if name == "" {
		return nil, errEmptyFilename
	}

	emptyReader := new(bytes.Buffer)

	name = fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	err := f.conn.Stor(name, emptyReader)
	if err != nil {
		return nil, err
	}

	f.logger.Logf("Created file with path %s", name)

	s := strings.Split(name, "/")

	res, err := f.conn.Retr(name)
	if err != nil {
		return nil, err
	}

	defer res.Close()

	return &ftpFile{
		response: res,
		name:     s[len(s)-1],
		path:     name,
		conn:     f.conn,
	}, nil
}

// Mkdir creates a directory on the FTP server. Here, os.FileMode is unused.
func (f *ftpFileSystem) Mkdir(name string, _ os.FileMode) error {
	if name == "" {
		return errEmptyDirectory
	}

	name = fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	err := f.conn.MakeDir(name)
	if err == nil {
		f.logger.Logf("Created directory with path %s", name)
		return err
	}

	return err
}

// MkdirAll creates directories recursively on the FTP server. Here, os.FileMode is unused.
func (f *ftpFileSystem) MkdirAll(path string, _ os.FileMode) error {
	dirs := strings.Split(path, "/")

	currentDir := dirs[0]

	v := 0

	for i, dir := range dirs {
		// Ignore empty directory names (can happen if there are double slashes).
		if dir == "" {
			continue
		}

		if i == 0 {
			currentDir = dir
		} else {
			currentDir = fmt.Sprintf("%s/%s", currentDir, dir)
		}

		err := f.conn.MakeDir(currentDir)
		if err != nil {
			// if error indicates that directory exists continue, else return.
			if fmt.Sprint(err) != directoryAlreadyExistError {
				continue
			}

			return err
		}

		v++
	}

	if v == 0 {
		return errors.New("Error Creating Directory")
	}

	return nil
}

// Open retrieves a file from the FTP server and returns a file handle.
func (f *ftpFileSystem) Open(name string) (File, error) {
	if name == "" {
		return nil, errEmptyFilename
	}

	name = fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	res, err := f.conn.Retr(name)
	if err != nil {
		return nil, err
	}

	s := strings.Split(name, "/")

	f.logger.Logf("Opened file with path %s", name)

	return &ftpFile{
		response: res,
		name:     s[len(s)-1],
		path:     name,
		conn:     f.conn,
	}, nil
}

// permissions are not clear for Ftp as file commands do not accept an argument and don't store their file permissions.
// currently, this function just calls the Open function. Here, os.FileMode is unused.
func (f *ftpFileSystem) OpenFile(name string, _ int, _ os.FileMode) (File, error) {
	return f.Open(name)
}

// Remove deletes a file from the FTP server.
func (f *ftpFileSystem) Remove(name string) error {
	if name == "" {
		return errEmptyFilename
	}

	name = fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	f.logger.Logf("file with path %s successfully removed", name)

	return f.conn.Delete(name)
}

// RemoveAll removes a directory and its contents recursively from the FTP server.
func (f *ftpFileSystem) RemoveAll(path string) error {
	if path == "" {
		return errEmptyPath
	}

	path = fmt.Sprintf("%s/%s", f.config.RemoteDir, path)

	err := f.conn.RemoveDirRecur(path)
	if err == nil {
		f.logger.Logf("directory with path %s successfully removed.", path)
	}

	return err
}

// Rename renames a file or directory on the FTP server.
func (f *ftpFileSystem) Rename(oldname, newname string) error {
	if oldname == "" || newname == "" {
		return errInvalidArg
	}

	if oldname == newname {
		f.logger.Logf("File has the same name")
		return nil
	}

	oldname = fmt.Sprintf("%s/%s", f.config.RemoteDir, oldname)
	newname = fmt.Sprintf("%s/%s", f.config.RemoteDir, newname)

	err := f.conn.Rename(oldname, newname)
	if err == nil {
		f.logger.Logf("Renamed file %s to %s", oldname, newname)
	}

	return err
}
