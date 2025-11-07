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
	fs, err := New(nil, nil, nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errInvalidConfig)
}

func TestNew_EmptyBucketName(t *testing.T) {
	config := &Config{BucketName: ""}

	fs, err := New(config, nil, nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errInvalidConfig)
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

	// Expect warning about background retry (with error in format string)
	mockLogger.EXPECT().Warnf(
		"GCS bucket %s not available, starting background retry: %v",
		"non-existent-bucket",
		gomock.Any(), // Error message varies
	)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)

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

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "test-bucket").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"GCS bucket %s not available, starting background retry: %v",
		"test-bucket",
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)

	// If connected, verify state
	if fs.(*fileSystem).CommonFileSystem.IsConnected() {
		t.Log("Successfully connected to GCS emulator")
	} else {
		t.Log("GCS emulator not available, retry started")

		fs.(*fileSystem).CommonFileSystem.SetDisableRetry(true)
	}
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
