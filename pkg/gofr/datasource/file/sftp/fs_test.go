package sftp

import (
	"errors"
	"os"
	"testing"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	File "gofr.dev/pkg/gofr/datasource/file"
)

type mocks struct {
	client  *MocksftpClient
	logger  *MockLogger
	metrics *MockMetrics
	file    *File.MockFile
}

var (
	errCreateFile = errors.New("failed to create file")
	errOpenFile   = errors.New("failed to open file")
)

func getMocks(t *testing.T) (FileSystem, mocks) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mockClient := NewMocksftpClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockFile := File.NewMockFile(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	client := FileSystem{logger: mockLogger, metrics: mockMetrics, client: mockClient}

	return client, mocks{
		client:  mockClient,
		logger:  mockLogger,
		metrics: mockMetrics,
		file:    mockFile,
	}
}

func TestFiles_Mkdir(t *testing.T) {
	files, mocks := getMocks(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"directory created successfully", nil},
		{"directory creation failed", errCreateFile},
	}

	for _, tc := range testCases {
		mocks.client.EXPECT().Mkdir("/test").Return(tc.err)

		err := files.Mkdir("/test", 1)

		require.Equal(t, tc.err, err, "TEST[%d] Failed. Desc %v")
	}
}

func TestFiles_MkdirAll(t *testing.T) {
	files, mocks := getMocks(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"directory created successfully", nil},
		{"directory creation failed", errCreateFile},
	}

	for _, tc := range testCases {
		mocks.client.EXPECT().MkdirAll("/test").Return(tc.err)

		err := files.MkdirAll("/test", 1)

		require.Equal(t, tc.err, err, "TEST[%d] Failed. Desc %v")
	}
}

func TestFiles_Remove(t *testing.T) {
	files, mocks := getMocks(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"directory removed successfully", nil},
		{"directory removal failed", errCreateFile},
	}

	for _, tc := range testCases {
		mocks.client.EXPECT().Remove("/test").Return(tc.err)

		err := files.Remove("/test")

		require.Equal(t, tc.err, err, "TEST[%d] Failed. Desc %v")
	}
}

func TestFiles_RemoveAll(t *testing.T) {
	files, mocks := getMocks(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"directory removed successfully", nil},
		{"directory removal failed", errCreateFile},
	}

	for _, tc := range testCases {
		mocks.client.EXPECT().RemoveAll("/test/upload").Return(tc.err)

		err := files.RemoveAll("/test/upload")

		require.Equal(t, tc.err, err, "TEST[%d] Failed. Desc %v")
	}
}

func TestFiles_Rename(t *testing.T) {
	files, mocks := getMocks(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"directory renamed successfully", nil},
		{"directory rename failed", errCreateFile},
	}

	for i, tc := range testCases {
		mocks.client.EXPECT().Rename("test.csv", "new_test.csv").Return(tc.err)

		err := files.Rename("test.csv", "new_test.csv")

		require.Equal(t, tc.err, err, "TEST[%d] Failed. Desc %v", i, tc.desc)
	}
}

func TestFiles_ChDir(t *testing.T) {
	files, mocks := getMocks(t)

	mocks.logger.EXPECT().Errorf("Chdir is not implemented for SFTP")

	err := files.ChDir("test.csv")

	require.NoError(t, err, "TEST[%d] Failed. Desc %v")
}

func TestFiles_GetWd(t *testing.T) {
	files, mocks := getMocks(t)

	testCases := []struct {
		desc string
		name string
		err  error
	}{
		{"directory renamed successfully", "file", nil},
		{"directory rename failed", "", errCreateFile},
	}

	for i, tc := range testCases {
		mocks.client.EXPECT().Getwd().Return(tc.name, tc.err)

		name, err := files.Getwd()

		require.Equal(t, tc.name, name, "TEST[%d] Failed. Desc %v", i, tc.desc)
		require.Equal(t, tc.err, err, "Test[%d] Failed.\n DESC %v", i, tc.desc)
	}
}

