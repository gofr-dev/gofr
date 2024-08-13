package file

import (
	"os"

	"gofr.dev/pkg/gofr/datasource"
)

type fileSystem struct {
	logger datasource.Logger
}

// New initializes local filesystem with logger.
func New(logger datasource.Logger) FileSystem {
	return fileSystem{logger: logger}
}

func (f fileSystem) Create(name string) (File, error) {
	newFile, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	return &file{File: newFile, logger: f.logger}, nil
}

func (fileSystem) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

func (fileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (f fileSystem) Open(name string) (File, error) {
	openFile, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	return &file{File: openFile, logger: f.logger}, nil
}

func (f fileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	openFile, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}

	return &file{File: openFile, logger: f.logger}, nil
}

func (fileSystem) Remove(name string) error {
	return os.Remove(name)
}

func (fileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (fileSystem) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

func (fileSystem) Stat(name string) (FileInfo, error) {
	return os.Stat(name)
}

func (fileSystem) CurrentDir() (string, error) {
	return os.Getwd()
}

func (fileSystem) ChangeDir(dir string) error {
	return os.Chdir(dir)
}

func (fileSystem) ReadDir(dir string) ([]FileInfo, error) {
	entries, err := os.ReadDir(dir)

	fileInfo := make([]FileInfo, len(entries))
	for i := range entries {
		fileInfo[i], _ = entries[i].Info()
	}

	return fileInfo, err
}
