package file

import (
	"bytes"
	"fmt"
	"io"
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

			if tt.dirName != "" {
				expectedName := tt.dirName
				if !strings.HasSuffix(expectedName, "/") {
					expectedName += "/"
				}

				// Mock StatObject to indicate directory doesn't exist
				mockProvider.EXPECT().
					StatObject(gomock.Any(), expectedName).
					Return(nil, errTest)

				mockWriter := NewMockWriteCloser(ctrl)

				mockProvider.EXPECT().
					NewWriter(gomock.Any(), expectedName).
					Return(mockWriter)

				mockWriter.EXPECT().
					Write([]byte("")).
					Return(0, tt.writeErr)

				if tt.closeErr != nil {
					mockWriter.EXPECT().Close().Return(tt.closeErr).AnyTimes()
				} else {
					mockWriter.EXPECT().Close().Return(nil).AnyTimes()
				}
			}

			err := fs.Mkdir(tt.dirName, os.ModePerm)

			if tt.expectError {
				require.Error(t, err)

				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
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

func TestCommonFileSystem_Create_Success(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockWriter := NewMockWriteCloser(ctrl)

	mockProvider.EXPECT().
		NewWriter(gomock.Any(), "newfile.txt").
		Return(mockWriter)

	f, err := fs.Create("newfile.txt")

	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestCommonFileSystem_Open_Success(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	now := time.Now()
	info := &ObjectInfo{
		Name:         "file.txt",
		Size:         123,
		ContentType:  "text/plain",
		LastModified: now,
		IsDir:        false,
	}

	mockProvider.EXPECT().
		StatObject(gomock.Any(), "file.txt").
		Return(info, nil)

	reader := io.NopCloser(strings.NewReader("hello"))
	mockProvider.EXPECT().
		NewReader(gomock.Any(), "file.txt").
		Return(reader, nil)

	f, err := fs.Open("file.txt")
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestCommonFileSystem_Open_StatObjectError(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockProvider.EXPECT().
		StatObject(gomock.Any(), "missing.txt").
		Return(nil, errTest)

	_, err := fs.Open("missing.txt")
	require.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

func TestCommonFileSystem_Open_NewReaderError(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	info := &ObjectInfo{
		Name:         "file.txt",
		Size:         10,
		ContentType:  "text/plain",
		LastModified: time.Now(),
		IsDir:        false,
	}

	mockProvider.EXPECT().
		StatObject(gomock.Any(), "file.txt").
		Return(info, nil)

	mockProvider.EXPECT().
		NewReader(gomock.Any(), "file.txt").
		Return(nil, errTest)

	_, err := fs.Open("file.txt")

	require.Error(t, err)
	assert.Equal(t, errTest, err)
}

func TestCommonFileSystem_OpenFile_LocalRDWR_Success(t *testing.T) {
	ctrl, _, fs := setupCommonFS(t)
	defer ctrl.Finish()

	tmp, err := os.CreateTemp(t.TempDir(), "commonfs_test_*")

	require.NoError(t, err)

	_, err = tmp.WriteString("content")

	require.NoError(t, err)

	tmp.Close()
	defer os.Remove(tmp.Name())

	f, err := fs.OpenFile(tmp.Name(), os.O_RDWR, 0644)

	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestCommonFileSystem_OpenFile_Append_Success(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockWriter := NewMockWriteCloser(ctrl)

	mockProvider.EXPECT().
		NewWriter(gomock.Any(), "append.txt").
		Return(mockWriter)

	f, err := fs.OpenFile("append.txt", os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestCommonFileSystem_OpenFile_Create_Success(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockWriter := NewMockWriteCloser(ctrl)

	mockProvider.EXPECT().
		NewWriter(gomock.Any(), "create.txt").
		Return(mockWriter)

	f, err := fs.OpenFile("create.txt", os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestCommonFileSystem_OpenFile_UnsupportedFlags_Error(t *testing.T) {
	ctrl, _, fs := setupCommonFS(t)
	defer ctrl.Finish()

	_, err := fs.OpenFile("unsupported.txt", os.O_WRONLY, 0644)
	require.Error(t, err)
	assert.Equal(t, errUnsupportedFlags, err)
}

func TestCommonFileSystem_Remove_Success(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockProvider.EXPECT().
		DeleteObject(gomock.Any(), "file.txt").
		Return(nil)

	err := fs.Remove("file.txt")
	require.NoError(t, err)
}

func TestCommonFileSystem_Remove_DeleteError(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockProvider.EXPECT().
		DeleteObject(gomock.Any(), "file.txt").
		Return(errTest)

	err := fs.Remove("file.txt")
	require.Error(t, err)
	assert.Equal(t, errTest, err)
}

func TestCommonFileSystem_Rename_Success(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockProvider.EXPECT().
		CopyObject(gomock.Any(), "old.txt", "new.txt").
		Return(nil)

	mockProvider.EXPECT().
		DeleteObject(gomock.Any(), "old.txt").
		Return(nil)

	err := fs.Rename("old.txt", "new.txt")
	require.NoError(t, err)
}

func TestCommonFileSystem_Rename_CopyError(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockProvider.EXPECT().
		CopyObject(gomock.Any(), "old.txt", "new.txt").
		Return(errTest)

	err := fs.Rename("old.txt", "new.txt")
	require.Error(t, err)
	assert.Equal(t, errTest, err)
}

func TestCommonFileSystem_Rename_DeleteError(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	mockProvider.EXPECT().
		CopyObject(gomock.Any(), "old.txt", "new.txt").
		Return(nil)

	mockProvider.EXPECT().
		DeleteObject(gomock.Any(), "old.txt").
		Return(errTest)

	err := fs.Rename("old.txt", "new.txt")
	require.Error(t, err)
	assert.Equal(t, errTest, err)
}

func TestCommonFileSystem_MkdirAll_Success(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	dirPath := "a/b/c"

	mockWriter := NewMockWriteCloser(ctrl)

	// Each Mkdir call will:
	//   StatObject("x/") -> err (not exists)
	//   NewWriter("x/") -> mockWriter
	//   Write("") -> nil
	//   explicit Close() -> nil
	//   deferred Close() -> nil
	gomock.InOrder(
		mockProvider.EXPECT().StatObject(gomock.Any(), "a/").Return(nil, errTest),
		mockProvider.EXPECT().NewWriter(gomock.Any(), "a/").Return(mockWriter),
		mockWriter.EXPECT().Write([]byte("")).Return(0, nil),
		mockWriter.EXPECT().Close().Return(nil), // explicit close
		mockWriter.EXPECT().Close().Return(nil), // deferred close

		mockProvider.EXPECT().StatObject(gomock.Any(), "a/b/").Return(nil, errTest),
		mockProvider.EXPECT().NewWriter(gomock.Any(), "a/b/").Return(mockWriter),
		mockWriter.EXPECT().Write([]byte("")).Return(0, nil),
		mockWriter.EXPECT().Close().Return(nil),
		mockWriter.EXPECT().Close().Return(nil),

		mockProvider.EXPECT().StatObject(gomock.Any(), "a/b/c/").Return(nil, errTest),
		mockProvider.EXPECT().NewWriter(gomock.Any(), "a/b/c/").Return(mockWriter),
		mockWriter.EXPECT().Write([]byte("")).Return(0, nil),
		mockWriter.EXPECT().Close().Return(nil),
		mockWriter.EXPECT().Close().Return(nil),
	)

	err := fs.MkdirAll(dirPath, os.ModePerm)
	require.NoError(t, err)
}

func TestCommonFileSystem_MkdirAll_EmptyPath_Error(t *testing.T) {
	fs := &CommonFileSystem{}

	err := fs.MkdirAll("", os.ModePerm)

	require.Error(t, err)
	assert.Equal(t, errEmptyDirectoryName, err)
}

func TestCommonFileSystem_MkdirAll_Mkdir_WriteError(t *testing.T) {
	ctrl, mockProvider, fs := setupCommonFS(t)
	defer ctrl.Finish()

	dirPath := "x/y"

	mockWriter := NewMockWriteCloser(ctrl)

	// For first component "x/" simulate Write error
	gomock.InOrder(
		mockProvider.EXPECT().StatObject(gomock.Any(), "x/").Return(nil, errTest),
		mockProvider.EXPECT().NewWriter(gomock.Any(), "x/").Return(mockWriter),
		mockWriter.EXPECT().Write([]byte("")).Return(0, errTest),
		mockWriter.EXPECT().Close().Return(nil), // deferred close after return
	)

	err := fs.MkdirAll(dirPath, os.ModePerm)

	require.Error(t, err)
	assert.Equal(t, errTest, err)
}

// Tests for UseLogger, UseMetrics, Connect

func TestCommonFileSystem_UseLogger_SetsWhenCorrectType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	fs := &CommonFileSystem{}

	fs.UseLogger(mockLogger)

	assert.Equal(t, mockLogger, fs.Logger)
}

func TestCommonFileSystem_UseMetrics_SetsWhenCorrectType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	fs := &CommonFileSystem{}

	fs.UseMetrics(mockMetrics)

	assert.Equal(t, mockMetrics, fs.Metrics)
}

func TestCommonFileSystem_Connect_UsesLoggerWhenPresent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	// Expect Debugf to be called once. Use gomock.Any() for the format and args.
	mockLogger.EXPECT().Debug(gomock.Any()).Times(1)

	fs := &CommonFileSystem{
		Logger:   mockLogger,
		Location: "my-location",
	}

	// Should call Debugf on provided logger
	err := fs.Connect(t.Context())
	assert.Equal(t, err, errProviderNil)
}

func TestCommonFileSystem_Connect_NoLogger_NoPanic(t *testing.T) {
	// No logger configured: Connect should be a no-op and not panic
	fs := &CommonFileSystem{
		Logger:   nil,
		Location: "no-logger",
	}

	err := fs.Connect(t.Context())
	assert.Equal(t, err, errProviderNil)
}

// Test for PrettyPrint on OperationLog: verify output contains key fields.
func TestOperationLog_PrettyPrint(t *testing.T) {
	var buf bytes.Buffer

	status := "OK"
	message := "done"

	ol := &OperationLog{
		Operation: "TestOp",
		Provider:  "local",
		Duration:  123,
		Status:    &status,
		Message:   &message,
	}

	ol.PrettyPrint(&buf)
	out := buf.String()

	assert.Contains(t, out, "TestOp")
	assert.Contains(t, out, "local")
	// Duration printed as number (µs suffix present in format string)
	assert.Contains(t, out, "123")
	assert.Contains(t, out, "OK")
	assert.Contains(t, out, "done")
}

// Test DefaultHistogramBuckets returns expected bucket values.
func TestDefaultHistogramBuckets(t *testing.T) {
	exp := []float64{0.1, 1, 10, 100, 1000}
	got := DefaultHistogramBuckets()

	assert.Equal(t, exp, got)
}

func TestValidateSeekOffset_SeekStartValid(t *testing.T) {
	got, err := ValidateSeekOffset(io.SeekStart, 10, 0, 100)

	require.NoError(t, err)
	assert.Equal(t, int64(10), got)
}

func TestValidateSeekOffset_SeekEndNegativeOffset(t *testing.T) {
	got, err := ValidateSeekOffset(io.SeekEnd, -10, 0, 100)

	require.NoError(t, err)
	assert.Equal(t, int64(90), got)
}

func TestValidateSeekOffset_SeekCurrentValid(t *testing.T) {
	got, err := ValidateSeekOffset(io.SeekCurrent, 5, 20, 100)

	require.NoError(t, err)
	assert.Equal(t, int64(25), got)
}

func TestValidateSeekOffset_InvalidWhence_ReturnsErrOutOfRange(t *testing.T) {
	_, err := ValidateSeekOffset(999, 0, 0, 100)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutOfRange)
}

func TestValidateSeekOffset_NegativeResultingOffset_ReturnsErrOutOfRange(t *testing.T) {
	_, err := ValidateSeekOffset(io.SeekStart, -1, 0, 100)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutOfRange)
}

func TestValidateSeekOffset_OffsetGreaterThanLength_ReturnsErrOutOfRange(t *testing.T) {
	_, err := ValidateSeekOffset(io.SeekStart, 101, 0, 100)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutOfRange)
}

func TestValidateSeekOffset_SeekCurrentBeyondLength_ReturnsErrOutOfRange(t *testing.T) {
	_, err := ValidateSeekOffset(io.SeekCurrent, 1000, 10, 100)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutOfRange)
}
