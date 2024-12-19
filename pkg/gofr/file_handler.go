package gofr

import (
	"errors"
	"os"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
)

// Developer Note: fileSysWrapper wraps a datasource.FileSystem to return file.File instead of an interface,
// this is useful when we want to use the file.File methods, but don't want the external datasource
// to be dependent on any particular version of gofr so that they can be maintained independently.
type fileSysWrapper struct {
	datasource.FileSystem
}

func (fsW *fileSysWrapper) Create(name string) (file.File, error) {
	fI, err := fsW.FileSystem.Create(name)

	f, ok := fI.(file.File)
	if !ok {
		return nil, errors.New("failed to create file")
	}

	return f, err
}

func (fsW *fileSysWrapper) Open(name string) (file.File, error) {
	fI, err := fsW.FileSystem.Open(name)

	f, ok := fI.(file.File)
	if !ok {
		return nil, errors.New("failed to open file")
	}

	return f, err
}

func (fsW *fileSysWrapper) OpenFile(name string, flag int, perm os.FileMode) (file.File, error) {
	fI, err := fsW.FileSystem.OpenFile(name, flag, perm)

	f, ok := fI.(file.File)
	if !ok {
		return nil, errors.New("failed to open file")
	}

	return f, err
}
