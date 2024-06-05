package file

import (
	"gofr.dev/pkg/gofr/datasource"
	"os"
)

type fileSystem struct {
}

func New() datasource.FileSystem {
	return fileSystem{}
}

func (f fileSystem) Create(name string) (datasource.File, error) {
	newFile, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	return &file{File: newFile}, nil
}

func (f fileSystem) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

func (f fileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (f fileSystem) Open(name string) (datasource.File, error) {
	openFile, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	return &file{File: openFile}, nil
}

func (f fileSystem) OpenFile(name string, flag int, perm os.FileMode) (datasource.File, error) {
	openFile, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}

	return &file{File: openFile}, nil
}

func (f fileSystem) Remove(name string) error {
	return os.Remove(name)
}

func (f fileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (f fileSystem) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}
