package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/option"
	"gotest.tools/v3/assert"
)

var (
	errObjectNotFound        = errors.New("object not found")
	errMock                  = fmt.Errorf("errMock")
	errorStat                = errors.New("stat error")
	errSimulatedFirstFailure = errors.New("simulated first failure")
)

func TestNew_FileSystemProvider(t *testing.T) {
	config := &Config{
		BucketName:      "test-bucket",
		EndPoint:        "http://localhost:4566",
		CredentialsJSON: `{"type":"service_account"}`,
		ProjectID:       "test-project",
	}

	provider := New(config)

	fs, ok := provider.(*FileSystem)
	require.True(t, ok, "New() should return *FileSystem")
	require.NotNil(t, fs, "returned FileSystem should not be nil")

	require.Equal(t, config, fs.config)
	require.Nil(t, fs.conn)
}

func TestFileSystem_Connect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "emulator mode with endpoint",
			config: &Config{
				EndPoint:   "http://localhost:9000",
				BucketName: "test-bucket",
			},
		},
		{
			name: "credentials JSON mode",
			config: &Config{
				CredentialsJSON: `{"type":"service_account"}`,
				BucketName:      "test-bucket",
			},
		},
		{
			name: "default mode",
			config: &Config{
				BucketName: "test-bucket",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subCtrl := gomock.NewController(t)
			defer subCtrl.Finish()

			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			mockMetrics.EXPECT().NewHistogram(
				file.AppFileStats,
				"App FTP Stats - duration of file operations",
				file.DefaultHistogramBuckets(),
			).Times(1)

			mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

			mockMetrics.EXPECT().RecordHistogram(
				gomock.Any(), file.AppFileStats, gomock.Any(),
				"type", gomock.Any(),
				"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

			fs := &FileSystem{
				config:       tt.config,
				logger:       mockLogger,
				metrics:      mockMetrics,
				disableRetry: true,
			}

			fs.Connect()
		})
	}
}
func TestFileSystem_startRetryConnect_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		BucketName: "retry-bucket",
		CredentialsJSON: `{
			"type": "service_account",
			"client_email": "test@example.com",
			"private_key": "-----BEGIN PRIVATE KEY-----\nMIIBOQIBAAJBAK...\n-----END PRIVATE KEY-----\n"
		}`,
	}

	fs := &FileSystem{
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	var callCount int

	mockLogger.EXPECT().Errorf("Retry: failed to connect to GCS: %v", gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof("GCS connection restored to bucket %s", "retry-bucket").Times(1)

	fs.conn = nil

	done := make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		//nolint:staticcheck // for-select loop is required for periodic retry logic with ticker; range cannot be used here
		for {
			select {
			case <-ticker.C:
				ctx := context.TODO()

				var (
					client *storage.Client
					err    error
				)

				callCount++
				if callCount == 1 {
					err = errSimulatedFirstFailure
				} else {
					client, err = storage.NewClient(ctx, option.WithCredentialsJSON([]byte(fs.config.CredentialsJSON)))
					if err == nil {
						fs.conn = &gcsClientImpl{
							client: client,
							bucket: client.Bucket(fs.config.BucketName),
						}
						fs.logger.Infof("GCS connection restored to bucket %s", fs.config.BucketName)

						done <- true

						return
					}
				}

				if err != nil {
					fs.logger.Errorf("Retry: failed to connect to GCS: %v", err)
				}
			}
		}
	}()

	select {
	case <-done:
		require.NotNil(t, fs.conn, "connection should be restored")
	case <-time.After(2 * time.Second):
		t.Fatal("retry did not succeed within timeout")
	}
}

func TestFileSystem_Open(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn:    mockGCS,
		config:  &Config{BucketName: "test-bucket"},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	t.Run("file found", func(t *testing.T) {
		attr := &storage.ObjectAttrs{
			Name:        "test.txt",
			Size:        100,
			Updated:     time.Now(),
			ContentType: "text/plain",
		}

		mockGCS.EXPECT().NewReader(gomock.Any(), "test.txt").Return(io.NopCloser(strings.NewReader("data")), nil)
		mockGCS.EXPECT().StatObject(gomock.Any(), "test.txt").Return(attr, nil)

		fileInfo, err := fs.Open("test.txt")

		require.NoError(t, err)
		require.NotNil(t, fileInfo)
	})

	t.Run("file not found", func(t *testing.T) {
		mockGCS.EXPECT().NewReader(gomock.Any(), "missing.txt").Return(nil, storage.ErrObjectNotExist)

		_, err := fs.Open("missing.txt")
		require.Error(t, err)
		require.Equal(t, file.ErrFileNotFound, err)
	})

	t.Run("error reading file", func(t *testing.T) {
		mockGCS.EXPECT().NewReader(gomock.Any(), "error.txt").Return(nil, errorRead)

		_, err := fs.Open("error.txt")
		require.Error(t, err)
	})

	t.Run("error on StatObject", func(t *testing.T) {
		mockGCS.EXPECT().NewReader(gomock.Any(), "statfail.txt").Return(io.NopCloser(strings.NewReader("data")), nil)
		mockGCS.EXPECT().StatObject(gomock.Any(), "statfail.txt").Return(nil, errorStat)

		_, err := fs.Open("statfail.txt")
		require.Error(t, err)
	})
}

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

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

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

			fileData, err := fs.Create(tt.createPath)

			if tt.expectError {
				require.Error(t, err, "Test %d (%s): expected an error", i, tt.name)
				return
			}

			require.NoError(t, err, "Test %d (%s): expected no error", i, tt.name)
			require.NotNil(t, fileData)
		})
	}
}

func TestGenerateCopyName(t *testing.T) {
	tests := []struct {
		original string
		count    int
		expected string
	}{
		{"file.txt", 1, "file copy 1.txt"},
		{"docs/report.pdf", 2, "docs/report copy 2.pdf"},
		{"image.png", 3, "image copy 3.png"},
		{"noext", 1, "noext copy 1"},
		{"dir/file.txt", 1, "dir/file copy 1.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.original, func(t *testing.T) {
			result := generateCopyName(tt.original, tt.count)
			require.Equal(t, tt.expected, result)
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
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

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
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

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

			mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

			mockGCS.EXPECT().StatObject(gomock.Any(), tt.filePath).Return(tt.mockAttr, tt.mockError)
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
				"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

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

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

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

func Test_FileSystem_UseLogger_UseMetrics(t *testing.T) {
	fs := &FileSystem{}

	logger := NewMockLogger(gomock.NewController(t))
	metrics := NewMockMetrics(gomock.NewController(t))

	require.Nil(t, fs.logger)
	require.Nil(t, fs.metrics)

	fs.UseLogger(logger)
	require.Equal(t, logger, fs.logger)

	fs.UseMetrics(metrics)
	require.Equal(t, metrics, fs.metrics)
}
