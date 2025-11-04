package gcs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/option"
)

var (
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
		t.Run(tt.name, func(*testing.T) {
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			mockMetrics.EXPECT().NewHistogram(
				file.AppFileStats,
				"App GCS Stats - duration of file operations",
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
						fs.conn = &storageAdapter{
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

func Test_Remove_GCS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockStorageProvider(ctrl)
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

	mockConn := NewMockStorageProvider(ctrl)
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

func TestFileSystem_Create_ParentDirectoryNotExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	expectedErr := errObjectNotFound

	mockConn.EXPECT().
		ListObjects(gomock.Any(), "parent/").
		Return(nil, expectedErr)

	mockLogger.EXPECT().Errorf("Failed to list parent directory %q: %v", "parent/", expectedErr)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	_, err := fs.Create("parent/file.txt")

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

func TestFileSystem_Create_ListObjectsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	expectedErr := errRead

	// Parent directory check succeeds
	mockConn.EXPECT().
		ListObjects(gomock.Any(), ".").
		Return([]string{}, nil)

	// But checking for existing file fails
	mockConn.EXPECT().
		ListObjects(gomock.Any(), "file.txt").
		Return(nil, expectedErr)

	mockLogger.EXPECT().Errorf("Failed to list objects for name %q: %v", "file.txt", expectedErr)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	_, err := fs.Create("file.txt")

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

func TestFileSystem_Create_FileAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockConn.EXPECT().
		ListObjects(gomock.Any(), ".").
		Return([]string{}, nil)

	// Original name exists
	mockConn.EXPECT().
		ListObjects(gomock.Any(), "file.txt").
		Return([]string{"file.txt"}, nil)

	expectedCopyName := "file copy 1.txt"
	mockConn.EXPECT().
		ListObjects(gomock.Any(), expectedCopyName).
		Return([]string{}, nil)

	mockWriter := &storage.Writer{}
	mockConn.EXPECT().
		NewWriter(gomock.Any(), expectedCopyName).
		Return(mockWriter)

	mockLogger.EXPECT().Infof("Write stream successfully opened for file %q", expectedCopyName)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f, err := fs.Create("file.txt")

	require.NoError(t, err)
	require.NotNil(t, f)
	require.Equal(t, expectedCopyName, f.(*File).name)
}

func TestFileSystem_Create_WriterTypeAssertionFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockConn.EXPECT().
		ListObjects(gomock.Any(), ".").
		Return([]string{}, nil)

	mockConn.EXPECT().
		ListObjects(gomock.Any(), "file.txt").
		Return([]string{}, nil)

	// Return a writer that's not *storage.Writer
	mockConn.EXPECT().
		NewWriter(gomock.Any(), "file.txt").
		Return(&mockWriteCloser{})

	mockLogger.EXPECT().Errorf("Type assertion failed for writer to *storage.Writer")
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	_, err := fs.Create("file.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, errWriterTypeAssertion)
}

func TestFileSystem_Open_FileNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockConn.EXPECT().
		NewReader(gomock.Any(), "notfound.txt").
		Return(nil, storage.ErrObjectNotExist)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	_, err := fs.Open("notfound.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, file.ErrFileNotFound)
}

func TestFileSystem_Open_ReaderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	expectedErr := errRead

	mockConn.EXPECT().
		NewReader(gomock.Any(), "file.txt").
		Return(nil, expectedErr)

	mockLogger.EXPECT().Errorf("failed to retrieve %q: %v", "file.txt", expectedErr)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	_, err := fs.Open("file.txt")

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

func TestFileSystem_Open_StatObjectError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	expectedErr := errStat
	mockReader := &fakeStorageReader{Reader: strings.NewReader("test")}

	mockConn.EXPECT().
		NewReader(gomock.Any(), "file.txt").
		Return(mockReader, nil)

	mockConn.EXPECT().
		StatObject(gomock.Any(), "file.txt").
		Return(nil, expectedErr)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	_, err := fs.Open("file.txt")

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
	require.True(t, mockReader.closed, "Reader should be closed on error")
}
func TestFileSystem_OpenFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockReader := &fakeStorageReader{Reader: strings.NewReader("test")}
	mockConn.EXPECT().
		NewReader(gomock.Any(), "file.txt").
		Return(mockReader, nil)

	mockConn.EXPECT().
		StatObject(gomock.Any(), "file.txt").
		Return(&file.ObjectInfo{
			Size:         4,
			ContentType:  "text/plain",
			LastModified: time.Now(),
		}, nil)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f, err := fs.OpenFile("file.txt", 0, 0644)

	require.NoError(t, err)
	require.NotNil(t, f)
}

func TestFileSystem_UseLogger_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)

	fs := &FileSystem{}

	fs.UseLogger(mockLogger)

	require.Equal(t, mockLogger, fs.logger)
}

func TestFileSystem_UseLogger_InvalidType(t *testing.T) {
	fs := &FileSystem{}

	// Pass a non-Logger type
	fs.UseLogger("not a logger")

	require.Nil(t, fs.logger)
}

func TestFileSystem_UseMetrics_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{}

	fs.UseMetrics(mockMetrics)

	require.Equal(t, mockMetrics, fs.metrics)
}

func TestFileSystem_UseMetrics_InvalidType(t *testing.T) {
	fs := &FileSystem{}

	// Pass a non-Metrics type
	fs.UseMetrics("not metrics")

	require.Nil(t, fs.metrics)
}

// Helper types

type mockWriteCloser struct {
	save func([]byte)
}

func (m *mockWriteCloser) Write(p []byte) (int, error) {
	if m.save != nil {
		m.save(p)
	}

	return len(p), nil
}

func (*mockWriteCloser) Close() error {
	return nil
}

type fakeStorageReader struct {
	*strings.Reader
	closed bool
}

func (f *fakeStorageReader) Close() error {
	f.closed = true
	return nil
}
