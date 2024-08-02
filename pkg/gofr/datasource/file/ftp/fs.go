package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"net/textproto"
	"os"
	"path"
	"slices"
	"time"

	"gofr.dev/pkg/gofr/container"

	"github.com/jlaffaye/ftp"
)

// Conn struct embeds the *ftp.ServerConn returned by ftp server on successful connection.
type Conn struct {
	*ftp.ServerConn
}

// Retr wraps the ftp retrieve method to return a ftpResponse interface type.
func (c *Conn) Retr(filepath string) (ftpResponse, error) {
	return c.ServerConn.Retr(filepath)
}

func (c *Conn) RetrFrom(filepath string, offset uint64) (ftpResponse, error) {
	return c.ServerConn.RetrFrom(filepath, offset)
}

var (
	errEmptyFilename  = errors.New("filename cannot be empty")
	errEmptyPath      = errors.New("file/directory path cannot be empty")
	errEmptyDirectory = errors.New("directory name cannot be empty")
	errInvalidArg     = errors.New("invalid filename/directory")
)

// fileSystem represents a file system interface over FTP.
type fileSystem struct {
	*file
	conn    ServerConn
	config  *Config
	logger  Logger
	metrics Metrics
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
func New(config *Config) container.FileSystemProvider {
	return &fileSystem{config: config}
}

// UseLogger sets the Logger interface for the FTP file system.
func (f *fileSystem) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the Metrics interface.
func (f *fileSystem) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}

// Connect establishes a connection to the FTP server and logs in.
func (f *fileSystem) Connect() {
	ftpServer := fmt.Sprintf("%v:%v", f.config.Host, f.config.Port)

	defer f.processLog(&FileLog{Operation: "Connect", Location: ftpServer}, time.Now())

	const dialTimeout = 5 * time.Second

	conn, err := ftp.Dial(ftpServer, ftp.DialWithTimeout(dialTimeout))
	if err != nil {
		f.logger.Errorf("Connection failed : %v", err)
		return
	}

	f.conn = &Conn{conn}

	f.logger.Logf("Connected to FTP Server : %v", ftpServer)

	err = conn.Login(f.config.User, f.config.Password)
	if err != nil {
		f.logger.Errorf("Login failed : %v", err)
		return
	}

	f.logger.Logf("Login Successful. Current remote location : %q", f.config.RemoteDir)
}

// Create creates an empty file on the FTP server.
func (f *fileSystem) Create(name string) (container.File, error) {
	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Create", Location: filePath}, time.Now())

	if name == "" {
		f.logger.Errorf("Create_File failed. Provide a valid filename : %v", errEmptyFilename)
		return nil, errEmptyFilename
	}

	emptyReader := new(bytes.Buffer)

	err := f.conn.Stor(filePath, emptyReader)
	if err != nil {
		f.logger.Errorf("Create_File failed. Error creating file with path %q : %v", filePath, err)
		return nil, err
	}

	filename := path.Base(filePath)

	res, err := f.conn.Retr(filePath)
	if err != nil {
		f.logger.Errorf("Create_File failed : %v", err)
		return nil, err
	}

	f.logger.Logf("Create_File successful. Created file %s at %q", name, filePath)

	defer res.Close()

	return &file{
		response: res,
		name:     filename,
		path:     filePath,
		conn:     f.conn,
		logger:   f.logger,
		metrics:  f.metrics,
	}, nil
}

// Mkdir creates a directory on the FTP server.
// Here, os.FileMode is unused, but is added to comply with FileSystem interface.
func (f *fileSystem) Mkdir(name string, _ os.FileMode) error {
	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Mkdir", Location: filePath}, time.Now())

	if name == "" {
		f.logger.Errorf("Mkdir failed. Provide a valid directory : %v", errEmptyDirectory)
		return errEmptyDirectory
	}

	err := f.conn.MakeDir(filePath)
	if err != nil {
		f.logger.Errorf("Mkdir failed. Error creating directory at %q : %v", filePath, err)
		return err
	}

	f.logger.Logf("%s successfully created", name)

	return nil
}

func (f *fileSystem) mkdirAllHelper(filepath string) []string {
	var dirs []string

	currentdir := filepath

	for {
		err := f.conn.MakeDir(currentdir)
		if err == nil {
			dirs = append(dirs, currentdir)
			break
		}

		parentDir, dir := path.Split(currentdir)

		dirs = append(dirs, dir)

		if parentDir == "" || parentDir == "/" {
			break
		}

		currentdir = path.Clean(parentDir)
	}

	slices.Reverse(dirs)

	return dirs
}

