package file

import (
	"io/fs"
	"os"
	"path"

	"gofr.dev/pkg/gofr/datasource"
)

type FileSys struct {
	logger datasource.Logger
}

// New initializes local filesystem with logger and provides the functionality to interact with the files and directories on local system.
func New(logger datasource.Logger) *FileSys {
	return &FileSys{logger: logger}
}

// Create creates or truncates the named file. If the file already exists, it is truncated.
// If the file does not exist, it is created with mode 666.
// If successful, methods on the returned File can be used for I/ O; the associated file descriptor has mode O_RDWR.
// If there is an error, it will be of type *PathError
func (f *FileSys) Create(name string) (File, error) {
	newFile, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	return &file{File: newFile, logger: f.logger}, nil
}

// Mkdir creates a new directory with the specified name and permission bits (before umask).
func (*FileSys) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

// MkdirAll creates a directory named path, along with any necessary parents, and returns nil, or else returns an error.
func (*FileSys) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}

// Open opens the named file for reading.
// If successful, methods on the returned file can be used for reading; the associated file descriptor has mode O_RDONLY.
func (f *FileSys) Open(name string) (File, error) {
	openFile, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	return &file{File: openFile, logger: f.logger}, nil
}

// OpenFile is the generalized open call; most users will use Open or Create instead.
// It takes flag and perm attributes to open the file.
func (f *FileSys) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	openFile, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}

	return &file{File: openFile, logger: f.logger}, nil
}

// Remove removes the named file or (empty) directory.
func (*FileSys) Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll removes path and any children it contains.
func (*FileSys) RemoveAll(name string) error {
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

// Rename renames (moves) old path to new path.
func (*FileSys) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

// Stat returns the file/directory info.
func (*FileSys) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// Getwd returns the full path of the current working directory.
func (*FileSys) Getwd() (string, error) {
	return os.Getwd()
}

// ChDir changes the current working directory to the named directory.
// If there is an error, it will be of type *PathError.
func (*FileSys) ChDir(dir string) error {
	return os.Chdir(dir)
}

// ReadDir reads the named directory, returning all its directory entries sorted by filename.
// If an error occurs reading the directory, ReadDir returns the entries it was able to read before the error, along with the error.
// It returns the list of files/directories present in the current directory when "." is passed.
func (*FileSys) ReadDir(dir string) ([]fs.FileInfo, error) {
	entries, err := os.ReadDir(dir)

	fileInfo := make([]fs.FileInfo, len(entries))
	for i := range entries {
		fileInfo[i], err = entries[i].Info()
		if err != nil {
			return fileInfo, err
		}
	}

	return fileInfo, err
}
