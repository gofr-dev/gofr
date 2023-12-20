package file

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	pkgSftp "github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"

	pkgErr "gofr.dev/pkg/errors"
)

func Test_NewSFTPFile(t *testing.T) {
	filename := "test.txt"
	mode := READWRITE
	c1 := &SFTPConfig{Host: "localhost", User: "", Password: "", Port: 22}

	_, err := newSFTPFile(c1, filename, mode)

	expErrMsg := "ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain"
	expErr := errors.New(expErrMsg)

	assert.Equal(t, expErr.Error(), err.Error(), "Test failed, Expected:%v, got:%v ", expErr, err)
}

type mockSftpClient struct {
	t *testing.T
	*pkgSftp.Client
}

func (s mockSftpClient) Move(source, destination string) error {
	if strings.Contains(source, "ErrorDirectory") {
		return pkgErr.Error("test error")
	}

	if strings.Contains(destination, "errorDestDir/filename") {
		return pkgErr.Error("unable to create destination file")
	}

	return nil
}

func (s mockSftpClient) Open(fileName string) (io.ReadWriteCloser, error) {
	if fileName == "Open error.txt" {
		return nil, pkgErr.FileNotFound{}
	}
	// Creating temporary directory for tests
	d := s.t.TempDir()
	_ = os.Chdir(d)

	// Creating file in the temp directory
	fd, _ := os.Create(fileName)

	return fd, nil
}

func (s mockSftpClient) Create(fileName string) (io.ReadWriteCloser, error) {
	if fileName == "Create error.txt" {
		return nil, errors.New("error in creating the file")
	}
	// Creating temporary directory for tests
	d := s.t.TempDir()
	_ = os.Chdir(d)

	// Creating file in the temp directory
	fd, _ := os.Create(fileName)

	return fd, nil
}

type mockFileInfo struct {
	name string
	os.FileInfo
}

func (m mockFileInfo) Name() string {
	return m.name
}

func (s mockSftpClient) ReadDir(dirName string) ([]os.FileInfo, error) {
	if dirName == "ErrorDirectory" {
		return nil, errors.New("error while reading directory")
	}

	files := make([]os.FileInfo, 0)
	m1 := mockFileInfo{name: "test1.txt"}
	m2 := mockFileInfo{name: "test2.txt"}
	files = append(files, m1, m2)

	return files, nil
}

func (s mockSftpClient) Stat(string) (os.FileInfo, error) {
	return nil, nil
}

func (s mockSftpClient) Mkdir(string) error {
	return nil
}

func Test_Fetch(t *testing.T) {
	s1 := &sftp{
		filename: "Open error.txt",
		fileMode: "r",
		client:   mockSftpClient{},
	}
	s2 := &sftp{
		filename: "Copy error.txt",
		fileMode: "r",
		client:   mockSftpClient{t: t},
	}
	s3 := &sftp{
		filename: "test2.txt",
		fileMode: "rw",
		client:   mockSftpClient{t: t},
	}

	openErr := pkgErr.FileNotFound{}
	copyErr := errors.New("invalid argument")

	testcases := []struct {
		desc   string
		s      *sftp
		expErr error
	}{
		{"Open Error", s1, openErr},
		{"Copy Error", s2, copyErr},
		{"Success", s3, nil},
	}
	for i, tc := range testcases {
		l := newLocalFile(tc.s.filename, tc.s.fileMode)
		_ = l.Open()
		err := tc.s.fetch(l.FD)
		assert.Equal(t, tc.expErr, err, "Test [%v] failed. Expected: %v, got: %v,", i, tc.expErr, err)
	}
}

func Test_Push(t *testing.T) {
	s1 := &sftp{
		filename: "Create error.txt",
		fileMode: "r",
		client:   mockSftpClient{},
	}
	s2 := &sftp{
		filename: "Copy error.txt",
		fileMode: "r",
		client:   mockSftpClient{t: t},
	}
	s3 := &sftp{
		filename: "test1.txt",
		fileMode: "rw",
		client:   mockSftpClient{t: t},
	}
	createErr := errors.New("error in creating the file")
	copyErr := errors.New("invalid argument")
	testcases := []struct {
		desc   string
		s      *sftp
		expErr error
	}{
		{"Create Error", s1, createErr},
		{"Copy Error", s2, copyErr},
		{"Success", s3, nil},
	}

	for i, tc := range testcases {
		l := newLocalFile(tc.s.filename, tc.s.fileMode)
		_ = l.Open()
		err := tc.s.push(l.FD)
		assert.Equal(t, tc.expErr, err, "Test [%v] failed. Expected: %v, got: %v,", i, tc.expErr, err)
	}
}

func Test_SftpList(t *testing.T) {
	s := &sftp{
		filename: "",
		fileMode: "",
		client:   mockSftpClient{},
	}
	// Creating temporary directory for tests
	d := t.TempDir()
	_ = os.Chdir(d)

	// Creating two files in the temp directory
	_, _ = os.Create("test1.txt")
	_, _ = os.Create("test2.txt")

	dirErr := errors.New("error while reading directory")

	testcases := []struct {
		desc    string
		dirName string
		expErr  error
	}{
		{"Read Error", "ErrorDirectory", dirErr},
		{"Success", d, nil},
	}
	for i, tc := range testcases {
		_, err := s.list(tc.dirName)
		assert.Equal(t, tc.expErr, err, "Test [%v] failed. Expected: %v, got: %v,", i, tc.expErr, err)
	}
}

