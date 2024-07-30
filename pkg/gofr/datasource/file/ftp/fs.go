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

// Conn struct embeds the *ftp.ServerConn returned by ftp server on successful connection.
type Conn struct {
	*ftp.ServerConn
}

// Retr wraps the ftp retrieve method to return a ftpResponse interface type.
func (c *Conn) Retr(path string) (ftpResponse, error) {
	return c.ServerConn.Retr(path)
}

func (c *Conn) RetrFrom(path string, offset uint64) (ftpResponse, error) {
	return c.ServerConn.RetrFrom(path, offset)
}

var (
	errEmptyFilename  = errors.New("filename cannot be empty")
	errEmptyPath      = errors.New("file/directory path cannot be empty")
	errEmptyDirectory = errors.New("directory name cannot be empty")
	errInvalidArg     = errors.New("invalid filename/directory")
	transferComplete  = "226 Transfer complete."
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
func New(config *Config) FileSystemProvider {
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
		f.logger.Errorf("Connection Failed Error: %v", err)
		return
	}

	f.conn = &Conn{conn}

	f.logger.Logf("Connection Success Connected to FTP Server : %v", ftpServer)

	err = conn.Login(f.config.User, f.config.Password)
	if err != nil {
		f.logger.Errorf("Login Failed Error: %v", err)
		return
	}

	f.logger.Logf("Login Success Current remote location: %q", f.config.RemoteDir)
}

// Create creates an empty file on the FTP server.
func (f *ftpFileSystem) Create(name string) (File, error) {
	if name == "" {
		f.logger.Errorf("Create_File Failed Provide a valid filename: %v", errEmptyFilename)
		return nil, errEmptyFilename
	}

	emptyReader := new(bytes.Buffer)

	filePath := fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	err := f.conn.Stor(filePath, emptyReader)
	if err != nil {
		f.logger.Errorf("Create_File Failed Error creating file  with path %q : %v", filePath, err)
		return nil, err
	}

	s := strings.Split(filePath, "/")

	res, err := f.conn.Retr(filePath)
	if err != nil {
		f.logger.Errorf("Create_File Failed Error : %v", err)
		return nil, err
	}

	f.logger.Logf("Create_File Success Successfully created file %s at %q", name, filePath)

	defer res.Close()

	return &ftpFile{
		response: res,
		name:     s[len(s)-1],
		path:     filePath,
		conn:     f.conn,
		logger:   f.logger,
	}, nil
}

// Mkdir creates a directory on the FTP server. Here, os.FileMode is unused.
func (f *ftpFileSystem) Mkdir(name string, _ os.FileMode) error {
	if name == "" {
		f.logger.Errorf("Mkdir Failed Provide a valid directory : %v", errEmptyDirectory)
		return errEmptyDirectory
	}

	filePath := fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	err := f.conn.MakeDir(filePath)
	if err != nil {
		f.logger.Errorf("Mkdir Failed Error creating directory at %q : %v", filePath, err)
		return err
	}

	f.logger.Logf("Mkdir Success %s successfully created", name)

	return nil
}

// MkdirAll creates directories recursively on the FTP server. Here, os.FileMode is unused.
// The directories are not created if even one directory exist.
func (f *ftpFileSystem) MkdirAll(path string, _ os.FileMode) error {
	if path == "" {
		f.logger.Errorf("MkdirAll Failed Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}

	dirs := strings.Split(path, "/")

	currentDir := dirs[0]

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
			f.logger.Errorf("MkdirAll Failed Error : %v", err)
			return err
		}
	}

	f.logger.Logf("MkdirAll Success Directories creation completed successfully.")

	return nil
}

// Open retrieves a file from the FTP server and returns a file handle.
func (f *ftpFileSystem) Open(name string) (File, error) {
	if name == "" {
		f.logger.Errorf("Open_file Failed Provide a valid filename : %v", errEmptyFilename)
		return nil, errEmptyFilename
	}

	filePath := fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	res, err := f.conn.Retr(filePath)
	if err != nil {
		f.logger.Errorf("Open_file Failed Error opening file : %v", err)
		return nil, err
	}

	s := strings.Split(filePath, "/")

	f.logger.Logf("Open_file Success Filepath : %q", filePath)

	defer res.Close()

	return &ftpFile{
		response: res,
		name:     s[len(s)-1],
		path:     filePath,
		conn:     f.conn,
		logger:   f.logger,
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
		f.logger.Errorf("Remove_file Failed Provide a valid filename : %v", errEmptyFilename)
		return errEmptyFilename
	}

	filePath := fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	err := f.conn.Delete(filePath)
	if err != nil && fmt.Sprint(err) != transferComplete {
		f.logger.Errorf("Remove_file Failed Error while deleting the file : %v", err)
		return err
	}

	f.logger.Logf("Remove_file Success File with path %q successfully removed", filePath)

	return nil
}

// RemoveAll removes a directory and its contents recursively from the FTP server.
func (f *ftpFileSystem) RemoveAll(path string) error {
	if path == "" {
		f.logger.Errorf("RemoveAll Failed Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}

	filePath := fmt.Sprintf("%s/%s", f.config.RemoteDir, path)

	err := f.conn.RemoveDirRecur(filePath)
	if err != nil {
		f.logger.Errorf("RemoveAll Failed Error while deleting directories : %v", err)
		return err
	}

	f.logger.Logf("RemoveAll Success Directories on path %q successfully deleted", filePath)

	return nil
}

// Rename renames a file or directory on the FTP server.
func (f *ftpFileSystem) Rename(oldname, newname string) error {
	if oldname == "" || newname == "" {
		f.logger.Errorf("Rename Failed Provide valid arguments : %v", errInvalidArg)
		return errInvalidArg
	}

	if oldname == newname {
		f.logger.Logf("Rename No Action File has the same name")
		return nil
	}

	oldFilePath := fmt.Sprintf("%s/%s", f.config.RemoteDir, oldname)
	newFilePath := fmt.Sprintf("%s/%s", f.config.RemoteDir, newname)

	err := f.conn.Rename(oldFilePath, newFilePath)
	if err != nil {
		f.logger.Errorf("Rename Failed Error while renaming file : %v", err)
		return err
	}

	f.logger.Logf("Rename Success Renamed file %q to %q", oldname, newname)

	return nil
}
