package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
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

	mockFtpConn := NewMockServerConn(ctrl)
	logger := NewMockLogger(ctrl)
	metrics := NewMockMetrics(ctrl)

	fs := &fileSystem{
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
	logger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any())
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any())
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any())
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()
	metrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectStorCall {
				emptyReader := new(bytes.Buffer)
				if tt.mockStorError {
					mockFtpConn.EXPECT().Stor("/ftp/one/"+tt.fileName, emptyReader).Return(errors.New("mocked STOR error"))
				} else {
					mockFtpConn.EXPECT().Stor("/ftp/one/"+tt.fileName, emptyReader).Return(nil)
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

	mockFtpConn := NewMockServerConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &fileSystem{
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
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Logf(gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectRename {
				if tt.mockError {
					mockFtpConn.EXPECT().Rename("/ftp/one/"+tt.fromPath, "/ftp/one/"+tt.toPath).Return(errors.New("mocked rename error"))
				} else {
					mockFtpConn.EXPECT().Rename("/ftp/one/"+tt.fromPath, "/ftp/one/"+tt.toPath).Return(nil)
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

	mockFtpConn := NewMockServerConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &fileSystem{
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
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectDelete {
				if tt.mockError {
					mockFtpConn.EXPECT().Delete("/ftp/one/" + tt.filePath).Return(errors.New("mocked delete error"))
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
		mockRetrExpect func(conn *MockServerConn, filePath string)
		expectError    bool
	}{
		{
			name:     "Successful open",
			basePath: "/ftp/one",
			filePath: "testfile_new.txt",
			mockRetrExpect: func(conn *MockServerConn, path string) {
				ctrl := gomock.NewController(t)

				response := NewMockftpResponse(ctrl)

				conn.EXPECT().Retr(path).Return(response, nil)
				response.EXPECT().Close().Return(nil)
			},
			expectError: false,
		},
		{
			name:     "Open with error",
			basePath: "/ftp/one",
			filePath: "nonexistent.txt",
			mockRetrExpect: func(conn *MockServerConn, path string) {
				conn.EXPECT().Retr(path).Return(nil, errors.New("mocked open error"))
			},
			expectError: true,
		},
		{
			name:     "empty path",
			basePath: "/ftp/one",
			filePath: "",
			mockRetrExpect: func(_ *MockServerConn, _ string) {
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &fileSystem{
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
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.filePath)

			tt.mockRetrExpect(mockFtpConn, path)

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
		mockMkdirExpect func(conn *MockServerConn, dirPath string)
		expectError     bool
	}{
		{
			name:     "Successful mkdir",
			basePath: "/ftp/one",
			dirPath:  "directory1",
			mockMkdirExpect: func(conn *MockServerConn, dirPath string) {
				conn.EXPECT().MakeDir(dirPath).Return(nil)
			},
			expectError: false,
		},
		{
			name:     "Mkdir with error",
			basePath: "/ftp/one",
			dirPath:  "directory2",
			mockMkdirExpect: func(conn *MockServerConn, dirPath string) {
				conn.EXPECT().MakeDir(dirPath).Return(errors.New("mocked mkdir error"))
			},
			expectError: true,
		},
		{
			name:     "Mkdir with empty directory path",
			basePath: "/ftp/one",
			dirPath:  "",
			mockMkdirExpect: func(_ *MockServerConn, _ string) {
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &fileSystem{
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
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.dirPath)

			tt.mockMkdirExpect(mockFtpConn, path)

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
		mockMkdirExpect func(conn *MockServerConn, dirPath string)
		expectError     bool
	}{
		{
			name:     "Successful mkdir all",
			basePath: "/ftp/one",
			dirPath:  "testdir1/testdir2",
			mockMkdirExpect: func(conn *MockServerConn, _ string) {
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
			mockMkdirExpect: func(_ *MockServerConn, _ string) {
			},
			expectError: true,
		},

		{
			name:     "one directory in the path exist",
			basePath: "/ftp/one",
			dirPath:  "testdir1/testdir2",
			mockMkdirExpect: func(conn *MockServerConn, _ string) {
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
			mockMkdirExpect: func(conn *MockServerConn, _ string) {
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(directoryError)
				conn.EXPECT().MakeDir("testdir1").Return(directoryError)
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(errors.New("mocked mkdir error"))
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &fileSystem{
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
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
		mockRemoveExpect func(conn *MockServerConn, removePath string)
		expectError      bool
	}{
		{
			name:       "Successful remove all",
			basePath:   "/ftp/one",
			removePath: "testdir1",
			mockRemoveExpect: func(conn *MockServerConn, removePath string) {
				conn.EXPECT().RemoveDirRecur(removePath).Return(nil)
			},
			expectError: false,
		},
		{
			name:       "Remove all with error",
			basePath:   "/ftp/one",
			removePath: "nonexistentdir",
			mockRemoveExpect: func(conn *MockServerConn, removePath string) {
				conn.EXPECT().RemoveDirRecur(removePath).Return(errors.New("mocked remove error"))
			},
			expectError: true,
		},
		{
			name:       "empty path",
			basePath:   "/ftp/one",
			removePath: "",
			mockRemoveExpect: func(_ *MockServerConn, _ string) {
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &fileSystem{
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
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.removePath)

			tt.mockRemoveExpect(mockFtpConn, path)

			err := fs.RemoveAll(tt.removePath)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}
