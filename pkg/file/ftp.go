package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	pkgFtp "github.com/jlaffaye/ftp"

	"gofr.dev/pkg/log"
)

type ftpOp interface {
	Read(fileName string) (io.ReadCloser, error)
	Write(fileName string, r io.Reader) error
	List(dir string) (entries []*pkgFtp.Entry, err error)
	Move(source, destination string) error
	Mkdir(path string) error
	Close() error
}

type ftp struct {
	fileName string
	fileMode Mode
	conn     ftpOp
}

type ftpConn struct {
	conn   *pkgFtp.ServerConn
	logger log.Logger
}

// Read to upload file using ServerConn.Read
func (s *ftpConn) Read(fileName string) (io.ReadCloser, error) {
	return s.conn.Retr(fileName)
}

// Write to download file using ServerConn.Write
func (s *ftpConn) Write(fileName string, r io.Reader) error {
	return s.conn.Stor(fileName, r)
}

// List to list files in directory on ftp using ServerConn.List
func (s *ftpConn) List(dir string) (entries []*pkgFtp.Entry, err error) {
	return s.conn.List(dir)
}

// Move to move files from source to destination
func (s *ftpConn) Move(source, destination string) error {
	err := s.conn.Rename(source, destination)
	if err != nil {
		return err
	}

	s.logger.Infof("Files moved successfully from source %v to destination %v", source, destination)

	return nil
}

// Mkdir to create new directory on ftp
func (s *ftpConn) Mkdir(path string) error {
	return s.conn.MakeDir(path)
}

// Close to quit the ftp connections
func (s *ftpConn) Close() error {
	return s.conn.Quit()
}

// createNestedDirFTP utility method to create directory recursively
func createNestedDirFTP(s *ftpConn, path string) {
	// Trim leading and trailing slashes
	path = strings.Trim(path, "/")

	dirs := strings.Split(path, "/")

	rootDir := "/"

	for _, dir := range dirs {
		if dir == "" {
			// Skip empty directory names (can happen with consecutive slashes)
			continue
		}

		// Directory doesn't exist, attempt to create it
		err := s.Mkdir(rootDir + dir + "/")
		if err != nil {
			s.logger.Infof("Error %v in creating directory %v", err.Error(), rootDir+dir+"/")
		}

		rootDir = rootDir + dir + "/"
	}
}

// newFTPFile factory function to connect to FTP server and return ftp pointer
func newFTPFile(c *FTPConfig, filename string, mode Mode) (*ftp, error) {
	ftpFile := &ftp{fileName: filename, fileMode: mode}

	err := connectFTP(c, ftpFile)

	go retryFTP(c, ftpFile)

	return ftpFile, err
}

// connectFTP utility function to connect to FTP server
func connectFTP(c *FTPConfig, f *ftp) error {
	var err error

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)

	conn, err := pkgFtp.Dial(addr)
	if err != nil {
		return err
	}

	err = conn.Login(c.User, c.Password)
	if err != nil {
		return err
	}

	f.conn = &ftpConn{conn: conn, logger: log.NewLogger()}

	return err
}

// fetch to download file from ftp
//
//lint:ignore gocritic // passing it by value makes the code more readable
func (f ftp) fetch(fd *os.File) error {
	srcFile, err := f.conn.Read(f.fileName)
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

// push to upload file to ftp
//
//lint:ignore gocritic // passing it by value makes the code more readable
func (f ftp) push(fd *os.File) error {
	err := f.conn.Write(f.fileName, fd)
	if err != nil {
		return err
	}

	return nil
}

// list to list files in given directory on ftp
//
//lint:ignore gocritic // passing it by value makes the code more readable
func (f ftp) list(folderName string) ([]string, error) {
	files := make([]string, 0)

	entries, err := f.conn.List(folderName)
	if err != nil {
		return nil, err
	}

	for i := range entries {
		files = append(files, entries[i].Name)
	}

	return files, nil
}

// move to move file from source to destination
func (f ftp) move(source, destination string) error {
	parentDir := filepath.Dir(destination)

	createNestedDirFTP(f.conn.(*ftpConn), parentDir)

	port, _ := strconv.Atoi(os.Getenv("FTP_PORT"))

	err := connectFTP(&FTPConfig{
		Host:          os.Getenv("FTP_HOST"),
		User:          os.Getenv("FTP_USER"),
		Password:      os.Getenv("FTP_PASSWORD"),
		Port:          port,
		RetryDuration: getRetryDuration(os.Getenv("FTP_RETRY_DURATION")),
	}, &f)
	if err != nil {
		return err
	}

	defer f.conn.Close()

	err = f.conn.Move(source, destination)
	if err != nil {
		return err
	}

	return err
}
