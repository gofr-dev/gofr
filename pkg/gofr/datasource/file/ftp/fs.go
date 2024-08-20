package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/jlaffaye/ftp"

	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

// Conn struct embeds the *ftp.ServerConn returned by ftp server on successful connection.
type Conn struct {
	*ftp.ServerConn
}

type Entry *ftp.Entry

// Retr wraps the ftp retrieve method to return a ftpResponse interface type.
func (c *Conn) Retr(filepath string) (ftpResponse, error) {
	return c.ServerConn.Retr(filepath)
}

func (c *Conn) RetrFrom(filepath string, offset uint64) (ftpResponse, error) {
	return c.ServerConn.RetrFrom(filepath, offset)
}

// List wraps the ftp list method to return a locally defined Entry type.
func (c *Conn) List(filepath string) ([]Entry, error) {
	res, err := c.ServerConn.List(filepath)

	var entries []Entry
	for _, entry := range res {
		entries = append(entries, entry)
	}

	return entries, err
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
	var status string

	ftpServer := fmt.Sprintf("%v:%v", f.config.Host, f.config.Port)

	defer f.processLog(&FileLog{
		Operation: "Connect",
		Location:  ftpServer,
		Status:    &status,
	}, time.Now())

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

	var fl = &file{}

	status := "ERROR"

	defer f.processLog(&FileLog{
		Operation: "Create",
		Location:  filePath,
		Status:    &status,
		Message:   &msg,
	}, fl.modTime)

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

	fl = &file{
		response:  res,
		name:      filename,
		path:      filePath,
		entryType: ftp.EntryTypeFile,
		conn:      f.conn,
		logger:    f.logger,
		metrics:   f.metrics,
	}

	mt := fl.ModTime()
	if !mt.IsZero() {
		fl.modTime = mt
	}

	return fl, nil
}

// Open retrieves a file from the FTP server and returns a file handle.
// Note: Here Open and OpenFile both methods have been implemented so that the
// FTP FileSystem comply with the gofr FileSystem interface.
func (f *fileSystem) Open(name string) (file_interface.File, error) {
	var msg string

	status := "ERROR"

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{
		Operation: "Open",
		Location:  filePath,
		Status:    &status,
		Message:   &msg,
	}, time.Now())

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

	fl := &file{
		response:  res,
		name:      filename,
		path:      filePath,
		entryType: ftp.EntryTypeFile,
		conn:      f.conn,
		logger:    f.logger,
		metrics:   f.metrics,
	}

	mt := fl.ModTime()
	if !mt.IsZero() {
		fl.modTime = mt
	}

	return fl, nil
}

// OpenFile retrieves a file from the FTP server and returns a file handle.
// Permissions are not clear for Ftp as file commands do not accept an argument and don't store their file permissions.
// currently, this function just calls the Open function.
// Here, os.FileMode is unused, but is added to comply with FileSystem interface.
func (f *fileSystem) OpenFile(name string, _ int, _ os.FileMode) (file_interface.File, error) {
	return f.Open(name)
}

// Remove deletes a file from the FTP server.
// Note: some server may return an error type even if delete is successful.
func (f *fileSystem) Remove(name string) error {
	var msg string

	status := "ERROR"

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{
		Operation: "Remove",
		Location:  filePath,
		Status:    &status,
		Message:   &msg},
		time.Now())

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

// Rename renames a file/directory on the FTP server.
func (f *fileSystem) Rename(oldname, newname string) error {
	var msg string

	var tempFile = &file{conn: f.conn, logger: f.logger, metrics: f.metrics}

	status := "ERROR"

	oldFilePath := path.Join(f.config.RemoteDir, oldname)

	newFilePath := path.Join(f.config.RemoteDir, newname)

	defer f.processLog(&FileLog{
		Operation: "Rename",
		Location:  oldFilePath,
		Status:    &status,
		Message:   &msg,
	}, tempFile.modTime)

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
	tempFile.path = newFilePath

	mt := tempFile.ModTime()
	if !mt.IsZero() {
		tempFile.modTime = mt
	}

	return nil
}

func (f *fileSystem) processLog(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
