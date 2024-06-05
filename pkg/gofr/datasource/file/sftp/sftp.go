package sftp

import (
	"fmt"
	"gofr.dev/pkg/gofr/datasource"
	"golang.org/x/crypto/ssh"
	"os"
	"time"

	"github.com/pkg/sftp"
)

// Fs is a datasource.FileSystem implementation that uses functions provided by the sftp package.
//
// For details in any method, check the documentation of the sftp package
// (github.com/pkg/sftp).
type Fs struct {
	client *sftp.Client
}

type Config struct {
	Username string
	Password string
	Host     string
	Port     int
}

func New(c Config) *Fs {
	config := &ssh.ClientConfig{
		User:            c.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(c.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), config)
	if err != nil {
		return nil
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil
	}

	return &Fs{client: client}
}

func (s Fs) Name() string { return "sftpfs" }

func (s Fs) Create(name string) (datasource.File, error) {
	return fileCreate(s.client, name)
}

func (s Fs) Mkdir(name string, perm os.FileMode) error {
	err := s.client.Mkdir(name)
	if err != nil {
		return err
	}
	return s.client.Chmod(name, perm)
}

func (s Fs) MkdirAll(path string, perm os.FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := s.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return err
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent
		err = s.MkdirAll(path[0:j-1], perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = s.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := s.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

func (s Fs) Open(name string) (datasource.File, error) {
	return fileOpen(s.client, name)
}

// OpenFile calls the OpenFile method on the SSHFS connection. The mode argument
// is ignored because it's ignored by the github.com/pkg/sftp implementation.
func (s Fs) OpenFile(name string, flag int, perm os.FileMode) (datasource.File, error) {
	sshfsFile, err := s.client.OpenFile(name, flag)
	if err != nil {
		return nil, err
	}
	err = sshfsFile.Chmod(perm)
	return &File{fd: sshfsFile}, err
}

func (s Fs) Remove(name string) error {
	return s.client.Remove(name)
}

func (s Fs) RemoveAll(path string) error {
	// TODO have a look at os.RemoveAll
	// https://github.com/golang/go/blob/master/src/os/path.go#L66
	return nil
}

func (s Fs) Rename(oldname, newname string) error {
	return s.client.Rename(oldname, newname)
}

func (s Fs) Stat(name string) (os.FileInfo, error) {
	return s.client.Stat(name)
}

func (s Fs) Lstat(p string) (os.FileInfo, error) {
	return s.client.Lstat(p)
}

func (s Fs) Chmod(name string, mode os.FileMode) error {
	return s.client.Chmod(name, mode)
}

func (s Fs) Chown(name string, uid, gid int) error {
	return s.client.Chown(name, uid, gid)
}

func (s Fs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return s.client.Chtimes(name, atime, mtime)
}

func (s Fs) UseLogger(logger interface{}) {
	//TODO implement me
	panic("implement me")
}

func (s Fs) UseMetrics(metrics interface{}) {
	//TODO implement me
	panic("implement me")
}

func (s Fs) Connect() {
	//TODO implement me
	panic("implement me")
}
