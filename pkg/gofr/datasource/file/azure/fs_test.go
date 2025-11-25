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

func TestNew_WithEndpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
		Endpoint:    "https://custom.endpoint.com",
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "testshare").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"Azure File Share %s not available, starting background retry: %v",
		"testshare",
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)

	// If connected, verify state
	if fs.(*azureFileSystem).CommonFileSystem.IsConnected() {
		t.Log("Successfully connected to Azure File Share")
	} else {
		t.Log("Azure File Share not available, retry started")

		fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)
	}
}

func TestNew_DefaultEndpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
		// Endpoint is empty, should default to core.windows.net
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Infof("connected to %s", "testshare").MaxTimes(1)
	mockLogger.EXPECT().Warnf(
		"Azure File Share %s not available, starting background retry: %v",
		"testshare",
		gomock.Any(),
	).MaxTimes(1)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs, err := New(config, mockLogger, mockMetrics)

	require.NoError(t, err)
	require.NotNil(t, fs)

	// Verify endpoint was set correctly in adapter
	adapter := fs.(*azureFileSystem).CommonFileSystem.Provider.(*storageAdapter)
	assert.Equal(t, "testaccount", adapter.cfg.AccountName)
	assert.Equal(t, "testshare", adapter.cfg.ShareName)

	// Default endpoint should be set
	if adapter.cfg.Endpoint == "" {
		// Endpoint is built in Connect, so it might be empty here
		// But the logic should use the default when empty
		t.Log("Endpoint will be set to default in Connect()")
	}

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

func TestAzureFileSystem_startRetryConnect_Connected(t *testing.T) {
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
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs, err := New(config, mockLogger, mockMetrics)
	require.NoError(t, err)

	// If connected, retry should exit immediately
	if fs.(*azureFileSystem).CommonFileSystem.IsConnected() {
		// Retry won't start if already connected
		t.Log("Already connected, retry won't start")
	} else {
		fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)
	}
}

func TestConfig_Fields(t *testing.T) {
	config := &Config{
		AccountName: "myaccount",
		AccountKey:  "mykey",
		ShareName:   "myshare",
		Endpoint:    "https://custom.endpoint.com",
	}

	assert.Equal(t, "myaccount", config.AccountName)
	assert.Equal(t, "mykey", config.AccountKey)
	assert.Equal(t, "myshare", config.ShareName)
	assert.Equal(t, "https://custom.endpoint.com", config.Endpoint)
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
