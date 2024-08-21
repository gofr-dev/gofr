package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"time"

	"github.com/jlaffaye/ftp"

	file_interface "gofr.dev/pkg/gofr/datasource/file"
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
	Host        string        // FTP server hostname
	User        string        // FTP username
	Password    string        // FTP password
	Port        int           // FTP port
	RemoteDir   string        // Remote directory path. Base Path for all FTP Operations.
	DialTimeout time.Duration // FTP connection timeout
}

// New initializes a new instance of FTP fileSystem with provided configuration.
func New(config *Config) file_interface.FileSystemProvider {
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
	var status, msg string

	ftpServer := fmt.Sprintf("%v:%v", f.config.Host, f.config.Port)

	defer f.processLog(&FileLog{Operation: "Connect", Location: ftpServer, Status: &status, Message: &msg}, time.Now())

	if f.config.DialTimeout == 0 {
		f.config.DialTimeout = time.Second * 5
	}

	conn, err := ftp.Dial(ftpServer, ftp.DialWithTimeout(f.config.DialTimeout))
	if err != nil {
		f.logger.Errorf("Connection failed : %v", err)
		status = "CONNECTION ERROR"
		return
	}

	f.conn = &Conn{conn}

	err = conn.Login(f.config.User, f.config.Password)
	if err != nil {
		f.logger.Errorf("Login failed : %v", err)
		status = "LOGIN ERROR"
		return
	}

	status = "LOGIN SUCCESS"

	f.logger.Logf("Connected to FTP server at %v", ftpServer)
}

// Create creates an empty file on the FTP server.
func (f *fileSystem) Create(name string) (file_interface.File, error) {
	filePath := path.Join(f.config.RemoteDir, name)

	var msg string

	status := "ERROR"

	defer f.processLog(&FileLog{Operation: "Create", Location: filePath, Status: &status, Message: &msg}, time.Now())

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

	defer res.Close()

	status = "SUCCESS"
	msg = fmt.Sprintf("Created file %q", name)

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
	var msg string

	status := "ERROR"

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Mkdir", Location: filePath, Status: &status, Message: &msg}, time.Now())

	if name == "" {
		f.logger.Errorf("Mkdir failed. Provide a valid directory : %v", errEmptyDirectory)
		return errEmptyDirectory
	}

	err := f.conn.MakeDir(filePath)
	if err != nil {
		f.logger.Errorf("Mkdir failed. Error creating directory at %q : %v", filePath, err)
		return err
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("%q created successfully", name)

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
	var msg string

	status := "ERROR"

	defer f.processLog(&FileLog{Operation: "MkdirAll", Location: path.Join(f.config.RemoteDir, name), Status: &status, Message: &msg}, time.Now())

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

	status = "SUCCESS"
	msg = fmt.Sprintf("Directories %q creation completed successfully", name)

	return nil
}

// Note: Here Open and OpenFile both methods have been implemented so that the
// FTP FileSystem comply with the gofr FileSystem interface.
// Open retrieves a file from the FTP server and returns a file handle.
func (f *fileSystem) Open(name string) (file_interface.File, error) {
	var msg string

	status := "ERROR"

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Open", Location: filePath, Status: &status, Message: &msg}, time.Now())

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

	status = "SUCCESS"
	msg = fmt.Sprintf("Opened file %q", name)

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
func (f *fileSystem) OpenFile(name string, _ int, _ os.FileMode) (file_interface.File, error) {
	return f.Open(name)
}

// Remove deletes a file from the FTP server.
// Note: some server may return an error type even if delete is successful
func (f *fileSystem) Remove(name string) error {
	var msg string

	status := "ERROR"

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Remove", Location: filePath, Status: &status, Message: &msg}, time.Now())

	if name == "" {
		f.logger.Errorf("Remove_file failed. Provide a valid filename : %v", errEmptyFilename)
		return errEmptyFilename
	}

	err := f.conn.Delete(filePath)
	if err != nil {
		f.logger.Errorf("Remove_file failed. Error while deleting the file: %v", err)
		return err
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("File with path %q removed successfully", filePath)

	return nil
}

// RemoveAll removes a directory and its contents recursively from the FTP server.
func (f *fileSystem) RemoveAll(name string) error {
	var msg string

	status := "ERROR"

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "RemoveAll", Location: filePath, Status: &status, Message: &msg}, time.Now())

	if name == "" {
		f.logger.Errorf("RemoveAll failed. Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}

	err := f.conn.RemoveDirRecur(filePath)
	if err != nil {
		f.logger.Errorf("RemoveAll failed. Error while deleting directories : %v", err)
		return err
	}

	msg = "Directories deleted successfully"
	status = "SUCCESS"

	return nil
}

// Rename renames a file or directory on the FTP server.
func (f *fileSystem) Rename(oldname, newname string) error {
	var msg string

	status := "ERROR"

	oldFilePath := path.Join(f.config.RemoteDir, oldname)

	newFilePath := path.Join(f.config.RemoteDir, newname)

	defer f.processLog(&FileLog{Operation: "Rename", Location: oldFilePath, Status: &status, Message: &msg}, time.Now())

	if oldname == "" || newname == "" {
		f.logger.Errorf("Provide valid arguments : %v", errInvalidArg)
		return errInvalidArg
	}

	if oldname == newname {
		msg = "File has the same name"
		status = "NO ACTION"
		return nil
	}

	err := f.conn.Rename(oldFilePath, newFilePath)
	if err != nil {
		f.logger.Errorf("Error while renaming file : %v", err)
		return err
	}

	msg = fmt.Sprintf("Renamed file %q to %q", oldname, newname)
	status = "SUCCESS"

	return nil
}

func (f *fileSystem) processLog(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)

	// TODO : Implement metrics
}
