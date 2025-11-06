package gcs

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
)

// mockGCSFileSystem creates a test helper that creates a mock file system without real connection.
func mockGCSFileSystem(config *Config) *fileSystem {
	return &fileSystem{
		config: config,
	}
}

func TestNew_NilConfig(t *testing.T) {
	fs, err := New(nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errInvalidConfig)
}

func TestNew_EmptyBucketName(t *testing.T) {
	config := &Config{
		BucketName: "",
	}

	fs, err := New(config)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errInvalidConfig)
}

func TestNew_ConnectionFailure(t *testing.T) {
	config := &Config{
		BucketName:      "non-existent-bucket",
		CredentialsJSON: `{"type":"service_account","project_id":"test"}`,
	}

	fs, err := New(config)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errConnectionFailed)
}

func TestTryConnect_WithNilLogger(t *testing.T) {
	config := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: `{"type":"service_account","project_id":"test"}`,
	}

	fs := mockGCSFileSystem(config)

	err := fs.tryConnect(context.Background())

	require.Error(t, err)
	assert.Nil(t, fs.client)
}

func TestTryConnect_WithLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)

	config := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: `{"type":"service_account","project_id":"test"}`,
	}

	fs := mockGCSFileSystem(config)
	fs.logger = mockLogger

	mockLogger.EXPECT().Debugf("connecting to GCS bucket: %s", "test-bucket")

	err := fs.tryConnect(context.Background())

	require.Error(t, err)
}

func TestTryConnect_InvalidCredentials(t *testing.T) {
	config := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: `invalid-json`,
	}

	fs := mockGCSFileSystem(config)

	err := fs.tryConnect(context.Background())

	require.Error(t, err)
}

func TestConnect_AlreadyConnectedFastPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		BucketName: "test-bucket",
	}

	fs := mockGCSFileSystem(config)
	fs.logger = mockLogger
	fs.metrics = mockMetrics
	fs.client = &storage.Client{}
	fs.bucket = &storage.BucketHandle{}

	mockMetrics.EXPECT().NewHistogram(
		file.AppFileStats,
		"App GCS Stats - duration of file operations",
		gomock.Any(),
	)
	mockLogger.EXPECT().Infof("connected to GCS bucket %s", "test-bucket")
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs.Connect()

	assert.True(t, fs.connected)
	assert.NotNil(t, fs.CommonFileSystem)
}

func TestConnect_ReconnectOnNilClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: `{"type":"service_account","project_id":"test"}`,
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.logger = mockLogger
	gcsFS.metrics = mockMetrics

	mockMetrics.EXPECT().NewHistogram(
		file.AppFileStats,
		"App GCS Stats - duration of file operations",
		gomock.Any(),
	)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debugf("attempting to reconnect to GCS bucket: %s", "test-bucket")
	mockLogger.EXPECT().Debugf("connecting to GCS bucket: %s", "test-bucket")
	mockLogger.EXPECT().Errorf("Failed to connect to GCS: %v", gomock.Any())

	gcsFS.Connect()

	assert.False(t, gcsFS.connected)
}

func TestConnect_ReconnectFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		BucketName:      "non-existent-bucket",
		CredentialsJSON: `invalid-json`,
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.logger = mockLogger
	gcsFS.metrics = mockMetrics

	mockMetrics.EXPECT().NewHistogram(
		file.AppFileStats,
		"App GCS Stats - duration of file operations",
		gomock.Any(),
	)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debugf("attempting to reconnect to GCS bucket: %s", "non-existent-bucket")
	mockLogger.EXPECT().Debugf("connecting to GCS bucket: %s", "non-existent-bucket")
	mockLogger.EXPECT().Errorf("Failed to connect to GCS: %v", gomock.Any())

	gcsFS.Connect()

	assert.False(t, gcsFS.connected)
}

func TestStartRetryConnect_RetryFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		BucketName:      "non-existent-bucket",
		CredentialsJSON: `invalid`,
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.logger = mockLogger
	gcsFS.metrics = mockMetrics

	mockLogger.EXPECT().Debugf("connecting to GCS bucket: %s", "non-existent-bucket").AnyTimes()
	mockLogger.EXPECT().Errorf("Retry: failed to connect to GCS: %v", gomock.Any()).AnyTimes()

	done := make(chan bool)

	go func() {
		time.Sleep(1500 * time.Millisecond)

		gcsFS.disableRetry = true

		done <- true
	}()

	go gcsFS.startRetryConnect()

	<-done
	assert.False(t, gcsFS.connected)
}

func TestUseLogger_ValidLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)

	config := &Config{
		BucketName: "test-bucket",
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.UseLogger(mockLogger)

	assert.Equal(t, mockLogger, gcsFS.logger)
}

func TestUseLogger_InvalidType(t *testing.T) {
	config := &Config{
		BucketName: "test-bucket",
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.UseLogger("not-a-logger")

	assert.Nil(t, gcsFS.logger)
}

func TestUseLogger_NilLogger(t *testing.T) {
	config := &Config{
		BucketName: "test-bucket",
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.UseLogger(nil)

	assert.Nil(t, gcsFS.logger)
}

func TestUseMetrics_ValidMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		BucketName: "test-bucket",
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.UseMetrics(mockMetrics)

	assert.NotNil(t, gcsFS.metrics)
}

func TestUseMetrics_InvalidType(t *testing.T) {
	config := &Config{
		BucketName: "test-bucket",
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.UseMetrics("not-metrics")

	assert.Nil(t, gcsFS.metrics)
}

func TestUseMetrics_NilMetrics(t *testing.T) {
	config := &Config{
		BucketName: "test-bucket",
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.UseMetrics(nil)

	assert.Nil(t, gcsFS.metrics)
}

func TestObserve_WithNilLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		BucketName: "test-bucket",
	}

	gcsFS := mockGCSFileSystem(config)
	gcsFS.metrics = mockMetrics

	status := file.StatusSuccess
	message := "test message"
	startTime := time.Now()

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	gcsFS.observe(file.OpConnect, startTime, &status, &message)

	assert.NotNil(t, mockMetrics)
}

func TestConfig_AllFields(t *testing.T) {
	config := &Config{
		EndPoint:        "http://localhost:4443",
		BucketName:      "test-bucket",
		CredentialsJSON: `{"type":"service_account"}`,
		ProjectID:       "test-project",
	}

	assert.Equal(t, "http://localhost:4443", config.EndPoint)
	assert.Equal(t, "test-bucket", config.BucketName)
	assert.Contains(t, config.CredentialsJSON, "service_account")
	assert.Equal(t, "test-project", config.ProjectID)
}

func TestConfig_OnlyBucketName(t *testing.T) {
	config := &Config{
		BucketName: "minimal-bucket",
	}

	assert.Equal(t, "minimal-bucket", config.BucketName)
	assert.Empty(t, config.EndPoint)
	assert.Empty(t, config.CredentialsJSON)
	assert.Empty(t, config.ProjectID)
}

func TestFileSystem_InitialState(t *testing.T) {
	config := &Config{
		BucketName: "test-bucket",
	}

	fs := mockGCSFileSystem(config)

	assert.NotNil(t, fs.config)
	assert.Nil(t, fs.client)
	assert.Nil(t, fs.bucket)
	assert.Nil(t, fs.logger)
	assert.Nil(t, fs.metrics)
	assert.False(t, fs.connected)
	assert.False(t, fs.disableRetry)
}