func TestFiles_Create(t *testing.T) {
	client, mocks := getMocks(t)

	mockSftpFile := sftp.File{}

	testCases := []struct {
		desc     string
		name     string
		expFile  File.File
		expError error
	}{
		{"File Created Successfully", "text.csv", sftpFile{File: &mockSftpFile, logger: mocks.logger}, nil},
		{"File Creation Failed", "text.csv", nil, errCreateFile},
	}

	for i, tc := range testCases {
		mocks.client.EXPECT().Create(tc.name).Return(&mockSftpFile, tc.expError)

		createdFile, err := client.Create(tc.name)

		require.Equal(t, tc.expFile, createdFile, "Test[%d] Failed.\n DESC %v", i, tc.desc)
		require.Equal(t, tc.expError, err, "Test[%d] Failed.\n DESC %v", i, tc.desc)
	}
}

func TestFiles_Open(t *testing.T) {
	client, mocks := getMocks(t)

	mockSftpFile := sftp.File{}

	testCases := []struct {
		desc     string
		name     string
		expFile  File.File
		expError error
	}{
		{"File Opened Successfully", "text.csv", sftpFile{File: &mockSftpFile, logger: mocks.logger}, nil},
		{"File Open Failed", "text.csv", nil, errCreateFile},
	}

	for i, tc := range testCases {
		mocks.client.EXPECT().Open(tc.name).Return(&mockSftpFile, tc.expError)

		openedFile, err := client.Open(tc.name)

		require.Equal(t, tc.expFile, openedFile, "Test[%d] Failed.\n DESC %v", i, tc.desc)
		require.Equal(t, tc.expError, err, "Test[%d] Failed.\n DESC %v", i, tc.desc)
	}
}

func TestFiles_OpenFile(t *testing.T) {
	client, mocks := getMocks(t)

	mockSftpFile := sftp.File{}

	testCases := []struct {
		desc     string
		name     string
		expFile  File.File
		expError error
	}{
		{"File Opened Successfully", "text.csv", sftpFile{File: &mockSftpFile, logger: mocks.logger}, nil},
		{"File Open Failed", "text.csv", nil, errOpenFile},
	}

	for i, tc := range testCases {
		mocks.client.EXPECT().OpenFile(tc.name, 0).Return(&mockSftpFile, tc.expError)

		openedFile, err := client.OpenFile(tc.name, 0, 0)

		require.Equal(t, tc.expFile, openedFile, "Test[%d] Failed.\n DESC %v", i, tc.desc)
		require.Equal(t, tc.expError, err, "Test[%d] Failed.\n DESC %v", i, tc.desc)
	}
}

func TestFiles_ReadDir(t *testing.T) {
	client, mocks := getMocks(t)

	file, _ := os.CreateTemp("temp", "t")
	file.Close()

	info, _ := file.Stat()

	osFile := []os.FileInfo{info}

	testCases := []struct {
		desc     string
		name     string
		expFile  []File.FileInfo
		expError error
	}{
		{"Dir Read Successfully", "text.csv", []File.FileInfo{info}, nil},
		{"Dir Read Failed", "text.csv", nil, errCreateFile},
	}

	for i, tc := range testCases {
		mocks.client.EXPECT().ReadDir(tc.name).Return(osFile, tc.expError)

		createdFile, err := client.ReadDir(tc.name)

		require.Equal(t, tc.expFile, createdFile, "Test[%d] Failed.\n DESC %v", i, tc.desc)
		require.Equal(t, tc.expError, err, "Test[%d] Failed.\n DESC %v", i, tc.desc)
	}
}

func TestFiles_Stat(t *testing.T) {
	client, mocks := getMocks(t)

	file, _ := os.CreateTemp("temp", "t")
	file.Close()

	info, _ := file.Stat()

	testCases := []struct {
		desc     string
		name     string
		expFile  File.FileInfo
		expError error
	}{
		{"File Stat Successfully Returned", "text.csv", info, nil},
		{"File Stat Fetch Failed", "text.csv", nil, errCreateFile},
	}

	for i, tc := range testCases {
		mocks.client.EXPECT().Stat(tc.name).Return(info, tc.expError)

		createdFile, err := client.Stat(tc.name)

		require.Equal(t, tc.expFile, createdFile, "Test[%d] Failed.\n DESC %v", i, tc.desc)
		require.Equal(t, tc.expError, err, "Test[%d] Failed.\n DESC %v", i, tc.desc)
	}
}
