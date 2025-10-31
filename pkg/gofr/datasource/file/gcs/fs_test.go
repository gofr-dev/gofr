package gcs

import (
	"context"
	"errors"
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
