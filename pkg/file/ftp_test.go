package file

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pkgFtp "github.com/jlaffaye/ftp"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

// Test_NewFTPFile to test behavior of newFTPFile function
func Test_NewFTPFile(t *testing.T) {
	testCases := []struct {
		desc   string
		config *FTPConfig
		expErr error
	}{
		{"Success case: credentials are right", &FTPConfig{Host: "localhost", User: "myuser", Password: "mypass", Port: 21}, nil},
		{"Success case: host is missing, default localhost is taken", &FTPConfig{User: "myuser", Password: "mypass", Port: 21}, nil},
		{"Failure case: port no is missing", &FTPConfig{Host: "localhost", User: "myuser", Password: "mypass"}, &net.OpError{}},
		{"Failure case: password is incorrect", &FTPConfig{Host: "localhost", User: "myuser", Password: "", Port: 21}, &textproto.Error{}},
		{"Failure case: host is incorrect", &FTPConfig{Host: "incorrectHost", User: "myuser", Password: "mypass", Port: 21}, &net.OpError{}},
		{"Failure case: port no is incorrect", &FTPConfig{Host: "localhost", User: "myuser", Password: "mypass", Port: 8000}, &net.OpError{}},
		{"Failure case: username is incorrect", &FTPConfig{Host: "localhost", User: "incorrrectUser", Password: "mypass", Port: 21},
			&textproto.Error{}},
		{"Failure case: username is missing", &FTPConfig{Host: "localhost", Password: "mypass", Port: 21}, &textproto.Error{}},
	}

	for i, tc := range testCases {
		ftpObj, err := newFTPFile(tc.config, "test.txt", READWRITE)

		assert.IsTypef(t, &ftp{}, ftpObj, "Test[%d] failed: %v", i+1, tc.desc)
		assert.IsTypef(t, tc.expErr, err, "Test[%d] failed: %v", i+1, tc.desc)
	}
}

// Test_fetch_FTP to test the behavior of fetch function
func Test_fetch_FTP(t *testing.T) {
	dir := t.TempDir()

	fileSuccess, err := os.CreateTemp(dir, "testFileSuccess.txt")
	if err != nil {
		t.Errorf("Error in creating temporary file: %v", err)

		return
	}

	fileFailure, err := os.CreateTemp(dir, "testFileFailure.txt")
	if err != nil {
		t.Errorf("Error in creating temporary file: %v", err)

		return
	}

	defer func() {
		fileSuccess.Close()
		fileFailure.Close()
	}()

	f := &ftp{conn: &mockFtpConn{t: t}}
	testCases := []struct {
		desc     string
		fileName string
		file     *os.File
		expErr   error
	}{
		{"Success case: able to fetch file", "test.txt", fileSuccess, nil},
		{"Failure case: failed to fetch file", "testFileFailure.txt", fileFailure, errors.Error("test error")},
		{"Failure case: file is missing", "testFileMissing.txt", fileFailure, errors.FileNotFound{FileName: "testFileMissing.txt"}},
	}

	for i, tc := range testCases {
		f.fileName = tc.fileName
		err := f.fetch(tc.file)

		assert.IsTypef(t, tc.expErr, err, "Test[%d] failed: %v", i+1, tc.desc)
	}
}

// Test_push_FTP to test the behavior of push function
func Test_push_FTP(t *testing.T) {
	dir := t.TempDir()

	fileSuccess, err := os.CreateTemp(dir, "testFileSuccess.txt")
	if err != nil {
		t.Errorf("Error in creating temporary file: %v", err)

		return
	}

	fileFailure, err := os.CreateTemp(dir, "testFileFailure.txt")
	if err != nil {
		t.Errorf("Error in creating temporary file: %v", err)

		return
	}

	f := &ftp{conn: &mockFtpConn{}}
	testCases := []struct {
		desc     string
		fileName string
		file     *os.File
		expErr   error
	}{
		{"Success case: able to push file", "testFileSuccess.txt", fileSuccess, nil},
		{"Failure case: failed to push file", "testFileFailure.txt", fileFailure, errors.Error("test error")},
		{"Failure case: file is missing", "testFileMissing.txt", fileFailure, errors.FileNotFound{FileName: "testFileMissing.txt"}},
	}

	for i, tc := range testCases {
		f.fileName = tc.fileName
		err = f.push(tc.file)

		assert.Equalf(t, tc.expErr, err, "Test[%d] failed: %v", i+1, tc.desc)
	}
}

