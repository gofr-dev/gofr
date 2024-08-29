package sftp

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/sftp"

	"golang.org/x/crypto/ssh"

	File "gofr.dev/pkg/gofr/datasource/file"
)

type fileSystem struct {
	logger  Logger
	metrics Metrics
	config  Config
	client  sftpClient
}

type Config struct {
	User            string
	Password        string
	Host            string
	Port            int
	HostKeyCallBack ssh.HostKeyCallback
}

func New(cfg Config) *fileSystem {
	return &fileSystem{config: cfg}
}

// UseLogger sets the logger for the FileSystem client.
func (f *fileSystem) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the metrics for the FileSystem client.
func (f *fileSystem) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}

// Connect establishes a connection to FileSystem and registers metrics using the provided configuration when the client was Created.
func (f *fileSystem) Connect() {
	f.logger.Debugf("connecting to SFTP server with host `%v` and port `%v`", f.config.Host, f.config.Port)

	addr := fmt.Sprintf("%s:%d", f.config.Host, f.config.Port)

	config := &ssh.ClientConfig{
		User:            f.config.User,
		Auth:            []ssh.AuthMethod{ssh.Password(f.config.Password)},
		HostKeyCallback: f.config.HostKeyCallBack,
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		f.logger.Errorf("failed to connect with sftp with err %v", err)
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		f.logger.Errorf("failed to create sftp client with err %v", err)
	}

	f.client = client

	f.logger.Logf("connected to SFTP client successfully")
}

func (f *fileSystem) Create(name string) (File.File, error) {
	status := "ERROR"

	defer f.sendOperationStats(&FileLog{
		Operation: "CREATE",
		Location:  name,
		Status:    &status,
	}, time.Now())

	newFile, err := f.client.Create(name)
	if err != nil {
		return nil, err
	}

	status = "SUCCESS"

	return sftpFile{
		File:   newFile,
		logger: f.logger,
	}, nil
}

func (f *fileSystem) Mkdir(name string, _ os.FileMode) error {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "MKDIR", Location: name, Status: &status}, time.Now())

	err := f.client.Mkdir(name)
	if err != nil {
		status = "ERROR"
		return err
	}

	return nil
}

func (f *fileSystem) MkdirAll(path string, perm os.FileMode) error {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "MKDIR", Location: path, Status: &status}, time.Now())

	err := f.client.MkdirAll(path)
	if err != nil {
		status = "ERROR"
		return err
	}

	return nil
}

func (f *fileSystem) Open(name string) (File.File, error) {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "OPEN", Location: name, Status: &status}, time.Now())

	openedFile, err := f.client.Open(name)
	if err != nil {
		status = "ERROR"

		return nil, err
	}

	return sftpFile{
		File:   openedFile,
		logger: f.logger,
	}, nil
}

func (f *fileSystem) OpenFile(name string, flag int, perm os.FileMode) (File.File, error) {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "OPENFILE", Location: name, Status: &status}, time.Now())

	openedFile, err := f.client.OpenFile(name, flag)
	if err != nil {
		status = "ERROR"

		return nil, err
	}

	return sftpFile{
		File:   openedFile,
		logger: f.logger,
	}, nil
}

func (f *fileSystem) Remove(name string) error {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "REMOVE", Location: name, Status: &status}, time.Now())

	err := f.client.Remove(name)
	if err != nil {
		status = "ERROR"
		return err
	}

	return nil
}

func (f *fileSystem) RemoveAll(path string) error {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "REMOVEALL", Location: path, Status: &status}, time.Now())

	err := f.client.RemoveAll(path)
	if err != nil {
		status = "ERROR"
		return err
	}

	return nil
}

func (f *fileSystem) Rename(oldname, newname string) error {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "RENAME", Location: fmt.Sprintf("%v to %v", oldname, newname),
		Status: &status}, time.Now())

	err := f.client.Rename(oldname, newname)
	if err != nil {
		status = "ERROR"
		return err
	}

	return nil
}

func (f *fileSystem) ReadDir(dir string) ([]File.FileInfo, error) {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "READDIR", Location: dir, Status: &status}, time.Now())

	dirs, err := f.client.ReadDir(dir)
	if err != nil {
		status = "ERROR"
		return nil, err
	}

	newDirs := make([]File.FileInfo, 0, len(dirs))

	for _, v := range dirs {
		newDirs = append(newDirs, v)
	}

	return newDirs, nil
}

func (f *fileSystem) Stat(name string) (File.FileInfo, error) {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "STAT", Location: name, Status: &status}, time.Now())

	fileInfo, err := f.client.Stat(name)
	if err != nil {
		status = "ERROR"
		return nil, err
	}

	return fileInfo, nil
}

func (f *fileSystem) ChDir(dirname string) error {
	f.logger.Errorf("Chdir is not implemented for SFTP")
	return nil
}

func (f *fileSystem) Getwd() (string, error) {
	status := "SUCCESS"

	defer f.sendOperationStats(&FileLog{Operation: "STAT", Location: "", Status: &status}, time.Now())

	name, err := f.client.Getwd()
	if err != nil {
		status = "ERROR"
		return "", err
	}

	return name, err
}

func (f *fileSystem) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)

	// TODO : Implement metrics
}
