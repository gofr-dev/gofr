package file

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gofr.dev/pkg/gofr/datasource"
)

var (
	errInvalidPath = errors.New("invalid path: contains illegal characters or path traversal attempts")
)

type fileSystem struct {
	logger datasource.Logger
}

// New initializes local filesystem with logger.
func New(logger datasource.Logger) FileSystem {
	return fileSystem{logger: logger}
}

func (fs fileSystem) Create(name string) (File, error) {
	cleanPath, err := fs.validatePath(name)
	if err != nil {
		return nil, err
	}

	newFile, err := os.Create(cleanPath)
	if err != nil {
		return nil, err
	}

	return &file{File: newFile, logger: fs.logger}, nil
}

func (fileSystem) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

func (fileSystem) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}

func (fs fileSystem) Open(name string) (File, error) {
	openFile, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	return &file{File: openFile, logger: fs.logger}, nil
}

func (fs fileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	openFile, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}

	return &file{File: openFile, logger: fs.logger}, nil
}

func (fs fileSystem) Remove(name string) error {
	cleanPath, err := fs.validatePath(name)
	if err != nil {
		return err
	}

	return os.Remove(cleanPath)
}

func (fileSystem) RemoveAll(name string) error {
	// In case we remove current working directory, say using "../currentDir"
	// the current directory needs to be reset to its parent directory.
	curr, err := os.Getwd()
	if err != nil {
		return err
	}

	err = os.RemoveAll(name)
	if err != nil {
		return err
	}

	removePath := path.Join(curr, name)
	if curr == removePath {
		err = os.Chdir(path.Join(curr, ".."))
		if err != nil {
			return err
		}
	}

	return nil
}

func (fileSystem) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

// Stat returns the file/directory info.
func (fileSystem) Stat(name string) (FileInfo, error) {
	return os.Stat(name)
}

// Getwd returns the full path of the current working directory.
func (fileSystem) Getwd() (string, error) {
	return os.Getwd()
}

// ChDir changes the current working directory to the named directory.
// If there is an error, it will be of type *PathError.
func (fileSystem) ChDir(dir string) error {
	return os.Chdir(dir)
}

// ReadDir reads the named directory, returning all its directory entries sorted by filename.
// If an error occurs reading the directory, ReadDir returns the entries it was able to read before the error, along with the error.
// It returns the list of files/directories present in the current directory when "." is passed.
func (fs fileSystem) ReadDir(dir string) ([]FileInfo, error) {
	cleanDir, err := fs.validatePath(dir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		return nil, err
	}

	fileInfo := make([]FileInfo, len(entries))
	for i := range entries {
		fileInfo[i], err = entries[i].Info()
		if err != nil {
			return fileInfo, err
		}
	}

	return fileInfo, err
}

// validatePath checks if the given path is safe to use.
func (fileSystem) validatePath(filePath string) (string, error) {
	// Clean the path to handle any . or .. sequences.
	cleanPath := filepath.Clean(filePath)

	// Check for absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", errInvalidPath
	}

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return "", errInvalidPath
	}

	return cleanPath, nil
}
