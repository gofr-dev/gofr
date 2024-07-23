package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"go.uber.org/mock/gomock"
	"testing"
)

// This test file contains test for all the ftpFileSystem functions.
// The ftp operations are mocked to check for various possible use cases.

func TestCreateFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Define test cases
	var createTests = []struct {
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

	for _, tt := range createTests {
		t.Run(tt.name, func(t *testing.T) {
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

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

			// Call the Create method
			_, err := fs.Create(tt.fileName)

			// Check expectations and errors
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestRenameFile(t *testing.T) {
	// Define test cases
	var renameTests = []struct {
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

	// Iterate over test cases
	for _, tt := range renameTests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			if tt.expectRename {
				if tt.mockError {
					mockFtpConn.EXPECT().Rename("/ftp/one/"+tt.fromPath, "/ftp/one/"+tt.toPath).Return(errors.New("mocked rename error"))
				} else {
					mockFtpConn.EXPECT().Rename("/ftp/one/"+tt.fromPath, "/ftp/one/"+tt.toPath).Return(nil)
				}
			}

			// Call Rename method
			err := fs.Rename(tt.fromPath, tt.toPath)

			// Check expectations and errors
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestRemoveFile(t *testing.T) {
	// Define test cases
	var deleteTests = []struct {
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

	// Iterate over test cases
	for _, tt := range deleteTests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			if tt.expectDelete {
				if tt.mockError {
					mockFtpConn.EXPECT().Delete("/ftp/one/" + tt.filePath).Return(errors.New("mocked delete error"))
				} else {
					mockFtpConn.EXPECT().Delete("/ftp/one/" + tt.filePath).Return(nil)
				}
			}

			// Call Remove method
			err := fs.Remove(tt.filePath)

			// Check expectations and errors
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestOpenFile(t *testing.T) {
	// Define test cases for Open method
	var openTests = []struct {
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

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for Open method
	for _, tt := range openTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Set mock expectations for Retr method
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.filePath)

			tt.mockRetrExpect(mockFtpConn, path)

			// Perform Open operation
			_, err := fs.Open(tt.filePath)

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful open
			if err == nil {
				t.Logf("Opened file: %s", tt.filePath)
			}
		})
	}
}

func TestOpenWithPerm(t *testing.T) {
	// Define test cases for OpenFile method
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

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for OpenFile method
	for _, tt := range openWithPermTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Set mock expectations for Retr method
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.filePath)
			tt.mockRetrExpect(mockFtpConn, path)

			// Perform OpenFile operation
			_, err := fs.OpenFile(tt.filePath, 0, 0075)

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful open
			if err == nil {
				t.Logf("Opened file with permissions: %s", tt.filePath)
			}
		})
	}
}

func TestMkDir(t *testing.T) {
	// Define test cases for Mkdir method
	var mkdirTests = []struct {
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

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for Mkdir method
	for _, tt := range mkdirTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Set mock expectations for MakeDir method
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.dirPath)
			tt.mockMkdirExpect(mockFtpConn, path)

			// Perform Mkdir operation
			err := fs.Mkdir(tt.dirPath, 0)

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful mkdir
			if err == nil {
				t.Logf("Created directory: %s", tt.dirPath)
			}
		})
	}
}

func TestMkDirAll(t *testing.T) {
	// Define test cases for MkdirAll method
	var mkdirAllTests = []struct {
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
			mockMkdirExpect: func(conn *MockServerConn, dirPath string) {
				conn.EXPECT().MakeDir("testdir1").Return(nil)
				conn.EXPECT().MakeDir("testdir1/testdir2").Return(nil)
			},
			expectError: false,
		},
	}

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for MkdirAll method
	for _, tt := range mkdirAllTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Set mock expectations for MakeDir method
			tt.mockMkdirExpect(mockFtpConn, tt.dirPath)

			// Perform MkdirAll operation
			err := fs.MkdirAll(tt.dirPath, 0)

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful mkdir all
			if err == nil {
				t.Logf("Created directories: %s", tt.dirPath)
			}
		})
	}
}

func TestRemoveDir(t *testing.T) {
	// Define test cases for RemoveAll method
	var removeAllTests = []struct {
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

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for RemoveAll method
	for _, tt := range removeAllTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Set mock expectations for RemoveDirRecur method
			path := fmt.Sprintf("%v/%v", tt.basePath, tt.removePath)
			tt.mockRemoveExpect(mockFtpConn, path)

			// Perform RemoveAll operation
			err := fs.RemoveAll(tt.removePath)

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful remove all
			if err == nil {
				t.Logf("Removed directory recursively: %s", tt.removePath)
			}
		})
	}
}
