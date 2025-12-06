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
	fs := New(nil)

	require.NotNil(t, fs)
	assert.Equal(t, "ftp://unconfigured", fs.(*fileSystem).CommonFileSystem.Location)
}

func TestNew_EmptyHost(t *testing.T) {
	config := &Config{Host: "", Port: 2121}

	fs := New(config)

	require.NotNil(t, fs)

	assert.Equal(t, "ftp://unconfigured", fs.(*fileSystem).CommonFileSystem.Location)
}

func TestNew_PortZero(t *testing.T) {
	config := &Config{Host: "localhost", Port: 0}

	fs := New(config)

	require.NotNil(t, fs)
	// Port 0 will default to 21 in buildLocation
	assert.Equal(t, "localhost:21", fs.(*fileSystem).CommonFileSystem.Location)
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

	fs := New(config)
	require.NotNil(t, fs)

	// Inject logger and metrics (mimicking AddFileStore behavior)
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

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
		Host:     "localhost",
		Port:     2121,
		User:     "testuser",
		Password: "testpass",
	}

	fs := New(config)
	require.NotNil(t, fs)

	assert.Equal(t, "localhost:2121", fs.(*fileSystem).CommonFileSystem.Location)

	// Inject logger and metrics
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "localhost:2121").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs.Connect()
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

	fs := New(config)

	require.NotNil(t, fs)

	assert.Equal(t, "localhost:2121/uploads", fs.(*fileSystem).CommonFileSystem.Location)

	// Inject logger and metrics
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "localhost:2121/uploads").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs.Connect()
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

	fs := New(config)
	require.NotNil(t, fs)

	assert.Equal(t, "localhost:2121", fs.(*fileSystem).CommonFileSystem.Location)

	// Inject logger and metrics
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "localhost:2121").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"FTP server %s not available, starting background retry: %v",
		gomock.Any(),
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs.Connect()
}

func TestNew_WithCustomDialTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &Config{
		Host:        "localhost",
		Port:        2121,
		User:        "testuser",
		Password:    "testpass",
		DialTimeout: 3 * time.Second,
	}

	fs := New(config)

	require.NotNil(t, fs)

	adapter := fs.(*fileSystem).CommonFileSystem.Provider.(*storageAdapter)
	assert.Equal(t, 3*time.Second, adapter.cfg.DialTimeout)
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
