package gofr

import (
	"errors"
	"fmt"
	"os"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	ErrFailedToCreateFile = errors.New("failed to create file")
	ErrFailedToOpenFile   = errors.New("failed to open file")
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
		return nil, fmt.Errorf("%w: %w", ErrFailedToCreateFile, err)
	}

	return f, err
}

func (fsW *fileSysWrapper) Open(name string) (file.File, error) {
	fI, err := fsW.FileSystem.Open(name)

	f, ok := fI.(file.File)
	if !ok {
		return nil, fmt.Errorf("%w: %w", ErrFailedToOpenFile, err)
	}

	return f, err
}

func (fsW *fileSysWrapper) OpenFile(name string, flag int, perm os.FileMode) (file.File, error) {
	fI, err := fsW.FileSystem.OpenFile(name, flag, perm)

	f, ok := fI.(file.File)
	if !ok {
		return nil, fmt.Errorf("%w: %w", ErrFailedToOpenFile, err)
	}

	return f, err
}
