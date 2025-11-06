package file

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/googleapi"
)

var errTest = fmt.Errorf("test error")

// setupCommonFS is a helper function to set up common test dependencies.
func setupCommonFS(t *testing.T) (*gomock.Controller, *MockStorageProvider, *CommonFileSystem) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockProvider := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Set up common expectations that all operations use
	mockLogger.EXPECT().
		Debug(gomock.Any()).
		AnyTimes()

	mockMetrics.EXPECT().
		RecordHistogram(gomock.Any(), AppFileStats, gomock.Any(), gomock.Any()).
		AnyTimes()

	fs := &CommonFileSystem{
		Provider: mockProvider,
		Location: "test-bucket",
		Logger:   mockLogger,
		Metrics:  mockMetrics,
	}

	return ctrl, mockProvider, fs
}

func TestCommonFileSystem_MkdirAll(t *testing.T) {
	tests := []struct {
		name        string
		dirPath     string
		expectError bool
	}{
		{
			name:        "single level directory",
			dirPath:     "testdir",
			expectError: false,
		},
		{
			name:        "nested directories",
			dirPath:     "parent/child/grandchild",
			expectError: false,
		},
		{
			name:        "empty path",
			dirPath:     "",
			expectError: false,
		},
		{
			name:        "path with leading slash",
			dirPath:     "/parent/child",
			expectError: false,
		},
		{
			name:        "path with trailing slash",
			dirPath:     "parent/child/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockProvider, fs := setupCommonFS(t)
			defer ctrl.Finish()

			if tt.dirPath != "" {
				cleaned := strings.Trim(tt.dirPath, "/")
				dirs := strings.Split(cleaned, "/")

				for range dirs {
					mockWriter := NewMockWriteCloser(ctrl)

					mockProvider.EXPECT().
						NewWriter(gomock.Any(), gomock.Any()).
						Return(mockWriter)

					mockWriter.EXPECT().
						Write([]byte("")).
						Return(0, nil)

					mockWriter.EXPECT().
						Close().
						Return(nil).
						AnyTimes()
				}
			}

			err := fs.MkdirAll(tt.dirPath, os.ModePerm)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCommonFileSystem_RemoveAll(t *testing.T) {
	tests := []struct {
		name        string
		dirPath     string
		objects     []string
		listErr     error
		deleteErr   error
		expectError bool
	}{
		{
			name:    "successful removal",
			dirPath: "testdir",
			objects: []string{
				"testdir/file1.txt",
				"testdir/file2.txt",
				"testdir/subdir/",
			},
			expectError: false,
		},
		{
			name:        "empty directory",
			dirPath:     "emptydir",
			objects:     []string{},
			expectError: false,
		},
		{
			name:        "list error",
			dirPath:     "testdir",
			listErr:     errTest,
			expectError: true,
		},
		{
			name:    "delete error",
			dirPath: "testdir",
			objects: []string{
				"testdir/file1.txt",
			},
			deleteErr:   errTest,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockProvider, fs := setupCommonFS(t)
			defer ctrl.Finish()

			if tt.listErr != nil {
				mockProvider.EXPECT().
					ListObjects(gomock.Any(), tt.dirPath).
					Return(nil, tt.listErr)
			} else {
				mockProvider.EXPECT().
					ListObjects(gomock.Any(), tt.dirPath).
					Return(tt.objects, nil)

				if tt.deleteErr != nil {
					mockProvider.EXPECT().
						DeleteObject(gomock.Any(), tt.objects[0]).
						Return(tt.deleteErr)
				} else {
					for _, obj := range tt.objects {
						mockProvider.EXPECT().
							DeleteObject(gomock.Any(), obj).
							Return(nil)
					}
				}
			}

			err := fs.RemoveAll(tt.dirPath)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCommonFileSystem_ReadDir(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		dir           string
		objects       []ObjectInfo
		prefixes      []string
		listErr       error
		expectedCount int
		expectError   bool
	}{
		{
			name: "successful read with files and directories",
			dir:  "testdir",
			objects: []ObjectInfo{
				{Name: "testdir/file1.txt", Size: 100, LastModified: now, IsDir: false},
				{Name: "testdir/file2.pdf", Size: 200, LastModified: now, IsDir: false},
			},
			prefixes: []string{
				"testdir/subdir1/",
				"testdir/subdir2/",
			},
			expectedCount: 4,
			expectError:   false,
		},
		{
			name:          "empty directory",
			dir:           "emptydir",
			objects:       []ObjectInfo{},
			prefixes:      []string{},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name: "directory with marker objects (should be skipped)",
			dir:  "testdir",
			objects: []ObjectInfo{
				{Name: "testdir/", Size: 0, IsDir: true},
				{Name: "testdir/file.txt", Size: 100, LastModified: now, IsDir: false},
			},
			prefixes:      []string{},
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:        "list error",
			dir:         "testdir",
			listErr:     errTest,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockProvider, fs := setupCommonFS(t)
			defer ctrl.Finish()

			if tt.listErr != nil {
				mockProvider.EXPECT().
					ListDir(gomock.Any(), tt.dir).
					Return(nil, nil, tt.listErr)
			} else {
				mockProvider.EXPECT().
					ListDir(gomock.Any(), tt.dir).
					Return(tt.objects, tt.prefixes, nil)
			}

			fileInfos, err := fs.ReadDir(tt.dir)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, fileInfos, tt.expectedCount)
			}
		})
	}
}

func TestCommonFileSystem_Stat(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		fileName    string
		statInfo    *ObjectInfo
		statErr     error
		listObjects []ObjectInfo
		listErr     error
		expectError bool
		expectedDir bool
	}{
		{
			name:     "successful stat for file",
			fileName: "file.txt",
			statInfo: &ObjectInfo{
				Name:         "file.txt",
				Size:         100,
				ContentType:  "text/plain",
				LastModified: now,
				IsDir:        false,
			},
			expectError: false,
			expectedDir: false,
		},
		{
			name:     "file not found, but directory exists",
			fileName: "testdir",
			statErr:  errTest,
			listObjects: []ObjectInfo{
				{Name: "testdir/file.txt", Size: 100, LastModified: now, IsDir: false},
			},
			expectError: false,
			expectedDir: true,
		},
		{
			name:        "file not found, directory also not found",
			fileName:    "nonexistent",
			statErr:     errTest,
			listObjects: []ObjectInfo{},
			expectError: true,
		},
		{
			name:        "stat error and list error",
			fileName:    "testfile",
			statErr:     errTest,
			listErr:     errTest,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockProvider, fs := setupCommonFS(t)
			defer ctrl.Finish()

			if tt.statErr != nil {
				mockProvider.EXPECT().
					StatObject(gomock.Any(), tt.fileName).
					Return(nil, tt.statErr)

				prefix := tt.fileName
				if !strings.HasSuffix(prefix, "/") {
					prefix += "/"
				}

				if tt.listErr != nil {
					mockProvider.EXPECT().
						ListDir(gomock.Any(), prefix).
						Return(nil, nil, tt.listErr)
				} else {
					mockProvider.EXPECT().
						ListDir(gomock.Any(), prefix).
						Return(tt.listObjects, nil, nil)
				}
			} else {
				mockProvider.EXPECT().
					StatObject(gomock.Any(), tt.fileName).
					Return(tt.statInfo, nil)
			}

			fileInfo, err := fs.Stat(tt.fileName)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, fileInfo)
				assert.Equal(t, tt.expectedDir, fileInfo.IsDir())
			}
		})
	}
}

