package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"net/textproto"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	errMockSentinel = errors.New("mocked error")
	errNotFound     = errors.New("mocked file not found")
)

func TestCreateFile(t *testing.T) {
	var tests = []struct {
		name           string
		fileName       string
		expectStorCall bool
		expectRetrCall bool
		expectError    bool
		mockStorError  bool
	}{
		{
			name:           "Successful creation",
			fileName:       "testfile.txt",
			expectStorCall: true,
			expectRetrCall: true,
			expectError:    false,
			mockStorError:  false,
		},
		{
			name:           "STOR method returns error",
			fileName:       "errorfile.txt",
			expectStorCall: true,
			expectRetrCall: false,
			expectError:    true,
			mockStorError:  true,
		},
		{
			name:           "Empty file name",
			fileName:       "",
			expectStorCall: false,
			expectRetrCall: false,
			expectError:    true,
			mockStorError:  false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	logger := NewMockLogger(ctrl)
	metrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  logger,
		metrics: metrics,
	}

	// mocked logs
	logger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()
	metrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectStorCall {
				emptyReader := new(bytes.Buffer)
				if tt.mockStorError {
					mockFtpConn.EXPECT().Stor("/ftp/one/"+tt.fileName, emptyReader).Return(errMockSentinel)
				} else {
					mockFtpConn.EXPECT().Stor("/ftp/one/"+tt.fileName, emptyReader).Return(nil)
					mockFtpConn.EXPECT().GetTime("/ftp/one/"+tt.fileName).Return(time.Now(), nil)
				}
			}

			if tt.expectRetrCall && tt.fileName != "" && !tt.mockStorError {
				mockResponse := NewMockftpResponse(ctrl)

				mockFtpConn.EXPECT().Retr("/ftp/one/"+tt.fileName).Return(mockResponse, nil)
				mockResponse.EXPECT().Close().Return(nil)
			}

			_, err := fs.Create(tt.fileName)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestRenameFile(t *testing.T) {
	var tests = []struct {
		name         string
		fromPath     string
		toPath       string
		expectRename bool
		expectError  bool
		mockError    bool
	}{
		{
			name:         "Successful rename",
			fromPath:     "testfile.txt",
			toPath:       "testfile_new.txt",
			expectRename: true,
			expectError:  false,
			mockError:    false,
		},
		{
			name:         "Rename with error",
			fromPath:     "testfile.txt",
			toPath:       "testfile_new.txt",
			expectRename: true,
			expectError:  true,
			mockError:    true,
		},
		{
			name:         "Empty from path",
			fromPath:     "",
			toPath:       "testfile_new.txt",
			expectRename: false,
			expectError:  true,
			mockError:    false,
		},
		{
			name:         "Empty to path",
			fromPath:     "testfile.txt",
			toPath:       "",
			expectRename: false,
			expectError:  true,
			mockError:    false,
		},
		{
			name:         "Same filename and toPath",
			fromPath:     "testfile.txt",
			toPath:       "testfile.txt",
			expectRename: false,
			expectError:  false,
			mockError:    false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Logf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectRename {
				if tt.mockError {
					mockFtpConn.EXPECT().Rename("/ftp/one/"+tt.fromPath, "/ftp/one/"+tt.toPath).Return(errMockSentinel)
				} else {
					mockFtpConn.EXPECT().Rename("/ftp/one/"+tt.fromPath, "/ftp/one/"+tt.toPath).Return(nil)
					mockFtpConn.EXPECT().GetTime("/ftp/one/"+tt.toPath).Return(time.Now(), nil)
				}
			}

			err := fs.Rename(tt.fromPath, tt.toPath)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestRemoveFile(t *testing.T) {
	var tests = []struct {
		name         string
		filePath     string
		expectDelete bool
		expectError  bool
		mockError    bool
	}{
		{
			name:         "Successful deletion",
			filePath:     "testfile.txt",
			expectDelete: true,
			expectError:  false,
			mockError:    false,
		},
		{
			name:         "Deletion with error",
			filePath:     "testfile.txt",
			expectDelete: true,
			expectError:  true,
			mockError:    true,
		},
		{
			name:         "Empty file path",
			filePath:     "",
			expectDelete: false,
			expectError:  true,
			mockError:    false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectDelete {
				if tt.mockError {
					mockFtpConn.EXPECT().Delete("/ftp/one/" + tt.filePath).Return(errMockSentinel)
				} else {
					mockFtpConn.EXPECT().Delete("/ftp/one/" + tt.filePath).Return(nil)
				}
			}

			err := fs.Remove(tt.filePath)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestOpenFile(t *testing.T) {
	var tests = []struct {
		name           string
		basePath       string
		filePath       string
		mockRetrExpect func(conn *MockserverConn, filePath string)
		expectError    bool
	}{
		{
			name:     "Successful open",
			basePath: "/ftp/one",
			filePath: "testfile_new.txt",
			mockRetrExpect: func(conn *MockserverConn, path string) {
				ctrl := gomock.NewController(t)

				response := NewMockftpResponse(ctrl)

				conn.EXPECT().Retr(path).Return(response, nil)
				response.EXPECT().Close().Return(nil)
				conn.EXPECT().GetTime("/ftp/one/testfile_new.txt").Return(time.Now(), nil)
			},
			expectError: false,
		},
		{
			name:     "Open with error",
			basePath: "/ftp/one",
			filePath: "nonexistent.txt",
			mockRetrExpect: func(conn *MockserverConn, path string) {
				conn.EXPECT().Retr(path).Return(nil, errMockSentinel)
			},
			expectError: true,
		},
		{
			name:     "empty path",
			basePath: "/ftp/one",
			filePath: "",
			mockRetrExpect: func(_ *MockserverConn, _ string) {
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes().AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathName := fmt.Sprintf("%v/%v", tt.basePath, tt.filePath)

			tt.mockRetrExpect(mockFtpConn, pathName)

			_, err := fs.Open(tt.filePath)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestMkDir(t *testing.T) {
	var tests = []struct {
		name            string
		basePath        string
		dirPath         string
		mockMkdirExpect func(conn *MockserverConn, dirPath string)
		expectError     bool
	}{
		{
			name:     "Successful mkdir",
			basePath: "/ftp/one",
			dirPath:  "directory1",
			mockMkdirExpect: func(conn *MockserverConn, dirPath string) {
				conn.EXPECT().MakeDir(dirPath).Return(nil)
			},
			expectError: false,
		},
		{
			name:     "Mkdir with error",
			basePath: "/ftp/one",
			dirPath:  "directory2",
			mockMkdirExpect: func(conn *MockserverConn, dirPath string) {
				conn.EXPECT().MakeDir(dirPath).Return(errMockSentinel)
			},
			expectError: true,
		},
		{
			name:     "Mkdir with empty directory path",
			basePath: "/ftp/one",
			dirPath:  "",
			mockMkdirExpect: func(_ *MockserverConn, _ string) {
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}
	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathName := fmt.Sprintf("%v/%v", tt.basePath, tt.dirPath)

			tt.mockMkdirExpect(mockFtpConn, pathName)

			err := fs.Mkdir(tt.dirPath, 0)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestMkDirAll(t *testing.T) {
	var directoryError = &textproto.Error{Code: 550, Msg: "Create directory operation failed."}

	var tests = []struct {
		name            string
		basePath        string
		dirPath         string
		mockMkdirExpect func(conn *MockserverConn, dirPath string)
		expectError     bool
	}{
		{
			name:     "Successful mkdir all",
			basePath: "/ftp/one",
			dirPath:  "testdir1/testdir2",
			mockMkdirExpect: func(conn *MockserverConn, _ string) {
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(directoryError)
				conn.EXPECT().MakeDir("testdir1").Return(nil)
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(nil)
			},
			expectError: false,
		},
		{
			name:     "empty path",
			basePath: "/ftp/one",
			dirPath:  "",
			mockMkdirExpect: func(_ *MockserverConn, _ string) {
			},
			expectError: true,
		},

		{
			name:     "one directory in the path exist",
			basePath: "/ftp/one",
			dirPath:  "testdir1/testdir2",
			mockMkdirExpect: func(conn *MockserverConn, _ string) {
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(directoryError)
				conn.EXPECT().MakeDir("testdir1").Return(directoryError)
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(nil)
			},
			expectError: false,
		},
		{
			name:     "Mkdir with error",
			basePath: "/ftp/one",
			dirPath:  "testdir1/testdir2",
			mockMkdirExpect: func(conn *MockserverConn, _ string) {
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(directoryError)
				conn.EXPECT().MakeDir("testdir1").Return(directoryError)
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(errMockSentinel)
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}
	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockMkdirExpect(mockFtpConn, tt.dirPath)

			err := fs.MkdirAll(tt.dirPath, 0)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestRemoveDir(t *testing.T) {
	var tests = []struct {
		name             string
		basePath         string
		removePath       string
		mockRemoveExpect func(conn *MockserverConn, basePath, removePath string)
		expectError      bool
	}{
		{
			name:       "Successful remove all",
			basePath:   "/ftp/one",
			removePath: "testdir1",
			mockRemoveExpect: func(conn *MockserverConn, basePath, removePath string) {
				conn.EXPECT().RemoveDirRecur(path.Join(basePath, removePath)).Return(nil)
			},
			expectError: false,
		},
		{
			name:       "Removing current directory",
			basePath:   "/ftp/one/testdir1",
			removePath: "../testdir1",
			mockRemoveExpect: func(conn *MockserverConn, basePath, removePath string) {
				conn.EXPECT().RemoveDirRecur(path.Join(basePath, removePath)).Return(nil)
			},
			expectError: false,
		},
		{
			name:       "RemoveAll with error",
			basePath:   "/ftp/one",
			removePath: "nonexistentdir",
			mockRemoveExpect: func(conn *MockserverConn, basePath, removePath string) {
				conn.EXPECT().RemoveDirRecur(path.Join(basePath, removePath)).Return(errMockSentinel)
			},
			expectError: true,
		},
		{
			name:       "Empty path",
			basePath:   "/ftp/one",
			removePath: "",
			mockRemoveExpect: func(_ *MockserverConn, _, _ string) {
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:     "ftp.example.com",
			User:     "username",
			Password: "password",
			Port:     21,
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs.config.RemoteDir = tt.basePath

			tt.mockRemoveExpect(mockFtpConn, tt.basePath, tt.removePath)

			err := fs.RemoveAll(tt.removePath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStat(t *testing.T) {
	var tests = []struct {
		name         string
		fileName     string
		mockEntry    []*ftp.Entry
		mockError    error
		expectError  bool
		expectedName string
	}{
		{
			name:     "Getting info of a file",
			fileName: "testfile.txt",
			mockEntry: []*ftp.Entry{
				{Name: "testfile.txt", Type: ftp.EntryTypeFile, Time: time.Now()},
			},
			mockError:    nil,
			expectError:  false,
			expectedName: "testfile.txt",
		},
		{
			name:        "Error getting info of a file",
			fileName:    "testfile.txt",
			mockEntry:   nil,
			mockError:   errMockSentinel,
			expectError: true,
		},
		{
			name:        "Empty String",
			expectError: true,
		},
		{
			name:         "Getting directory info",
			fileName:     "temp",
			mockEntry:    nil,
			mockError:    nil,
			expectError:  false,
			expectedName: "temp",
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
				"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

			filePath := path.Join(fs.config.RemoteDir, tt.fileName)

			if tt.mockEntry != nil || tt.mockError != nil {
				mockFtpConn.EXPECT().List(filePath).Return(tt.mockEntry, tt.mockError)
			}
			// if it is the fourth testcase
			if i == 3 {
				mockFtpConn.EXPECT().GetTime(filePath).Return(time.Now(), nil)
			}

			fileInfo, err := fs.Stat(tt.fileName)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedName, fileInfo.Name())
			}
		})
	}
}

func TestGetwd(t *testing.T) {
	var tests = []struct {
		name        string
		mockDir     string
		mockError   error
		expectError bool
	}{
		{
			name:        "Successful retrieval of path",
			mockDir:     "/ftp/hello",
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "error in retrieving path",
			mockDir:     "",
			mockError:   errMockSentinel,
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockFtpConn.EXPECT().CurrentDir().Return(tt.mockDir, tt.mockError)
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
				"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

			dir, err := fs.Getwd()

			if tt.expectError {
				require.Error(t, err)
				assert.Empty(t, dir)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.mockDir, dir)
			}
		})
	}
}

func TestChDir(t *testing.T) {
	var tests = []struct {
		name        string
		newDir      string
		mockError   error
		expectError bool
	}{
		{
			name:        "Successful change dir",
			newDir:      "newdir",
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "Change dir with error",
			newDir:      "errordir",
			mockError:   errMockSentinel,
			expectError: true,
		},
		{
			name:        "Empty String",
			newDir:      "",
			mockError:   nil,
			expectError: false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPath := path.Join(fs.config.RemoteDir, tt.newDir)

			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
			mockFtpConn.EXPECT().ChangeDir(newPath).Return(tt.mockError)
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
				"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

			err := fs.ChDir(tt.newDir)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, newPath, fs.config.RemoteDir)
			}
		})
	}
}

func TestReadDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	tests := getReadDirTestCases(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runReadDirTest(t, fs, mockFtpConn, mockLogger, mockMetrics, &tt)
		})
	}
}

func getReadDirTestCases(t *testing.T) []struct {
	name         string
	dir          string
	mockEntries  []*ftp.Entry
	mockError    error
	expectError  bool
	expectedName []string
} {
	t.Helper()

	return []struct {
		name         string
		dir          string
		mockEntries  []*ftp.Entry
		mockError    error
		expectError  bool
		expectedName []string
	}{
		{
			name: "Successful read dir",
			dir:  "someDir",
			mockEntries: []*ftp.Entry{
				{Name: "file1.txt", Type: ftp.EntryTypeFile, Time: time.Now()},
				{Name: "file2.txt", Type: ftp.EntryTypeFile, Time: time.Now()},
			},
			mockError:   nil,
			expectError: false,
			expectedName: []string{
				"file1.txt",
				"file2.txt",
			},
		},
		{
			name:        "Read dir with error",
			dir:         "someDir",
			mockEntries: nil,
			mockError:   errMockSentinel,
			expectError: true,
		},
		{
			name:        "Empty directory path",
			dir:         "",
			mockEntries: nil,
			mockError:   errMockSentinel,
			expectError: true,
		},
		{
			name: "Read current directory",
			dir:  ".",
			mockEntries: []*ftp.Entry{
				{Name: "hello", Type: ftp.EntryTypeFolder, Time: time.Now()},
				{Name: "file3.txt", Type: ftp.EntryTypeFile, Time: time.Now()},
			},
			mockError:   nil,
			expectError: false,
			expectedName: []string{
				"hello",
				"file3.txt",
			},
		},
	}
}

func runReadDirTest(
	t *testing.T,
	fs *FileSystem,
	mockFtpConn *MockserverConn,
	mockLogger *MockLogger,
	mockMetrics *MockMetrics,
	tt *struct {
		name         string
		dir          string
		mockEntries  []*ftp.Entry
		mockError    error
		expectError  bool
		expectedName []string
	},
) {
	t.Helper()

	pathName := fs.config.RemoteDir
	if tt.dir != "." {
		pathName = filepath.Join(fs.config.RemoteDir, tt.dir)
	}

	mockFtpConn.EXPECT().List(pathName).Return(tt.mockEntries, tt.mockError)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	files, err := fs.ReadDir(tt.dir)

	if tt.expectError {
		require.Error(t, err)
	} else {
		require.NoError(t, err)

		var names []string
		for _, file := range files {
			names = append(names, file.Name())
		}

		assert.ElementsMatch(t, tt.expectedName, names)
	}
}
