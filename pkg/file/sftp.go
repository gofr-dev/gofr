package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pkgSftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sftp struct {
	filename string
	fileMode Mode
	client   fileOp
}

type sftpClient struct {
	*pkgSftp.Client
}

type fileOp interface {
	Open(fileName string) (io.ReadWriteCloser, error)
	Create(fileName string) (io.ReadWriteCloser, error)
	ReadDir(dirName string) ([]os.FileInfo, error)
	Move(source, destination string) error
	Stat(path string) (os.FileInfo, error)
	Mkdir(path string) error
}

// Move moves the file from source to destination
func (s sftpClient) Move(source, destination string) error {
	return s.Client.Rename(source, destination)
}

// Open opens the named file for reading.
func (s sftpClient) Open(fileName string) (io.ReadWriteCloser, error) {
	return s.Client.Open(fileName)
}

// Create creates the named file mode 0666 (before umask), truncating it if it
// already exists. If successful, methods on the returned File can be used for
// I/O; the associated file descriptor has mode O_RDWR. If you need more
// control over the flags/mode used to open the file see client.OpenFile.
func (s sftpClient) Create(fileName string) (io.ReadWriteCloser, error) {
	return s.Client.Create(fileName)
}

// ReadDir reads the directory named by dirname and returns a list of
// directory entries.
func (s sftpClient) ReadDir(dirName string) ([]os.FileInfo, error) {
	return s.Client.ReadDir(dirName)
}

// Stat returns a FileInfo structure describing the file specified by path 'p'.
// If 'p' is a symbolic link, the returned FileInfo structure describes the referent file.
func (s sftpClient) Stat(path string) (os.FileInfo, error) {
	return s.Client.Stat(path)
}

// Mkdir creates the specified directory.An error will be returned if a file or
// directory with the specified path already exists, or if the directory's
// parent folder does not exist (the method cannot create complete paths).
func (s sftpClient) Mkdir(path string) error {
	return s.Client.Mkdir(path)
}

// createNestedDirSFTP utility method to create directory recursively
func createNestedDirSFTP(s sftpClient, path string) error {
	dirs := strings.Split(path, "/")
	dirPath := ""

	for _, dir := range dirs {
		dirPath = fmt.Sprintf("%v%v/", dirPath, dir)

		if _, err := s.Stat(dirPath); err == nil {
			continue
		}

		if err := s.Mkdir(dirPath); err != nil {
			return err
		}
	}

	return nil
}

func newSFTPFile(c *SFTPConfig, filename string, mode Mode) (*sftp, error) {
	sftpFile := &sftp{filename: filename, fileMode: mode}
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	config := &ssh.ClientConfig{
		User:            c.User,
		Auth:            []ssh.AuthMethod{ssh.Password(c.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // using InsecureIgnoreHostKey to accept any host key
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	client, err := pkgSftp.NewClient(conn)
	if err != nil {
		return nil, err
	}

	sftpFile.client = sftpClient{Client: client}

	return sftpFile, nil
}

func (s *sftp) move(source, destination string) error {
	parentDir := filepath.Dir(destination)

	if err := createNestedDirSFTP(s.client.(sftpClient), parentDir); err != nil {
		return err
	}

	return s.client.Move(source, destination)
}

func (s *sftp) fetch(fd *os.File) error {
	srcFile, err := s.client.Open(s.filename)
	if err != nil {
		return err
	}

	defer srcFile.Close()

	_, err = io.Copy(fd, srcFile)
	if err != nil {
		return err
	}

	return nil
}
func (s *sftp) push(fd *os.File) error {
	destFile, err := s.client.Create(s.filename)
	if err != nil {
		return err
	}

	defer destFile.Close()

	_, err = io.Copy(destFile, fd)
	if err != nil {
		return err
	}

	return nil
}

func (s *sftp) list(folderName string) ([]string, error) {
	files := make([]string, 0)

	fInfo, err := s.client.ReadDir(folderName)
	if err != nil {
		return nil, err
	}

	for i := range fInfo {
		files = append(files, fInfo[i].Name())
	}

	return files, nil
}
