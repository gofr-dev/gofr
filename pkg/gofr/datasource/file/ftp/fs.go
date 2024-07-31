package ftp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/textproto"
	"os"
	"path"
	"time"

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
	errEmptyFilename            = errors.New("filename cannot be empty")
	errEmptyPath                = errors.New("file/directory path cannot be empty")
	errEmptyDirectory           = errors.New("directory name cannot be empty")
	errInvalidArg               = errors.New("invalid filename/directory")
	transferCompleteError       = &textproto.Error{Code: 226, Msg: "Transfer complete."}
	directoryAlreadyExistsError = &textproto.Error{Code: 550, Msg: "Create directory operation failed."}
)

// ftpFileSystem represents a file system interface over FTP.
type ftpFileSystem struct {
	*ftpFile
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
func New(config *Config) FileSystemProvider {
	return &ftpFileSystem{config: config}
}

// UseLogger sets the logger interface for the FTP file system.
func (f *ftpFileSystem) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the metrics for the MongoDB client which asserts the Metrics interface.
func (f *ftpFileSystem) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}

// Connect establishes a connection to the FTP server and logs in.
func (f *ftpFileSystem) Connect() {
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
func (f *ftpFileSystem) Create(name string) (File, error) {
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

	_, s := path.Split(filePath)

	res, err := f.conn.Retr(filePath)
	if err != nil {
		f.logger.Errorf("Create_File failed : %v", err)
		return nil, err
	}

	f.logger.Logf("Create_File successful. Created file %s at %q", name, filePath)

	defer res.Close()

	return &ftpFile{
		response: res,
		name:     s,
		path:     filePath,
		conn:     f.conn,
		logger:   f.logger,
		metrics:  f.metrics,
	}, nil
}

// Mkdir creates a directory on the FTP server. Here, os.FileMode is unused.
func (f *ftpFileSystem) Mkdir(name string, _ os.FileMode) error {
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

func (f *ftpFileSystem) mkdirAllHelper(filepath string) []string {
	var dirs []string

	currentdir := filepath

	for {
		err := f.conn.MakeDir(currentdir)
		if err != nil {
			parentDir, dir := path.Split(currentdir)

			dirs = append([]string{dir}, dirs...)
			if parentDir == "" || parentDir == "/" {
				break
			}
			currentdir = path.Clean(parentDir)
		} else {
			dirs = append([]string{currentdir}, dirs...)
			break
		}

	}

	return dirs
}

// MkdirAll creates directories recursively on the FTP server. Here, os.FileMode is unused.
// The directories are not created if even one directory exist.
func (f *ftpFileSystem) MkdirAll(name string, _ os.FileMode) error {
	defer f.processLog(&FileLog{Operation: "MkdirAll", Location: path.Join(f.config.RemoteDir, name)}, time.Now())

	if name == "" {
		f.logger.Errorf("MkdirAll failed. Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}

	dirs := f.mkdirAllHelper(name)

	currentDir := dirs[0]

	for i, dir := range dirs {
		if i == 0 {
			continue
		} else {
			currentDir = path.Join(currentDir, dir)
		}

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
func (f *ftpFileSystem) Open(name string) (File, error) {
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

	_, s := path.Split(filePath)

	f.logger.Logf("Open_file successful. Filepath : %q", filePath)

	defer res.Close()

	return &ftpFile{
		response: res,
		name:     s,
		path:     filePath,
		conn:     f.conn,
		logger:   f.logger,
		metrics:  f.metrics,
	}, nil
}

// permissions are not clear for Ftp as file commands do not accept an argument and don't store their file permissions.
// currently, this function just calls the Open function. Here, os.FileMode is unused.
func (f *ftpFileSystem) OpenFile(name string, _ int, _ os.FileMode) (File, error) {
	return f.Open(name)
}

// Remove deletes a file from the FTP server.
func (f *ftpFileSystem) Remove(name string) error {
	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Remove", Location: filePath}, time.Now())

	if name == "" {
		f.logger.Errorf("Remove_file failed. Provide a valid filename : %v", errEmptyFilename)
		return errEmptyFilename
	}

	err := f.conn.Delete(filePath)

	if err != nil {
		var textprotoError *textproto.Error

		if errors.As(err, &textprotoError) && textprotoError.Msg == transferCompleteError.Msg {
			f.logger.Logf("Remove_file successful. File with path %q successfully removed", filePath)
			return nil
		}

		f.logger.Errorf("Remove_file failed. Error while deleting the file: %v", err)
		return err
	}

	f.logger.Logf("Remove_file success. File with path %q successfully removed", filePath)
	return nil
}

// RemoveAll removes a directory and its contents recursively from the FTP server.
func (f *ftpFileSystem) RemoveAll(name string) error {
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
func (f *ftpFileSystem) Rename(oldname, newname string) error {
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

func (f *ftpFileSystem) processLog(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debugf("%v", fl)

	f.metrics.RecordHistogram(context.Background(), "app_ftp_stats", float64(duration), "hostname", fmt.Sprintf("%v:%v", f.config.Host, f.config.Port),
		"remote directory", f.config.RemoteDir, "type", fl.Operation)
}
