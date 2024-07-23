package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/datasource"

	"github.com/jlaffaye/ftp"
)

// ftpFileSystem represents a file system interface over FTP.
type ftpFileSystem struct {
	file   *ftpFile          // Pointer to a single file object (not used in the provided methods)
	conn   ServerConn        // FTP server connection
	config *Config           // FTP configuration
	logger datasource.Logger // Logger interface for logging
}

// UseLogger sets the logger interface for the FTP file system.
func (f *ftpFileSystem) UseLogger(logger interface{}) {
	f.logger = logger.(datasource.Logger)
}

// Config represents the FTP configuration.
type Config struct {
	Host      string // FTP server hostname
	User      string // FTP username
	Password  string // FTP password
	Port      string // FTP port
	RemoteDir string // Remote directory path
}

// New initializes a new instance of ftpFileSystem with provided configuration.
func New(config *Config) datasource.FileSystem {
	return &ftpFileSystem{config: config}
}

// UseMetrics sets the metrics for the ftpFileSystem client which asserts the Metrics interface.
// Currently not implemented.
func (f *ftpFileSystem) UseMetrics(metrics interface{}) {

}

// Connect establishes a connection to the FTP server and logs in.
func (f *ftpFileSystem) Connect() {
	// Construct FTP server address
	ftpServer := fmt.Sprintf("%v:%v", f.config.Host, f.config.Port)

	// Connect to FTP server
	const dialTimeoutSeconds = 5

	conn, err := ftp.Dial(ftpServer, ftp.DialWithTimeout(dialTimeoutSeconds*time.Second))
	if err != nil {
		log.Printf("Failed to connect to FTP server: %v", err)
		return
	}

	f.conn = &Conn{conn}

	// Login to FTP server
	err = conn.Login(f.config.User, f.config.Password)
	if err != nil {
		log.Printf("Failed to login: %v", err)
	} else {
		log.Printf("Login Successful")
	}
}

// Create creates an empty file on the FTP server.
func (f *ftpFileSystem) Create(name string) (datasource.File, error) {
	// empty io.Reader
	emptyReader := new(bytes.Buffer)

	if name == "" {
		return nil, errors.New("empty filename")
	}

	// construct the path
	name = fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	// Issue STOR command to create the empty file on FTP server
	err := f.conn.Stor(name, emptyReader)
	if err != nil {
		return nil, err
	}

	log.Printf("Created file %s", name)

	s := strings.Split(name, "/")

	// Retrieve the file from FTP server and return a file handle
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

// Mkdir creates a directory on the FTP server.
func (f *ftpFileSystem) Mkdir(name string, _ os.FileMode) error {
	if name == "" {
		return errors.New("empty directory name")
	}

	name = fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	return f.conn.MakeDir(name)
}

// MkdirAll creates directories recursively on the FTP server.
func (f *ftpFileSystem) MkdirAll(path string, _ os.FileMode) error {
	// Split path into individual directory names
	dirs := strings.Split(path, "/")

	// Start creating directories
	currentDir := ""

	for _, dir := range dirs {
		// Ignore empty directory names (can happen if there are double slashes)
		if dir == "" {
			continue
		}

		// Build the full path up to this directory
		currentDir = filepath.Join(currentDir, dir)

		// Attempt to create the directory
		err := f.conn.MakeDir(currentDir)
		if err != nil {
			continue // Continue creating next directory on failure
		}
	}

	return nil
}

// Open retrieves a file from the FTP server and returns a file handle.
func (f *ftpFileSystem) Open(name string) (datasource.File, error) {
	if name == "" {
		return nil, errors.New("empty filename")
	}

	name = fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	// Retrieve the file from FTP server and return a file handle
	res, err := f.conn.Retr(name)
	if err != nil {
		return nil, err
	}

	s := strings.Split(name, "/")

	// Return the file handle
	return &ftpFile{
		response: res,
		name:     s[len(s)-1],
		path:     name,
		conn:     f.conn,
	}, nil
}

// OpenFile retrieves a file from the FTP server with specified flags and permissions, and returns a file handle.
// permissions are not clear for Ftp as file commands do not accept an argument and don't store their file permissions.
// currently, this function just calls the Open function.
func (f *ftpFileSystem) OpenFile(name string, _ int, _ os.FileMode) (datasource.File, error) {
	return f.Open(name)
}

// Remove deletes a file from the FTP server.
func (f *ftpFileSystem) Remove(name string) error {
	if name == "" {
		return errors.New("empty filename")
	}

	name = fmt.Sprintf("%s/%s", f.config.RemoteDir, name)

	return f.conn.Delete(name)
}

// RemoveAll removes a directory and its contents recursively from the FTP server.
func (f *ftpFileSystem) RemoveAll(path string) error {
	if path == "" {
		return errors.New("empty path")
	}

	path = fmt.Sprintf("%s/%s", f.config.RemoteDir, path)

	return f.conn.RemoveDirRecur(path)
}

// Rename renames a file or directory on the FTP server.
func (f *ftpFileSystem) Rename(oldname, newname string) error {
	if oldname == "" || newname == "" {
		return errors.New("invalid filename/directory")
	}

	// construct the path
	oldname = fmt.Sprintf("%s/%s", f.config.RemoteDir, oldname)
	newname = fmt.Sprintf("%s/%s", f.config.RemoteDir, newname)

	return f.conn.Rename(oldname, newname)
}
