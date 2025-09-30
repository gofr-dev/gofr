package ftp

import (
	"fmt"
	"os"
	"path"
	"slices"
	"time"

	"github.com/jlaffaye/ftp"
	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

// Mkdir creates a directory on the FTP server.
// Here, os.FileMode is unused, but is added to comply with FileSystem interface.
func (f *FileSystem) Mkdir(name string, _ os.FileMode) error {
	var msg string

	status := statusError

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.sendOperationStats(&FileLog{
		Operation: "Mkdir",
		Location:  filePath,
		Status:    &status,
		Message:   &msg,
	}, time.Now())

	if name == "" {
		f.logger.Errorf("Mkdir failed. Provide a valid directory : %v", errEmptyDirectory)
		return errEmptyDirectory
	}

	err := f.conn.MakeDir(filePath)
	if err != nil {
		f.logger.Errorf("Mkdir failed. Error creating directory at %q : %v", filePath, err)
		return err
	}

	status = statusSuccess
	msg = fmt.Sprintf("%q created successfully", name)

	return nil
}

func (f *FileSystem) mkdirAllHelper(filepath string) []string {
	var dirs []string

	currentdir := filepath

	for {
		err := f.conn.MakeDir(currentdir)
		if err == nil {
			dirs = append(dirs, currentdir)
			break
		}

		parentDir, dir := path.Split(currentdir)

		dirs = append(dirs, dir)

		if parentDir == "" || parentDir == "/" {
			break
		}

		currentdir = path.Clean(parentDir)
	}

	slices.Reverse(dirs)

	return dirs
}

// MkdirAll creates directories recursively on the FTP server.
// Here, os.FileMode is unused, but is added to comply with FileSystem interface.
func (f *FileSystem) MkdirAll(name string, _ os.FileMode) error {
	var msg string

	status := statusError

	defer f.sendOperationStats(&FileLog{
		Operation: "MkdirAll",
		Location:  path.Join(f.config.RemoteDir, name),
		Status:    &status,
		Message:   &msg,
	}, time.Now())

	if name == "" {
		f.logger.Errorf("MkdirAll failed. Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}
	// returns a slice of those directories that do not exist with the first index being the latest existing parent directory path.
	dirs := f.mkdirAllHelper(name)

	currentDir := dirs[0]

	for i, dir := range dirs {
		if i == 0 {
			continue
		}

		currentDir = path.Join(currentDir, dir)

		err := f.conn.MakeDir(currentDir)
		if err != nil {
			f.logger.Errorf("MkdirAll failed : %v", err)
			return err
		}
	}

	status = statusSuccess
	msg = fmt.Sprintf("Directories %q creation completed successfully", name)

	return nil
}

// RemoveAll removes a directory and its contents recursively from the FTP server.
func (f *FileSystem) RemoveAll(name string) error {
	var msg string

	status := statusError

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.sendOperationStats(&FileLog{
		Operation: "RemoveAll",
		Location:  filePath,
		Status:    &status,
		Message:   &msg,
	}, time.Now())

	if name == "" {
		f.logger.Errorf("RemoveAll failed. Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}

	// If user changes the directory at any point of time, the fs.config.RemoteDir gets updated each time.
	// Hence, in case we remove current working directory, say using "../currentDir"
	// the fs.config.RemoteDir needs to be reset to its parent directory.
	if filePath == f.config.RemoteDir {
		f.config.RemoteDir = path.Join(f.config.RemoteDir, "..")
	}

	err := f.conn.RemoveDirRecur(filePath)
	if err != nil {
		f.logger.Errorf("RemoveAll failed. Error while deleting directories : %v", err)
		return err
	}

	msg = fmt.Sprintf("Directory with path %q deleted successfully", filePath)
	status = statusSuccess

	return nil
}

// Stat returns information of the files/directories in the specified directory.
func (f *FileSystem) Stat(name string) (file_interface.FileInfo, error) {
	status := statusError

	defer f.sendOperationStats(&FileLog{
		Operation: "Stat",
		Location:  f.config.RemoteDir,
		Status:    &status,
	}, time.Now())

	if name == "" {
		f.logger.Errorf("Stat failed. Provide a valid path : %v", errEmptyPath)
		return nil, errEmptyPath
	}

	filePath := path.Join(f.config.RemoteDir, name)

	// if it is a directory
	if path.Ext(name) == "" {
		fl := &File{
			name:      name,
			path:      filePath,
			entryType: ftp.EntryTypeFolder,
			conn:      f.conn,
			logger:    f.logger,
			metrics:   f.metrics,
		}
		fl.modTime = fl.ModTime()

		return fl, nil
	}

	// if it is a file
	entry, err := f.conn.List(filePath)
	if err != nil {
		f.logger.Errorf("Stat failed. Error Retrieving file : %v", errEmptyPath)
		return nil, err
	}

	status = statusSuccess

	return &File{
		name:      entry[0].Name,
		path:      filePath,
		entryType: entry[0].Type,
		modTime:   entry[0].Time,
		conn:      f.conn,
		logger:    f.logger,
		metrics:   f.metrics,
	}, nil
}

// Getwd returns the full path of the current directory.
func (f *FileSystem) Getwd() (string, error) {
	defer f.sendOperationStats(&FileLog{
		Operation: "Getwd",
		Location:  f.config.RemoteDir,
	}, time.Now())

	return f.conn.CurrentDir()
}

// ChDir takes the relative path as argument and changes the current directory.
func (f *FileSystem) ChDir(dir string) error {
	var msg string

	status := statusError

	defer f.sendOperationStats(&FileLog{
		Operation: "ChDir",
		Location:  f.config.RemoteDir,
		Status:    &status,
		Message:   &msg,
	}, time.Now())

	filepath := path.Join(f.config.RemoteDir, dir)

	err := f.conn.ChangeDir(filepath)
	if err != nil {
		f.logger.Errorf("ChangeDir failed : %v", errEmptyPath)
		return err
	}

	msg = fmt.Sprintf("Changed current directory from %q to %q", f.config.RemoteDir, filepath)
	f.config.RemoteDir = filepath
	status = statusSuccess

	return nil
}

// ReadDir reads the named directory, returning all its directory entries sorted by filename.
// If an error occurs reading the directory, ReadDir returns the entries it was able to read before the error, along with the error.
// It returns the list of files/directories present in the current directory when "." is passed.
func (f *FileSystem) ReadDir(dir string) ([]file_interface.FileInfo, error) {
	var msg string

	status := statusError

	defer f.sendOperationStats(&FileLog{
		Operation: "ReadDir",
		Location:  f.config.RemoteDir,
		Status:    &status,
		Message:   &msg,
	}, time.Now())

	filepath := f.config.RemoteDir
	if dir != "." {
		filepath = path.Join(f.config.RemoteDir, dir)
	}

	entries, err := f.conn.List(filepath)
	if err != nil {
		f.logger.Errorf("ReadDir failed. Error reading directory : %v", errEmptyPath)
		return nil, err
	}

	fileInfo := make([]file_interface.FileInfo, 0)

	for _, entry := range entries {
		entryPath := path.Join(filepath, entry.Name)

		fileInfo = append(fileInfo, &File{
			name:      entry.Name,
			modTime:   entry.Time,
			entryType: entry.Type,
			path:      entryPath,
			conn:      f.conn,
			logger:    f.logger,
			metrics:   f.metrics,
		})
	}

	status = statusSuccess
	msg = fmt.Sprintf("Found %d entries in %q", len(entries), filepath)

	return fileInfo, nil
}
