package handler

import (
	"fmt"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

const size = 20

type File interface {
	// Open should open the file in the provided mode. Implementation depends on the file storage to be used.
	Open() error
	// Read calls the internal file descriptor method to Read.
	Read([]byte) (int, error)
	// Write calls the internal file descriptor method to Write.
	Write([]byte) (int, error)
	// Close calls the internal file descriptor method to Close.
	Close() error
	// List lists all the files in the directory
	List(directory string) ([]string, error)
	// Move moves file from source to destination
	Move(source, destination string) error
}

type fileHandler struct {
	fileOP File
}

//nolint:revive // has to be initialized using New func
func New(f File) fileHandler {
	return fileHandler{fileOP: f}
}

func (f fileHandler) Read(*gofr.Context) (interface{}, error) {
	err := f.fileOP.Open()
	if err != nil {
		return nil, err
	}

	defer f.fileOP.Close()

	b := make([]byte, size)

	_, err = f.fileOP.Read(b)
	if err != nil {
		return nil, err
	}

	return string(b), nil
}

func (f fileHandler) Write(*gofr.Context) (interface{}, error) {
	err := f.fileOP.Open()
	if err != nil {
		return nil, err
	}

	b := []byte("Welcome to gofr.dev!")

	_, err = f.fileOP.Write(b)
	if err != nil {
		return nil, err
	}

	err = f.fileOP.Close()
	if err != nil {
		return nil, err
	}

	return "File written successfully!", err
}

func (f fileHandler) List(*gofr.Context) (interface{}, error) {
	k, err := f.fileOP.List(".")

	return k, err
}

func (f fileHandler) Move(ctx *gofr.Context) (interface{}, error) {
	src := ctx.Param("src")
	if src == "" {
		return nil, errors.MissingParam{Param: []string{"src"}}
	}

	dest := ctx.Param("dest")
	if dest == "" {
		return nil, errors.MissingParam{Param: []string{"dest"}}
	}

	err := f.fileOP.Move(src, dest)
	if err != nil {
		return nil, err
	}

	defer f.fileOP.Close()

	return fmt.Sprintf("File moved successfully from source:%s to destination:%s", src, dest), nil
}
