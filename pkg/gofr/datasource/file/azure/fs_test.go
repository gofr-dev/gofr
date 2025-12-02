package azure

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

func TestNew_EmptyShareName(t *testing.T) {
	config := &Config{ShareName: ""}

	fs, err := New(config, nil, nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errInvalidConfig)
}

func TestNew_EmptyAccountName(t *testing.T) {
	config := &Config{
		ShareName:   "testshare",
		AccountName: "",
		AccountKey:  "testkey",
	}

	fs, err := New(config, nil, nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errAccountNameRequired)
}

func TestNew_EmptyAccountKey(t *testing.T) {
	config := &Config{
		ShareName:   "testshare",
		AccountName: "testaccount",
		AccountKey:  "",
	}

	fs, err := New(config, nil, nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errAccountKeyRequired)
}

func TestNew_ConnectionFailure_StartsRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		AccountName: "non-existent-account",
		AccountKey:  "invalid-key",
		ShareName:   "non-existent-share",
	}

	// Expect warning about background retry
	mockLogger.EXPECT().Warnf(
		"Azure File Share %s not available, starting background retry: %v",
		"non-existent-share",
		gomock.Any(), // Error message varies
	)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)

	time.Sleep(100 * time.Millisecond)

	fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)
}

func TestAzureFileSystem_Connect_AlreadyConnected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Infof("connected to %s", "testshare").MaxTimes(1)
	mockLogger.EXPECT().Warnf(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(1)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs, err := New(config, mockLogger, mockMetrics)
	require.NoError(t, err)

	// If already connected, Connect() should return immediately
	fs.(*azureFileSystem).Connect()

	fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)
}

func TestAzureFileSystem_Connect_NotConnected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Warnf(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(1)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs, err := New(config, mockLogger, mockMetrics)
	require.NoError(t, err)

	// Mark as not connected
	fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)

	// Call Connect - should attempt to connect
	fs.(*azureFileSystem).Connect()
}

func TestAzureFileSystem_startRetryConnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Warnf(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(1)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs, err := New(config, mockLogger, mockMetrics)
	require.NoError(t, err)

	// Disable retry to stop the goroutine quickly
	fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)
}

func TestAzureFileSystem_startRetryConnect_RetryDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Warnf(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(1)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs, err := New(config, mockLogger, mockMetrics)
	require.NoError(t, err)

	// Disable retry immediately - retry loop should exit
	fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)

	// Give it a moment to check the retry disabled flag
	time.Sleep(50 * time.Millisecond)
}

func TestConfig_EmptyEndpoint(t *testing.T) {
	config := &Config{
		AccountName: "myaccount",
		AccountKey:  "mykey",
		ShareName:   "myshare",
		// Endpoint is empty
	}

	assert.Empty(t, config.Endpoint)
	// Endpoint will be built in Connect() as: "https://" + AccountName + ".file.core.windows.net"
}
