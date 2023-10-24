/*
Package file provides various functionalities for reading, writing and manipulating files and supports
different file storage backends like aws, azure, gcp, sftp, ftp along with the support for
local files.
*/
package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gofr.dev/pkg/errors"
)

type fileAbstractor struct {
	fileName             string
	fileMode             int
	FD                   *os.File
	remoteFileAbstracter cloudStore
}

func newLocalFile(filename string, mode Mode) *fileAbstractor {
	return &fileAbstractor{
		fileName: filename,
		fileMode: fetchLocalFileMode(mode),
	}
}

// Open It opens a file, either locally or remotely, based on the presence of a remote file abstractor.
// It handles various modes (read, write, append) and file access operations while managing temporary files as needed.
//
//nolint:gocognit // cannot reduce complexity without affecting readability.
func (l *fileAbstractor) Open() error {
	if l.remoteFileAbstracter == nil {
		file, err := os.OpenFile(l.fileName, l.fileMode, os.ModePerm)
		if err != nil {
			return err
		}

		l.FD = file

		return nil
	}

	fileMode := l.fileMode

	tmpFileMode := fetchLocalFileMode(READWRITE) // tmp file should be opened in WRITE mode for downloading and READ mode for uploading
	if l.fileMode == fetchLocalFileMode(APPEND) {
		tmpFileMode |= os.O_APPEND
	}

	tmpFileName := l.fileName + randomString()

	l.fileName = "/tmp/" + tmpFileName
	l.fileMode = tmpFileMode

	fd, err := os.OpenFile(l.fileName, l.fileMode, os.ModePerm)
	if err != nil {
		return err
	}

	l.FD = fd

	err = l.remoteFileAbstracter.fetch(l.FD)
	if err != nil && fileMode == fetchLocalFileMode(READ) {
		return err
	}

	// if a file is requested in READ mode, then the temp file should have only READ access. (Note that Data has been downloaded
	// to it, That means it needs write access to do that.)
	if l.fileMode == fetchLocalFileMode(READ) {
		_ = l.Close()
		l.fileMode = fetchLocalFileMode(READ) // tmpFile should also be in READ mode if azure file is in READ mode
		_ = l.Open()
	} else {
		_, err = l.Seek(startOffset, defaultWhence)
	}

	return err
}

// Read returns the number of bytes read along with any encountered errors, handling the case of a missing file.
func (l *fileAbstractor) Read(b []byte) (int, error) {
	if l.FD == nil {
		return 0, errors.FileNotFound{}
	}

	return l.FD.Read(b)
}

// Write returning the number of bytes written and any encountered errors while handling the case of a missing file.
func (l *fileAbstractor) Write(b []byte) (int, error) {
	if l.FD == nil {
		return 0, errors.FileNotFound{}
	}

	return l.FD.Write(b)
}

// Seek returns the new offset position along with any encountered errors, handling the case of a missing file.
func (l *fileAbstractor) Seek(offset int64, whence int) (int64, error) {
	if l.FD == nil {
		return 0, errors.FileNotFound{}
	}

	return l.FD.Seek(offset, whence)
}

// Close It handles local and remote file scenarios, seeking to the start of the file (if applicable), pushing changes
// to the remote file (if available), closing the file descriptor, and finally removing any temporary files if they exist.
func (l *fileAbstractor) Close() error {
	if l.FD == nil {
		return errors.FileNotFound{}
	}

	if l.remoteFileAbstracter == nil {
		return l.FD.Close()
	}

	if _, err := l.Seek(startOffset, defaultWhence); err != nil { // offset is set to the start of the file
		return err
	}

	err := l.remoteFileAbstracter.push(l.FD)
	if err != nil {
		return err
	}

	err = l.FD.Close()
	if err != nil {
		return err
	}

	return os.Remove(l.fileName)
}

// List returns a slice of strings containing the names of the listed files and any encountered errors during the operation.
func (l *fileAbstractor) List(directory string) ([]string, error) {
	files := make([]string, 0)

	if l.remoteFileAbstracter != nil {
		return l.remoteFileAbstracter.list(directory)
	}

	fInfo, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	for i := range fInfo {
		files = append(files, fInfo[i].Name())
	}

	return files, nil
}

// Move function is responsible for moving a file from a source location to a destination location
func (l *fileAbstractor) Move(src, dest string) error {
	if l.remoteFileAbstracter != nil {
		return l.remoteFileAbstracter.move(src, dest)
	}

	parentDir := filepath.Dir(dest)

	if err := createNestedDir(parentDir); err != nil {
		return err
	}

	return os.Rename(src, dest)
}

func (l *fileAbstractor) Copy(string, string) (int, error) {
	return 0, nil
}

func (l *fileAbstractor) Delete(string) error {
	return nil
}

// createNestedDir utility function to create directory recursively
func createNestedDir(path string) error {
	dirs := strings.Split(path, "/")
	dirPath := ""

	for _, dir := range dirs {
		dirPath = fmt.Sprintf("%v%v/", dirPath, dir)

		if _, err := os.Stat(dirPath); err == nil {
			continue
		}

		if err := os.Mkdir(dirPath, os.ModePerm|os.ModeDir); err != nil {
			return err
		}
	}

	return nil
}