// MkdirAll creates directories recursively on the FTP server. Here, os.FileMode is unused.
// Here, os.FileMode is unused, but is added to comply with FileSystem interface.
func (f *fileSystem) MkdirAll(name string, _ os.FileMode) error {
	defer f.processLog(&FileLog{Operation: "MkdirAll", Location: path.Join(f.config.RemoteDir, name)}, time.Now())

	if name == "" {
		f.logger.Errorf("MkdirAll failed. Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}
	// returns a slice of those directories that do not exist with the first index being the latest existing parent directory path.
	dirs := f.mkdirAllHelper(name)

	currentDir := dirs[0]

	for i, dir := range dirs {
		if i == 0 {
			continue
		}

		currentDir = path.Join(currentDir, dir)

		err := f.conn.MakeDir(currentDir)
		if err != nil {
			f.logger.Errorf("MkdirAll failed : %v", err)

			return err
		}
	}

	f.logger.Logf("Directories creation completed successfully.")

	return nil
}

// Open retrieves a file from the FTP server and returns a file handle.
func (f *fileSystem) Open(name string) (container.File, error) {
	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Open", Location: filePath}, time.Now())

	if name == "" {
		f.logger.Errorf("Open_file failed. Provide a valid filename : %v", errEmptyFilename)
		return nil, errEmptyFilename
	}

	res, err := f.conn.Retr(filePath)
	if err != nil {
		f.logger.Errorf("Open_file failed. Error opening file : %v", err)
		return nil, err
	}

	defer res.Close()

	filename := path.Base(filePath)

	f.logger.Logf("Open_file successful. Filepath : %q", filePath)

	return &file{
		response: res,
		name:     filename,
		path:     filePath,
		conn:     f.conn,
		logger:   f.logger,
		metrics:  f.metrics,
	}, nil
}

// permissions are not clear for Ftp as file commands do not accept an argument and don't store their file permissions.
// currently, this function just calls the Open function.
// Here, os.FileMode is unused, but is added to comply with FileSystem interface.
func (f *fileSystem) OpenFile(name string, _ int, _ os.FileMode) (container.File, error) {
	return f.Open(name)
}

// Remove deletes a file from the FTP server.
func (f *fileSystem) Remove(name string) error {
	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Remove", Location: filePath}, time.Now())

	if name == "" {
		f.logger.Errorf("Remove_file failed. Provide a valid filename : %v", errEmptyFilename)
		return errEmptyFilename
	}

	err := f.conn.Delete(filePath)

	var textprotoError *textproto.Error
	switch {
	case errors.As(err, &textprotoError) && textprotoError.Code == ftp.StatusClosingDataConnection && textprotoError.Msg == "Transfer complete.":

	case err != nil:
		f.logger.Errorf("Remove_file failed. Error while deleting the file: %v", err)
		return err
	}

	f.logger.Logf("Remove_file success. File with path %q successfully removed", filePath)

	return nil
}

// RemoveAll removes a directory and its contents recursively from the FTP server.
func (f *fileSystem) RemoveAll(name string) error {
	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "RemoveAll", Location: filePath}, time.Now())

	if name == "" {
		f.logger.Errorf("RemoveAll failed. Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}

	err := f.conn.RemoveDirRecur(filePath)
	if err != nil {
		f.logger.Errorf("RemoveAll failed. Error while deleting directories : %v", err)
		return err
	}

	f.logger.Logf("Directories on path %q successfully deleted", filePath)

	return nil
}

// Rename renames a file or directory on the FTP server.
func (f *fileSystem) Rename(oldname, newname string) error {
	oldFilePath := path.Join(f.config.RemoteDir, oldname)

	newFilePath := path.Join(f.config.RemoteDir, newname)

	defer f.processLog(&FileLog{Operation: "Rename", Location: oldFilePath}, time.Now())

	if oldname == "" || newname == "" {
		f.logger.Errorf("Provide valid arguments : %v", errInvalidArg)
		return errInvalidArg
	}

	if oldname == newname {
		f.logger.Logf("File has the same name")
		return nil
	}

	err := f.conn.Rename(oldFilePath, newFilePath)
	if err != nil {
		f.logger.Errorf("Error while renaming file : %v", err)
		return err
	}

	f.logger.Logf("Renamed file %q to %q", oldname, newname)

	return nil
}

func (f *fileSystem) processLog(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debugf("%v", fl)
}