func Test_SftpMove(t *testing.T) {
	s := &sftp{
		filename: "",
		fileMode: "",
		client:   mockSftpClient{},
	}

	testcases := []struct {
		desc        string
		source      string
		destination string
		expErr      error
	}{
		{"Success", "/test.txt", "/test.txt", nil},
		{"Move Error", "ErrorDirectory", "ErrorDirectory", pkgErr.Error("test error")},
		{"Move Error", "sourceDir", "errorDestDir/filename", pkgErr.Error("unable to create destination file")},
	}
	for i, tc := range testcases {
		err := s.client.Move(tc.source, tc.destination)
		assert.Equal(t, tc.expErr, err, "Test [%v] failed. Expected: %v, got: %v,", i, tc.expErr, err)
	}
}

func Test_Sftpmove(t *testing.T) {
	sftpCfg := &SFTPConfig{
		Host:     "localhost",
		User:     "myuser",
		Password: "mypass",
		Port:     2222,
	}

	s, err := newSFTPFile(sftpCfg, "testFile.txt", "rw")
	if err != nil {
		t.Fatalf("Error in creating connection to SFTP: %v", err)
	}

	_, ok := s.client.(sftpClient)
	if !ok {
		t.Fatalf("Unable to parse ftpConn")
	}

	err = s.move("/test.txt", "/test.txt")

	assert.Equal(t, errors.New("file does not exist"), err, "Test failed.")
}

func tempDirSFTP(s sftpClient) (string, error) {
	tempDir := fmt.Sprintf("/tempDir%v", uuid.NewString())

	// Create the temporary directory inside the SFTP server.
	if err := s.Mkdir(tempDir); err != nil {
		return "", err
	}

	return tempDir, nil
}

func Test_createNestedDir_SFTP(t *testing.T) {
	t.Skip("Skipping the test as while creating directory on GHA will get permission denied error")

	sftpCfg := &SFTPConfig{
		Host:     "localhost",
		User:     "myuser",
		Password: "mypass",
		Port:     2222,
	}

	s, err := newSFTPFile(sftpCfg, "testFile.txt", "rw")
	if err != nil {
		t.Fatalf("Error in creating connection to SFTP: %v", err)
	}

	client, ok := s.client.(sftpClient)
	if !ok {
		t.Fatalf("Unable to parse ftpConn")
	}

	// Create a temporary directory inside the container.
	tempDir, err := tempDirSFTP(client)
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	defer func() {
		// Remove the temporary directory after the test is done.
		if err := removeDirSFTP(client, tempDir); err != nil {
			t.Errorf("Failed to clean up test directories: %v", err)
		}
	}()

	testCases := []struct {
		desc string
		path string
	}{
		{"Success case: able to create dir", tempDir},
		{"Success case: parent dir already exists", tempDir + "/innerTestDir"},
		{"Success case: parent dir already exists", tempDir + "/innerTestDir/subDir"},
	}

	for i, tc := range testCases {
		err := createNestedDirSFTP(client, tc.path)

		assert.Nil(t, err, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}

func removeDirSFTP(s sftpClient, path string) error {
	// Use the same logic to split the path as in createNestedDirSFTP.
	// Reverse the order to remove inner directories first.
	dirs := strings.Split(path, "/")
	for i := len(dirs) - 1; i >= 0; i-- {
		dirPath := strings.Join(dirs[:i+1], "/")

		_, err := s.Stat(dirPath)
		if err != nil {
			// Directory doesn't exist, skip it.
			continue
		}

		// RemoveAll will remove the directory and its contents recursively.
		if err := sftpRemoveAll(s, dirPath); err != nil {
			return err
		}
	}

	return nil
}

func sftpRemoveAll(s sftpClient, path string) error {
	entries, err := listDirEntries(s, path)
	if err != nil {
		return err
	}

	err = removeEntries(s, entries, path)
	if err != nil {
		return err
	}

	err = removeDirectory(s, path)
	if err != nil {
		return err
	}

	return nil
}

func listDirEntries(s sftpClient, path string) ([]os.FileInfo, error) {
	return s.ReadDir(path)
}

func removeEntries(s sftpClient, entries []os.FileInfo, path string) error {
	for _, entry := range entries {
		entryPath := path + "/" + entry.Name()
		if entry.IsDir() {
			if err := sftpRemoveAll(s, entryPath); err != nil {
				return err
			}
		} else {
			if err := s.Remove(entryPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func removeDirectory(s sftpClient, path string) error {
	return s.RemoveDirectory(path)
}

func TestSftpConn(t *testing.T) {
	filename := "test.txt"
	mode := READWRITE
	conf := &SFTPConfig{Host: "localhost", User: "myuser", Password: "mypass", Port: 2222}
	sftpFile, err := newSFTPFile(conf, filename, mode)

	if err != nil {
		t.Fatalf("Unable to create SFTP session: %v", err)
	}

	s, ok := sftpFile.client.(sftpClient)

	if err = s.Close(); err != nil {
		t.Fatalf("Unable to close SFTP session: %v", err)
	}

	if !ok {
		t.Fatal("Unable to type assert")
	}

	_, err = s.ReadDir("testVal")
	assert.NotNil(t, err)

	_, err = s.Open("testVal")
	assert.NotNil(t, err)

	_, err = s.Create("testVal")
	assert.NotNil(t, err)

	err = s.Move("testSource", "testDestination")
	assert.NotNil(t, err)
}
