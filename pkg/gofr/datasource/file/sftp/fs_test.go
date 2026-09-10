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

	require.ErrorIs(t, err, errChDirNotSupported, "TEST[%d] Failed. Desc %v")
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

// captureOperationLogs returns a FileSystem whose logger records every FileLog
// emitted by sendOperationStats, so tests can assert the operation name and the
// success/error status that observability actually reports.
func captureOperationLogs(t *testing.T) (FileSystem, *MocksftpClient, *[]FileLog) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mockClient := NewMocksftpClient(ctrl)
	mockLogger := NewMockLogger(ctrl)

	logs := make([]FileLog, 0)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes().Do(func(args ...any) {
		if fl, ok := args[0].(*FileLog); ok {
			logs = append(logs, *fl)
		}
	})
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any()).AnyTimes()

	return FileSystem{logger: mockLogger, client: mockClient}, mockClient, &logs
}

// TestFiles_OperationLogs_NameAndStatus pins the operation name and status that
// each method reports. It regression-guards three defects: Getwd reported STAT,
// MkdirAll reported MKDIR, and a failed OpenFile reported SUCCESS.
func TestFiles_OperationLogs_NameAndStatus(t *testing.T) {
	testCases := []struct {
		desc      string
		setup     func(c *MocksftpClient)
		call      func(f FileSystem)
		expOp     string
		expStatus string
	}{
		{
			desc:      "Getwd reports GETWD, not STAT",
			setup:     func(c *MocksftpClient) { c.EXPECT().Getwd().Return("/home", nil) },
			call:      func(f FileSystem) { _, _ = f.Getwd() },
			expOp:     File.OpGetwd,
			expStatus: statusSuccess,
		},
		{
			desc:      "MkdirAll reports MKDIR_ALL, not MKDIR",
			setup:     func(c *MocksftpClient) { c.EXPECT().MkdirAll("/a/b").Return(nil) },
			call:      func(f FileSystem) { _ = f.MkdirAll("/a/b", 0) },
			expOp:     File.OpMkdirAll,
			expStatus: statusSuccess,
		},
		{
			desc:      "Mkdir still reports MKDIR",
			setup:     func(c *MocksftpClient) { c.EXPECT().Mkdir("/a").Return(nil) },
			call:      func(f FileSystem) { _ = f.Mkdir("/a", 0) },
			expOp:     File.OpMkdir,
			expStatus: statusSuccess,
		},
		{
			desc: "failed OpenFile reports ERROR, not SUCCESS",
			setup: func(c *MocksftpClient) {
				c.EXPECT().OpenFile("x.csv", 0).Return(nil, errOpenFile)
			},
			call:      func(f FileSystem) { _, _ = f.OpenFile("x.csv", 0, 0) },
			expOp:     File.OpOpenFile,
			expStatus: statusError,
		},
		{
			desc:      "ChDir reports CHDIR with ERROR",
			setup:     func(_ *MocksftpClient) {},
			call:      func(f FileSystem) { _ = f.ChDir("/a") },
			expOp:     File.OpChDir,
			expStatus: statusError,
		},
	}

	for i, tc := range testCases {
		fs, client, logs := captureOperationLogs(t)

		tc.setup(client)
		tc.call(fs)

		require.Len(t, *logs, 1, "TEST[%d] Failed. Desc %v", i, tc.desc)
		require.Equal(t, tc.expOp, (*logs)[0].Operation, "TEST[%d] Failed. Desc %v", i, tc.desc)
		require.Equal(t, tc.expStatus, *(*logs)[0].Status, "TEST[%d] Failed. Desc %v", i, tc.desc)
	}
}

// TestFiles_ChDir_ReturnsError guards against ChDir silently reporting success
// for an operation the SFTP client does not implement.
func TestFiles_ChDir_ReturnsError(t *testing.T) {
	files, mocks := getMocks(t)

	mocks.logger.EXPECT().Errorf("Chdir is not implemented for SFTP")

	require.ErrorIs(t, files.ChDir("any"), errChDirNotSupported)
}