func TestCommonFileSystem_ChDir(t *testing.T) {
	ctrl, _, fs := setupCommonFS(t)
	defer ctrl.Finish()

	err := fs.ChDir("some/dir")

	require.Error(t, err)
	assert.Equal(t, errChDirNotSupported, err)
}

func TestCommonFileSystem_Getwd(t *testing.T) {
	tests := []struct {
		name         string
		location     string
		expectedPath string
	}{
		{
			name:         "bucket name",
			location:     "my-bucket",
			expectedPath: "my-bucket",
		},
		{
			name:         "FTP connection",
			location:     "ftp://host",
			expectedPath: "ftp://host",
		},
		{
			name:         "empty location",
			location:     "",
			expectedPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProvider := NewMockStorageProvider(ctrl)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			// Set up observability expectations
			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

			fs := &CommonFileSystem{
				Provider: mockProvider,
				Location: tt.location,
				Logger:   mockLogger,
				Metrics:  mockMetrics,
			}

			path, err := fs.Getwd()

			require.NoError(t, err)
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

// Mock WriteCloser for testing.
type MockWriteCloser struct {
	ctrl     *gomock.Controller
	recorder *MockWriteCloserMockRecorder
}

type MockWriteCloserMockRecorder struct {
	mock *MockWriteCloser
}

func NewMockWriteCloser(ctrl *gomock.Controller) *MockWriteCloser {
	mock := &MockWriteCloser{ctrl: ctrl}
	mock.recorder = &MockWriteCloserMockRecorder{mock}

	return mock
}

func (m *MockWriteCloser) EXPECT() *MockWriteCloserMockRecorder {
	return m.recorder
}

func (m *MockWriteCloser) Write(p []byte) (n int, err error) {
	ret := m.ctrl.Call(m, "Write", p)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

func (m *MockWriteCloserMockRecorder) Write(p any) *gomock.Call {
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "Write", reflect.TypeOf((*MockWriteCloser)(nil).Write), p)
}

func (m *MockWriteCloser) Close() error {
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)

	return ret0
}

func (m *MockWriteCloserMockRecorder) Close() *gomock.Call {
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "Close", reflect.TypeOf((*MockWriteCloser)(nil).Close))
}

func TestIsAlreadyExistsError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "GCS error code 409",
			err:      &googleapi.Error{Code: 409},
			expected: true,
		},
		{
			name:     "GCS error code 412",
			err:      &googleapi.Error{Code: 412},
			expected: true,
		},
		{
			name:     "GCS error code 404",
			err:      &googleapi.Error{Code: 404},
			expected: false,
		},
		{
			name:     "unrelated error",
			err:      errTest,
			expected: false,
		},
		{
			name:     "wrapped GCS error 409",
			err:      &googleapi.Error{Code: 409, Message: "Object already exists"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlreadyExistsError(tt.err)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateCopyName(t *testing.T) {
	tests := []struct {
		name     string
		original string
		count    int
		expected string
	}{
		{
			name:     "simple file with extension",
			original: "file.txt",
			count:    1,
			expected: "file copy 1.txt",
		},
		{
			name:     "file with multiple dots",
			original: "document.backup.tar.gz",
			count:    2,
			expected: "document.backup.tar copy 2.gz",
		},
		{
			name:     "file without extension",
			original: "README",
			count:    1,
			expected: "README copy 1",
		},
		{
			name:     "file with path",
			original: "folder/subfolder/file.pdf",
			count:    3,
			expected: "folder/subfolder/file copy 3.pdf",
		},
		{
			name:     "file with high count number",
			count:    999,
			expected: " copy 999",
		},
		{
			name:     "file with spaces in name",
			original: "my document.docx",
			count:    1,
			expected: "my document copy 1.docx",
		},
		{
			name:     "hidden file",
			original: ".gitignore",
			count:    1,
			expected: " copy 1.gitignore",
		},
		{
			name:     "file with leading dot only",
			original: ".config",
			count:    2,
			expected: " copy 2.config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateCopyName(tt.original, tt.count)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCommonFileSystem_Mkdir(t *testing.T) {
	tests := []struct {
		name        string
		dirName     string
		writeErr    error
		closeErr    error
		expectError bool
		expectedErr error
	}{
		{
			name:        "successful directory creation",
			dirName:     "testdir",
			expectError: false,
		},
		{
			name:        "directory with trailing slash",
			dirName:     "testdir/",
			expectError: false,
		},
		{
			name:        "nested directory path",
			dirName:     "parent/child",
			expectError: false,
		},
		{
			name:        "empty directory name",
			dirName:     "",
			expectError: true,
			expectedErr: errEmptyDirectoryName,
		},
		{
			name:        "write error",
			dirName:     "testdir",
			writeErr:    errTest,
			expectError: true,
		},
		{
			name:        "close error",
			dirName:     "testdir",
			closeErr:    errTest,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockProvider, fs := setupCommonFS(t)
			defer ctrl.Finish()

			setupMkdirMocks(ctrl, mockProvider, tt.dirName, tt.writeErr, tt.closeErr)

			err := fs.Mkdir(tt.dirName, os.ModePerm)

			assertTestResult(t, err, tt.expectError, tt.expectedErr)
		})
	}
}

// setupMkdirMocks configures mock expectations for Mkdir operation.
func setupMkdirMocks(ctrl *gomock.Controller, mockProvider *MockStorageProvider,
	dirName string, writeErr, closeErr error) {
	if dirName == "" {
		return
	}

	mockWriter := NewMockWriteCloser(ctrl)

	expectedName := dirName
	if !strings.HasSuffix(expectedName, "/") {
		expectedName += "/"
	}

	mockProvider.EXPECT().
		NewWriter(gomock.Any(), expectedName).
		Return(mockWriter)

	mockWriter.EXPECT().
		Write([]byte("")).
		Return(0, writeErr)

	configureMockClose(mockWriter, closeErr)
}

// assertTestResult validates test outcomes without branching logic.
func assertTestResult(t *testing.T, err error, expectError bool, expectedErr error) {
	t.Helper()

	assert.Equal(t, expectError, err != nil, "Error expectation mismatch")

	if expectedErr != nil {
		assert.Equal(t, expectedErr, err)
	}
}

// configureMockClose sets up Close() expectations based on error conditions.
func configureMockClose(mockWriter *MockWriteCloser, closeErr error) {
	if closeErr != nil {
		mockWriter.EXPECT().Close().Return(closeErr).AnyTimes()
		return
	}

	mockWriter.EXPECT().Close().Return(nil).AnyTimes()
}
