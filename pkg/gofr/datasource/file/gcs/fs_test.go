package gcs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
)

func TestNew_NilConfig(t *testing.T) {
	fs := New(nil)

	require.NotNil(t, fs)
	assert.Empty(t, fs.(*fileSystem).CommonFileSystem.Location)
}

func TestNew_EmptyBucketName(t *testing.T) {
	config := &Config{BucketName: ""}

	fs := New(config)

	require.NotNil(t, fs)
	assert.Empty(t, fs.(*fileSystem).CommonFileSystem.Location)
}

func TestNew_ConnectionFailure_StartsRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		BucketName:      "non-existent-bucket",
		CredentialsJSON: `{"type":"service_account","project_id":"test"}`,
	}

	fs := New(config)
	require.NotNil(t, fs)

	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	// Expect warning about background retry (with error in format string)
	mockLogger.EXPECT().Errorf(
		"GCS bucket %s not available, starting background retry: %v",
		"non-existent-bucket",
		gomock.Any(), // Error message varies
	)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	// Now connect
	fs.Connect()

	time.Sleep(100 * time.Millisecond)

	fs.(*fileSystem).CommonFileSystem.SetDisableRetry(true)
}

func TestNew_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		BucketName: "test-bucket",
		EndPoint:   "http://localhost:4443",
	}

	fs := New(config)
	require.NotNil(t, fs)

	// Inject logger and metrics (mimicking AddFileStore behavior)
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("GCS connection established to bucket %s", "test-bucket").MaxTimes(1)
	mockLogger.EXPECT().Errorf(
		"GCS bucket %s not available, starting background retry: %v",
		"test-bucket",
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	// Now connect
	fs.Connect()

	// If connected, verify state
	if fs.(*fileSystem).CommonFileSystem.IsConnected() {
		t.Log("Successfully connected to GCS emulator")
	} else {
		t.Log("GCS emulator not available, retry started")

		fs.(*fileSystem).CommonFileSystem.SetDisableRetry(true)
	}
}

func TestGCSFileSystem_Observe_ProviderName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Location:     "test-bucket",
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			ProviderName: "GCS",
		},
	}

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		file.AppFileStats,
		gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", "GCS",
	)

	mockLogger.EXPECT().Debug(gomock.Any()).Do(func(log any) {
		opLog, ok := log.(*file.OperationLog)
		require.True(t, ok)
		assert.Equal(t, "GCS", opLog.Provider)
	})

	operation := file.OpConnect
	startTime := time.Now()
	status := "SUCCESS"
	message := "test message"

	fs.Observe(operation, startTime, &status, &message)
}

func TestConnect_AlreadyConnected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket", EndPoint: "http://localhost:4443"}

	adapter := &storageAdapter{cfg: config}

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider: adapter,
			Location: config.BucketName,
			Logger:   mockLogger,
			Metrics:  mockMetrics,
		},
	}

	// Manually mark as connected
	fs.CommonFileSystem.SetConnected(true)

	// Should not call any logger methods (fast-path)
	fs.Connect()

	assert.True(t, fs.CommonFileSystem.IsConnected())
}

func TestStartRetryConnect_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket", EndPoint: "http://localhost:4443"}

	adapter := &storageAdapter{cfg: config}

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider: adapter,
			Location: config.BucketName,
			Logger:   mockLogger,
			Metrics:  mockMetrics,
		},
	}

	// Expect histogram registration
	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	// Expect debug logs for observe
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	// If connection succeeds, expect success log
	mockLogger.EXPECT().Infof("connected to %s", config.BucketName).MaxTimes(1)
	mockLogger.EXPECT().Infof("GCS connection restored to bucket %s", config.BucketName).MaxTimes(1)

	// If connection fails, expect debug retry log
	mockLogger.EXPECT().Debugf("GCS retry attempt failed, will try again in 30s").AnyTimes()

	// Start retry with short interval
	done := make(chan bool)

	go func() {
		time.Sleep(2 * time.Second)
		fs.CommonFileSystem.SetDisableRetry(true)

		done <- true
	}()

	go fs.startRetryConnect()

	<-done
}

func TestFileSystem_Connect_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider: mockProvider,
			Location: "test-bucket",
			Logger:   mockLogger,
			Metrics:  mockMetrics,
		},
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())
	mockProvider.EXPECT().Connect(gomock.Any()).Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Infof("connected to %s", "test-bucket")
	mockLogger.EXPECT().Infof("GCS connection established to bucket %s", "test-bucket")

	fs.Connect()

	assert.True(t, fs.CommonFileSystem.IsConnected())
}

func TestStartRetryConnect_ExitsImmediately(t *testing.T) {
	tests := []struct {
		desc      string
		connected bool
		disabled  bool
	}{
		{desc: "already connected", connected: true},
		{desc: "retry disabled", disabled: true},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// A gomock provider with no expectations fails the test if a connect is attempted.
			fs := &fileSystem{
				CommonFileSystem: &file.CommonFileSystem{
					Provider: file.NewMockStorageProvider(gomock.NewController(t)),
					Location: "test-bucket",
				},
			}

			fs.CommonFileSystem.SetConnected(tc.connected)
			fs.CommonFileSystem.SetDisableRetry(tc.disabled)

			done := make(chan struct{})

			go func() {
				fs.startRetryConnect()
				close(done)
			}()

			// Without the early return, startRetryConnect blocks on its one-minute ticker.
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("startRetryConnect did not return immediately")
			}

			assert.Equal(t, tc.connected, fs.CommonFileSystem.IsConnected())
		})
	}
}
