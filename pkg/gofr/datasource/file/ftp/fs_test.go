package ftp

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

func TestNew_EmptyHost(t *testing.T) {
	config := &Config{Host: "", Port: 2121}

	fs, err := New(config, nil, nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errInvalidConfig)
}

func TestNew_InvalidPort_Negative(t *testing.T) {
	config := &Config{Host: "localhost", Port: -1}

	fs, err := New(config, nil, nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errInvalidConfig)
}

func TestNew_PortZero(t *testing.T) {
	config := &Config{Host: "localhost", Port: 0}

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
		Host:     "non-existent-host",
		Port:     9999,
		User:     "testuser",
		Password: "testpass",
	}

	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

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
		Host:     "localhost",
		Port:     2121,
		User:     "testuser",
		Password: "testpass",
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "localhost:2121").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)

	assert.Equal(t, "localhost:2121", fs.(*fileSystem).CommonFileSystem.Location)
}

func TestNew_WithRemoteDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		Host:      "localhost",
		Port:      2121,
		User:      "testuser",
		Password:  "testpass",
		RemoteDir: "/uploads",
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "localhost:2121/uploads").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)

	assert.Equal(t, "localhost:2121/uploads", fs.(*fileSystem).CommonFileSystem.Location)
}

func TestNew_WithRootRemoteDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		Host:      "localhost",
		Port:      2121,
		User:      "testuser",
		Password:  "testpass",
		RemoteDir: "/",
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "localhost:2121").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)

	assert.Equal(t, "localhost:2121", fs.(*fileSystem).CommonFileSystem.Location)
}

func TestNew_WithCustomDialTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		Host:        "localhost",
		Port:        2121,
		User:        "testuser",
		Password:    "testpass",
		DialTimeout: 3 * time.Second,
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "localhost:2121").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)
}

func TestConnect_AlreadyConnected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		Host:     "localhost",
		Port:     2121,
		User:     "testuser",
		Password: "testpass",
	}

	adapter := &storageAdapter{cfg: config}

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider: adapter,
			Location: "localhost:2121",
			Logger:   mockLogger,
			Metrics:  mockMetrics,
		},
	}

	fs.CommonFileSystem.SetConnected(true)

	fs.Connect()

	assert.True(t, fs.CommonFileSystem.IsConnected())
}

func TestStartRetryConnect_ExitWhenRetryDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		Host:     "localhost",
		Port:     2121,
		User:     "testuser",
		Password: "testpass",
	}

	adapter := &storageAdapter{cfg: config}

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider: adapter,
			Location: "localhost:2121",
			Logger:   mockLogger,
			Metrics:  mockMetrics,
		},
	}

	// Disable retry before starting
	fs.CommonFileSystem.SetDisableRetry(true)

	// Should exit immediately without any expectations
	done := make(chan bool)

	go func() {
		fs.startRetryConnect()

		done <- true
	}()

	select {
	case <-done:
		// Success: function exited immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("startRetryConnect did not exit when retry is disabled")
	}
}
