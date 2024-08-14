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
func (f *fileSystem) Mkdir(name string, _ os.FileMode) error {
	var msg string

	status := "ERROR"

	filePath := path.Join(f.config.RemoteDir, name)

	defer f.processLog(&FileLog{Operation: "Mkdir", Location: filePath, Status: &status, Message: &msg}, time.Now())

	if name == "" {
		f.logger.Errorf("Mkdir failed. Provide a valid directory : %v", errEmptyDirectory)
		return errEmptyDirectory
	}

	err := f.conn.MakeDir(filePath)
	if err != nil {
		f.logger.Errorf("Mkdir failed. Error creating directory at %q : %v", filePath, err)
		return err
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("%q created successfully", name)

	return nil
}

func (f *fileSystem) mkdirAllHelper(filepath string) []string {
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

// MkdirAll creates directories recursively on the FTP server. Here, os.FileMode is unused.
// Here, os.FileMode is unused, but is added to comply with FileSystem interface.
func (f *fileSystem) MkdirAll(name string, _ os.FileMode) error {
	var msg string

	status := "ERROR"

	defer f.processLog(&FileLog{Operation: "MkdirAll", Location: path.Join(f.config.RemoteDir, name), Status: &status, Message: &msg}, time.Now())

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

	status = "SUCCESS"
	msg = fmt.Sprintf("Directories %q creation completed successfully", name)

	return nil
}

// RemoveAll removes a directory and its contents recursively from the FTP server.
func (f *fileSystem) RemoveAll(name string) error {
	var msg string

	status := "ERROR"

	filePath := f.config.RemoteDir

	if name != "." {
		filePath = path.Join(f.config.RemoteDir, name)
	}

	defer f.processLog(&FileLog{Operation: "RemoveAll", Location: filePath, Status: &status, Message: &msg}, time.Now())

	if name == "" {
		f.logger.Errorf("RemoveAll failed. Provide a valid path : %v", errEmptyPath)
		return errEmptyPath
	}

	err := f.conn.RemoveDirRecur(filePath)
	if err != nil {
		f.logger.Errorf("RemoveAll failed. Error while deleting directories : %v", err)
		return err
	}

	msg = "Directories deleted successfully"
	status = "SUCCESS"

	return nil
}

// Stat returns the file/directory information in the directory.
func (f *fileSystem) Stat(name string) (file_interface.FileInfo, error) {
	status := "ERROR"

	defer f.processLog(&FileLog{
		Operation: "Stat",
		Location:  f.config.RemoteDir,
		Status:    &status,
	}, time.Now())

	if name == "" {
		f.logger.Errorf("Stat failed. Provide a valid path : %v", errEmptyPath)
		return nil, errEmptyPath
	}

	filePath := path.Join(f.config.RemoteDir, name)

	if path.Ext(name) == "" {
		fl := &file{
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

	entry, err := f.conn.List(filePath)
	if err != nil {
		f.logger.Errorf("Stat failed. Error Retrieving file : %v", errEmptyPath)
		return nil, err
	}

	status = "SUCCESS"

	return &file{
		name:      entry[0].Name,
		path:      filePath,
		entryType: entry[0].Type,
		modTime:   entry[0].Time,
		conn:      f.conn,
		logger:    f.logger,
		metrics:   f.metrics,
	}, nil
}

// CurrentDir returns the path of the current directory.
func (f *fileSystem) CurrentDir() (string, error) {
	defer f.processLog(&FileLog{
		Operation: "CurrentDir",
		Location:  f.config.RemoteDir,
	}, time.Now())

	return f.conn.CurrentDir()
}

// ChangeDir changes the current directory.
func (f *fileSystem) ChangeDir(dir string) error {
	var msg string

	status := "ERROR"

	defer f.processLog(&FileLog{
		Operation: "ChangeDir",
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

	f.config.RemoteDir = filepath
	status = "SUCCESS"
	msg = fmt.Sprintf("Changed current directory to %q", filepath)

	return nil
}

// ReadDir returns a list of files/directories present in the directory.
func (f *fileSystem) ReadDir(dir string) ([]file_interface.FileInfo, error) {
	var msg string

	status := "ERROR"

	defer f.processLog(&FileLog{Operation: "ChangeDir", Location: f.config.RemoteDir, Status: &status, Message: &msg}, time.Now())

	filepath := f.config.RemoteDir
	if dir != "." {
		filepath = path.Join(f.config.RemoteDir, dir)
	}

	entries, err := f.conn.List(filepath)
	if err != nil {
		f.logger.Errorf("ReadDir failed. Error reading directory : %v", errEmptyPath)
		return nil, err
	}

	var fileInfo []file_interface.FileInfo

	for _, entry := range entries {
		entryPath := path.Join(filepath, entry.Name)

		fileInfo = append(fileInfo, &file{
			name:      entry.Name,
			modTime:   entry.Time,
			entryType: entry.Type,
			path:      entryPath,
			conn:      f.conn,
			logger:    f.logger,
			metrics:   f.metrics,
		})
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("Found %d entries in %q", len(entries), filepath)

	return fileInfo, nil
}
