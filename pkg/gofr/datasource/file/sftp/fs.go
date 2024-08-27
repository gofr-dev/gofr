package sftp

import (
	"fmt"
	"os"

	"github.com/pkg/sftp"

	"golang.org/x/crypto/ssh"

	File "gofr.dev/pkg/gofr/datasource/file"
)

type files struct {
	logger  Logger
	metrics Metrics
	config  Config
	client  sftpClient
}

type Config struct {
	User     string
	Password string
	Host     string
	Port     int
}

func New(cfg Config) *files {
	return &files{config: cfg}
}

// UseLogger sets the logger for the FileSystem client.
func (f *files) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the metrics for the FileSystem client.
func (f *files) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}

// Connect establishes a connection to FileSystem and registers metrics using the provided configuration when the client was Created.
func (f *files) Connect() {
	addr := fmt.Sprintf("%s:%d", f.config.Host, f.config.Port)

	config := &ssh.ClientConfig{
		User:            f.config.User,
		Auth:            []ssh.AuthMethod{ssh.Password(f.config.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // using InsecureIgnoreHostKey to accept any host key
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
}

func (f *files) Create(name string) (File.File, error) {
	newFile, err := f.client.Create(name)
	if err != nil {
		return nil, err
	}

	return file{
		File:   newFile,
		logger: f.logger,
	}, nil
}

func (f *files) Mkdir(name string, perm os.FileMode) error {
	return f.client.Mkdir(name)
}

func (f *files) MkdirAll(path string, perm os.FileMode) error {
	return f.client.MkdirAll(path)
}

func (f *files) Open(name string) (File.File, error) {
	openedFile, err := f.client.Open(name)
	if err != nil {
		return nil, err
	}

	return file{
		File:   openedFile,
		logger: f.logger,
	}, nil
}

func (f *files) OpenFile(name string, flag int, perm os.FileMode) (File.File, error) {
	openedFile, err := f.client.OpenFile(name, flag)
	if err != nil {
		return nil, err
	}

	return file{
		File:   openedFile,
		logger: f.logger,
	}, nil
}

func (f *files) Remove(name string) error {
	return f.client.Remove(name)
}

func (f *files) RemoveAll(path string) error {
	return f.client.RemoveAll(path)
}

func (f *files) Rename(oldname, newname string) error {
	return f.client.Rename(oldname, newname)
}

func (f *files) ReadDir(dir string) ([]File.FileInfo, error) {
	dirs, err := f.client.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	newDirs := make([]File.FileInfo, 0, len(dirs))

	for _, v := range dirs {
		newDirs = append(newDirs, v)
	}

	return newDirs, nil
}

func (f *files) Stat(name string) (File.FileInfo, error) {
	return f.client.Stat(name)
}

func (f *files) ChDir(dirname string) error {
	f.logger.Errorf("Chdir is not implemented for SFTP")
	return nil
}

func (f *files) Getwd() (string, error) {
	return f.client.Getwd()
}
