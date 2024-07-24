package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// This test file contains test for all the ftpFileSystem functions.
// The ftp operations are mocked to check for various possible use cases.
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

	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewMockLogger(INFO))

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

			if tt.expectRetrCall {
				Response := NewMockftpResponse(ctrl)

				mockFtpConn.EXPECT().Retr("/ftp/one/"+tt.fileName).Return(Response, nil)
				Response.EXPECT().Close().Return(nil)
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
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewMockLogger(INFO))

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

	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewMockLogger(INFO))

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
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewMockLogger(INFO))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.filePath)

			tt.mockRetrExpect(mockFtpConn, path)

			_, err := fs.Open(tt.filePath)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestOpenWithPerm(t *testing.T) {
	var openWithPermTests = []struct {
		name           string
		basePath       string
		filePath       string
		mockRetrExpect func(conn *MockServerConn, filePath string)
		expectError    bool
	}{
		{
			name:     "Successful open with permissions",
			basePath: "/ftp/one",
			filePath: "/ftp/one/testfile_new.txt",
			mockRetrExpect: func(conn *MockServerConn, path string) {
				ctrl := gomock.NewController(t)
				response := NewMockftpResponse(ctrl)
				conn.EXPECT().Retr(path).Return(response, nil)
			},
			expectError: false,
		},
		{
			name:     "Open with permissions and error",
			basePath: "/ftp/one",
			filePath: "/ftp/one/nonexistent.txt",
			mockRetrExpect: func(conn *MockServerConn, path string) {
				conn.EXPECT().Retr(path).Return(nil, errors.New("mocked open error"))
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewMockLogger(INFO))

	for _, tt := range openWithPermTests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.filePath)

			tt.mockRetrExpect(mockFtpConn, path)

			_, err := fs.OpenFile(tt.filePath, 0, 0075)

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
			dirPath:  "/ftp/one/directory1",
			mockMkdirExpect: func(conn *MockServerConn, dirPath string) {
				conn.EXPECT().MakeDir(dirPath).Return(nil)
			},
			expectError: false,
		},
		{
			name:     "Mkdir with error",
			basePath: "/ftp/one",
			dirPath:  "/ftp/one/directory2",
			mockMkdirExpect: func(conn *MockServerConn, dirPath string) {
				conn.EXPECT().MakeDir(dirPath).Return(errors.New("mocked mkdir error"))
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewMockLogger(INFO))

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
				conn.EXPECT().MakeDir("testdir1").Return(nil)
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(nil)
			},
			expectError: false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewMockLogger(INFO))

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
			removePath: "/ftp/one/testdir1",
			mockRemoveExpect: func(conn *MockServerConn, removePath string) {
				conn.EXPECT().RemoveDirRecur(removePath).Return(nil)
			},
			expectError: false,
		},
		{
			name:       "Remove all with error",
			basePath:   "/ftp/one",
			removePath: "/ftp/one/nonexistentdir",
			mockRemoveExpect: func(conn *MockServerConn, removePath string) {
				conn.EXPECT().RemoveDirRecur(removePath).Return(errors.New("mocked remove error"))
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewMockLogger(INFO))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.removePath)

			tt.mockRemoveExpect(mockFtpConn, path)

			err := fs.RemoveAll(tt.removePath)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}
