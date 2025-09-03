package gcs

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

var (
	errObjectNotFound = errors.New("object not found")
	errMock           = fmt.Errorf("errMock")
)

func Test_CreateFile(t *testing.T) {
	type testCase struct {
		name        string
		createPath  string
		setupMocks  func(mockGCS *MockgcsClient)
		expectError bool
		isRoot      bool
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: "fake-creds",
		ProjectID:       "test-project",
	}

	fs := &FileSystem{
		conn:    mockGCS,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	tests := []testCase{
		{
			name:       "create file at root level",
			createPath: "abc.txt",
			setupMocks: func(m *MockgcsClient) {
				m.EXPECT().ListObjects(gomock.Any(), ".").Return([]string{}, nil)
				m.EXPECT().ListObjects(gomock.Any(), "abc.txt").Return([]string{}, nil)
				m.EXPECT().NewWriter(gomock.Any(), "abc.txt").Return(&storage.Writer{})
			},

			expectError: false,
			isRoot:      true,
		},
		{
			name:       "fail when parent directory does not exist",
			createPath: "abc/abc.txt",
			setupMocks: func(m *MockgcsClient) {
				m.EXPECT().ListObjects(gomock.Any(), "abc/").Return(nil, errMock)
			},
			expectError: true,
			isRoot:      false,
		},
		{
			name:       "create file inside existing directory",
			createPath: "abc/efg.txt",
			setupMocks: func(m *MockgcsClient) {
				// parent path "abc/" exists
				m.EXPECT().ListObjects(gomock.Any(), "abc/").Return([]string{"abc/.keep"}, nil)
				// filename does not exist
				m.EXPECT().ListObjects(gomock.Any(), "abc/efg.txt").Return([]string{}, nil)
				m.EXPECT().NewWriter(gomock.Any(), "abc/efg.txt").Return(&storage.Writer{})
			},
			expectError: false,
			isRoot:      false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks(mockGCS)

			file, err := fs.Create(tt.createPath)

			if tt.expectError {
				require.Error(t, err, "Test %d (%s): expected an error", i, tt.name)
				return
			}

			require.NoError(t, err, "Test %d (%s): expected no error", i, tt.name)
			require.NotNil(t, file)
		})
	}
}
func Test_Remove_GCS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn:    mockGCS,
		logger:  mockLogger,
		config:  &Config{BucketName: "test-bucket"},
		metrics: mockMetrics,
	}

	// Expectations
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	mockGCS.EXPECT().
		DeleteObject(gomock.Any(), "abc/a1.txt").
		Return(nil).
		Times(1)

	err := fs.Remove("abc/a1.txt")
	require.NoError(t, err)
}

var (
	errDeleteFailed = errors.New("delete failed")
	errCopyFailed   = errors.New("copy failed")
)

func TestRenameFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	tests := []struct {
		name          string
		initialName   string
		newName       string
		setupMocks    func()
		expectedError bool
	}{
		{
			name:        "Rename file to new name",
			initialName: "dir/file.txt",
			newName:     "dir/file-renamed.txt",
			setupMocks: func() {
				mockConn.EXPECT().CopyObject(gomock.Any(), "dir/file.txt", "dir/file-renamed.txt").Return(nil)
				mockConn.EXPECT().DeleteObject(gomock.Any(), "dir/file.txt").Return(nil)
			},
			expectedError: false,
		},
		{
			name:        "Rename file with copy failure",
			initialName: "dir/file.txt",
			newName:     "dir/file-renamed.txt",
			setupMocks: func() {
				mockConn.EXPECT().CopyObject(gomock.Any(), "dir/file.txt", "dir/file-renamed.txt").Return(errCopyFailed)
			},
			expectedError: true,
		},
		{
			name:        "Rename file with delete failure",
			initialName: "dir/file.txt",
			newName:     "dir/file-renamed.txt",
			setupMocks: func() {
				mockConn.EXPECT().CopyObject(gomock.Any(), "dir/file.txt", "dir/file-renamed.txt").Return(nil)
				mockConn.EXPECT().DeleteObject(gomock.Any(), "dir/file.txt").Return(errDeleteFailed)
			},
			expectedError: true,
		},
		{
			name:          "Rename file to same name",
			initialName:   "dir/file.txt",
			newName:       "dir/file.txt",
			setupMocks:    func() {}, // No calls expected
			expectedError: false,
		},
		{
			name:          "Rename file to different directory (not allowed)",
			initialName:   "dir1/file.txt",
			newName:       "dir2/file.txt",
			setupMocks:    func() {}, // No calls expected
			expectedError: true,
		},
	}

	// Set up logger mocks globally
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			err := fs.Rename(tt.initialName, tt.newName)

			if tt.expectedError {
				require.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err, "Unexpected error: %v", err)
			}
		})
	}
}

func Test_StatFile_GCS(t *testing.T) {
	tm := time.Now()

	type result struct {
		name  string
		size  int64
		isDir bool
	}

	tests := []struct {
		name        string
		filePath    string
		mockAttr    *storage.ObjectAttrs
		mockError   error
		expected    result
		expectError bool
	}{
		{
			name:     "Valid file stat",
			filePath: "abc/efg/file.txt",
			mockAttr: &storage.ObjectAttrs{
				Name:        "abc/efg/file.txt",
				Size:        123,
				Updated:     tm,
				ContentType: "text/plain",
			},
			expected: result{
				name:  "abc/efg/file.txt",
				size:  123,
				isDir: false,
			},
		},
		{
			name:        "File not found",
			filePath:    "notfound.txt",
			mockAttr:    nil,
			mockError:   errObjectNotFound,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGCS := NewMockgcsClient(ctrl)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			config := &Config{BucketName: "test-bucket"}

			fs := &FileSystem{
				conn:    mockGCS,
				config:  config,
				logger:  mockLogger,
				metrics: mockMetrics,
			}

			mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

			mockGCS.EXPECT().StatObject(gomock.Any(), tt.filePath).Return(tt.mockAttr, tt.mockError)
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
				"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

			res, err := fs.Stat(tt.filePath)
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			actual := result{
				name:  res.Name(),
				size:  res.Size(),
				isDir: res.IsDir(),
			}

			assert.Equal(t, tt.expected, actual)
		})
	}
}
func Test_Stat_FileAndDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn:    mockGCS,
		logger:  mockLogger,
		metrics: mockMetrics,
		config: &Config{
			BucketName: "test-bucket",
		},
	}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	fileName := "documents/testfile.txt"
	fileAttrs := &storage.ObjectAttrs{
		Name:        fileName,
		Size:        1024,
		ContentType: "text/plain",
		Updated:     time.Now(),
	}
	mockGCS.EXPECT().StatObject(gomock.Any(), fileName).Return(fileAttrs, nil)

	info, err := fs.Stat(fileName)
	assert.NilError(t, err)
	assert.Equal(t, fileName, info.Name())
	assert.Equal(t, int64(1024), info.Size())
	assert.Check(t, !info.IsDir())

	dirName := "documents/folder/"
	dirAttrs := &storage.ObjectAttrs{
		Name:        dirName,
		Size:        0,
		ContentType: "application/x-directory",
		Updated:     time.Now(),
	}

	mockGCS.EXPECT().StatObject(gomock.Any(), dirName).Return(dirAttrs, nil)

	info, err = fs.Stat(dirName)

	assert.NilError(t, err)
	assert.Equal(t, dirName, info.Name())
	assert.Equal(t, int64(0), info.Size())
	assert.Check(t, info.IsDir())
}