// Test_list_FTP to test the behavior of list function
func Test_list_FTP(t *testing.T) {
	f := &ftp{conn: &mockFtpConn{}}
	testCases := []struct {
		desc       string
		folderName string
		expFiles   []string
		expErr     error
	}{
		{"Success case: able to list directory", "testDirSuccess", []string{"testfile1", "testfile2"}, nil},
		{"Success case: failed to list directory", "testDirFailure", nil, errors.Error("test error")},
		{"Success case: directory is missing", "testDirMissing", nil, errors.FileNotFound{FileName: "testDirMissing"}},
	}

	for i, tc := range testCases {
		files, err := f.list(tc.folderName)

		assert.Equalf(t, tc.expFiles, files, "Test[%d] failed: %v", i+1, tc.desc)
		assert.Equalf(t, tc.expErr, err, "Test[%d] failed: %v", i+1, tc.desc)
	}
}

// Test_move_FTP to test the behavior of move function
func Test_move_FTP(t *testing.T) {
	f := &ftp{conn: &mockFtpConn{t: t}}
	testcases := []struct {
		desc        string
		source      string
		destination string
		expErr      error
	}{
		{"Success", "/testSuccess.txt", "testDir/test.txt", nil},
		{"Failed: Source file not found", "/testFail.txt", "test/test.txt",
			errors.FileNotFound{FileName: "abcd", Path: "/testFail.txt"}},
		{"Failed: Unable to create destination dir", "/testFail.txt", "destFailed/test.txt",
			errors.Error("unable to create destination file")},
		{"Failed: Permission Denied", "/PermissionDenied.txt", "testDir/PermissionDenied.txt",
			errors.Error("Permission Denied")},
	}

	for i, tc := range testcases {
		err := f.conn.Move(tc.source, tc.destination)
		assert.Equal(t, tc.expErr, err, "Test [%d] failed. Expected: %v, got: %v,", i, tc.expErr, err)
	}
}

// Test_createNestedDirFTP to test behavior of createNestedDirFTP
func Test_createNestedDirFTP(t *testing.T) {
	parentDir := "testDestination"

	ftpCfg := FTPConfig{
		Host:     "localhost",
		User:     "myuser",
		Password: "mypass",
		Port:     21,
	}

	conn := initializeFTP(t, ftpCfg)

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	client := ftpConn{conn: conn, logger: logger}

	tests := []struct {
		desc   string
		path   string
		expLog string
	}{
		{"Success case: creates a new directory", parentDir, ""},
		{"Success case: parent dir already exists so creates only sub directory", parentDir + "/subDirectory",
			"Error 550 Create directory operation failed. in creating directory /testDestination/"},
	}

	for i, tc := range tests {
		createNestedDirFTP(&client, tc.path)

		assert.Containsf(t, b.String(), tc.expLog, "Test[%d] Failed: %v", i+1, tc.desc)
	}

	cleanUp(t, conn)
}

// mockFtpConn is the mock implementation of ftpOp
type mockFtpConn struct {
	t *testing.T
}

// Retr is the mock implementation of ftpOp.Read
func (s *mockFtpConn) Read(fileName string) (io.ReadCloser, error) {
	if strings.Contains(fileName, "testFileFailure") {
		return nil, errors.Error("test error")
	}

	if strings.Contains(fileName, "testFileMissing") {
		return nil, errors.FileNotFound{FileName: "testFileMissing.txt"}
	}

	dir := s.t.TempDir()

	file, err := os.CreateTemp(dir, fileName)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// Stor is the mock implementation of ftpOp.Write
func (s *mockFtpConn) Write(fileName string, _ io.Reader) error {
	if strings.Contains(fileName, "testFileFailure") {
		return errors.Error("test error")
	}

	if strings.Contains(fileName, "testFileMissing") {
		return errors.FileNotFound{FileName: "testFileMissing.txt"}
	}

	return nil
}

// List is the mock implementation of ftpOp.List
func (s *mockFtpConn) List(dirName string) (entries []*pkgFtp.Entry, err error) {
	if dirName == "testDirSuccess" {
		return []*pkgFtp.Entry{{Name: "testfile1"}, {Name: "testfile2"}}, nil
	}

	if strings.Contains(dirName, "testDirMissing") {
		return nil, errors.FileNotFound{FileName: "testDirMissing"}
	}

	return nil, errors.Error("test error")
}

// Move is the mock implementation of ftpConn.Rename
func (s *mockFtpConn) Move(source, destination string) error {
	if source == "/testSuccess.txt" {
		return nil
	}

	if source == "/PermissionDenied.txt" {
		return errors.Error("Permission Denied")
	}

	if destination == "destFailed/test.txt" {
		return errors.Error("unable to create destination file")
	}

	return errors.FileNotFound{FileName: "abcd", Path: "/testFail.txt"}
}

func (s *mockFtpConn) Mkdir(string) error {
	return nil
}

func TestFtpConn(t *testing.T) {
	conn, err := pkgFtp.Dial("localhost:21")
	if err != nil {
		t.Fatal(err)
	}

	s := &ftpConn{conn: conn}
	_, err = s.Read("testVal")
	assert.NotNil(t, err)

	err = s.Write("testVal", strings.NewReader("test"))
	assert.NotNil(t, err)

	_, err = s.List("testVal")
	assert.NotNil(t, err)

	err = s.Move("testSource", "testDestination")
	assert.NotNil(t, err)
}

func Test_move(t *testing.T) {
	tempDir := t.TempDir()

	fileName := filepath.Join(tempDir, "testFTPFile.txt")

	file, err := os.Create(fileName)
	if err != nil {
		t.Fatalf("error while creating '%s' file: %v", fileName, err)
	}

	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			t.Fatalf("error while closing the '%s' file: %v", file.Name(), err)
		}
	}(file)

	conn := initializeFTP(t, FTPConfig{Host: "localhost", User: "myuser", Password: "mypass", Port: 21})

	err = createFTPDirectory(conn, "testDir")
	if err != nil {
		t.Fatalf("error while creating directory in FTP: %v", err)
	}

	err = uploadFileToFTPDirectory(conn, fileName, "testDir/testFTPFile.txt")
	if err != nil {
		t.Fatalf("error while uploading file to the FTP: %v", err)
	}

	f := &ftp{conn: &ftpConn{conn: conn, logger: log.NewMockLogger(io.Discard)}}

	tests := []struct {
		desc        string
		source      string
		destination string
		expErr      error
	}{
		{"Success case: able to move file", "testDir", "testDestination", nil},
		{"Failure case: remote path does not exists", "invalid", "destination",
			&textproto.Error{Code: 550, Msg: "RNFR command failed."}},
	}

	for i, tc := range tests {
		err = f.move(tc.source, tc.destination)

		assert.Equalf(t, tc.expErr, err, "Test[%d] failed: %v", i+1, tc.desc)
	}

	cleanUp(t, conn)
}

func initializeFTP(t *testing.T, ftpConfigs FTPConfig) *pkgFtp.ServerConn {
	address := fmt.Sprintf("%s:%d", ftpConfigs.Host, ftpConfigs.Port)

	conn, err := pkgFtp.Dial(address)
	if err != nil {
		t.Fatalf("error while establishing connection to the host: %v", err)
	}

	err = conn.Login(ftpConfigs.User, ftpConfigs.Password)
	if err != nil {
		t.Fatalf("error while login to the ftp server: %v", err)
	}

	t.Setenv("FTP_USER", ftpConfigs.User)
	t.Setenv("FTP_PASSWORD", ftpConfigs.Password)
	t.Setenv("FTP_HOST", ftpConfigs.Host)
	t.Setenv("FTP_PORT", fmt.Sprintf("%d", ftpConfigs.Port))

	return conn
}

func createFTPDirectory(conn *pkgFtp.ServerConn, directoryPath string) error {
	return conn.MakeDir(directoryPath)
}

func uploadFileToFTPDirectory(conn *pkgFtp.ServerConn, filePath, remotePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	return conn.Stor(remotePath, bytes.NewReader(data))
}

func cleanUp(t *testing.T, conn *pkgFtp.ServerConn) {
	err := conn.RemoveDirRecur("testDestination")
	if err != nil {
		t.Fatalf("error while removing directory from FTP. err: %v", err)
	}

	err = conn.Quit()
	if err != nil {
		t.Fatalf("error while closing the connection to the host: %v", err)
	}
}

func (s *mockFtpConn) Close() error {
	return nil
}
