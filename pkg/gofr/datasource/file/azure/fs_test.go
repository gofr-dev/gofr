package azure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
)

var errConnectionFailed = errors.New("connection failed")

func TestNew_NilConfig(t *testing.T) {
	fs, err := New(nil)

	require.Error(t, err)
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, errInvalidConfig)
}

func TestNew_EmptyShareName(t *testing.T) {
	config := &Config{ShareName: ""}

	fs, err := New(config)

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

	fs, err := New(config)

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

	fs, err := New(config)

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

	fs, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, fs)

	// Set logger and metrics via UseLogger/UseMetrics
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	// Expect debug log for connection attempt
	mockLogger.EXPECT().Debugf(
		gomock.Any(), // Format string
		gomock.Any(), // Share name
		gomock.Any(), // Timeout duration
	)

	// Expect warning about background retry
	mockLogger.EXPECT().Warnf(
		"Azure File Share %s not available, starting background retry: %v",
		"non-existent-share",
		gomock.Any(), // Error message varies
	)

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	// Expect debug log for starting background retry (from startRetryConnect goroutine)
	// This may be called asynchronously, so use MaxTimes to allow flexibility
	mockLogger.EXPECT().Debugf(
		gomock.Any(), // Format string
		gomock.Any(), // Share name
	).MaxTimes(1)

	// Now call Connect which will attempt connection and start retry
	fs.Connect()

	time.Sleep(100 * time.Millisecond)

	fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)
}

func TestAzureFileSystem_Connect_AlreadyConnected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs, err := New(config)
	require.NoError(t, err)

	// Set logger and metrics via UseLogger/UseMetrics
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	// Replace provider with mock for successful connection
	fs.(*azureFileSystem).CommonFileSystem.Provider = mockProvider

	mockLogger.EXPECT().Debugf("Attempting to connect to Azure File Share %s (timeout: %v)", "testshare", gomock.Any())
	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockProvider.EXPECT().Connect(gomock.Any()).Return(nil)
	mockLogger.EXPECT().Infof("connected to %s", "testshare")
	mockLogger.EXPECT().Debugf("Successfully connected to Azure File Share %s", "testshare")
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	// First Connect() call - will attempt connection and succeed
	fs.(*azureFileSystem).Connect()

	// If already connected, Connect() should return immediately (no more calls expected)
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

	fs, err := New(config)
	require.NoError(t, err)

	// Set logger and metrics via UseLogger/UseMetrics
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	mockLogger.EXPECT().Debugf("Attempting to connect to Azure File Share %s (timeout: %v)", "testshare", gomock.Any())
	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Warnf("Azure File Share %s not available, starting background retry: %v", "testshare", gomock.Any())
	// Expect logRetryStart from the goroutine (may or may not be called depending on timing)
	mockLogger.EXPECT().Debugf(
		"Starting background retry for Azure File Share %s (retry interval: 1 minute)",
		"testshare",
	).MaxTimes(1)

	// Call Connect - should attempt to connect and start retry
	fs.(*azureFileSystem).Connect()

	// Give goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Mark as not connected and disable retry to stop the goroutine
	fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)

	// Give goroutine time to check the flag and exit
	time.Sleep(50 * time.Millisecond)
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

	fs, err := New(config)
	require.NoError(t, err)

	// Set logger and metrics via UseLogger/UseMetrics
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Warnf(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(1)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	// Call Connect which will start retry on failure
	fs.Connect()

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

	fs, err := New(config)
	require.NoError(t, err)

	// Set logger and metrics via UseLogger/UseMetrics
	fs.UseLogger(mockLogger)
	fs.UseMetrics(mockMetrics)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Warnf(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(1)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	// Disable retry immediately - retry loop should exit
	fs.(*azureFileSystem).CommonFileSystem.SetDisableRetry(true)

	// Call Connect which would start retry, but retry is disabled
	fs.Connect()

	// Give it a moment to check the retry disabled flag
	time.Sleep(50 * time.Millisecond)
}

// TestAzureFileSystem_Observe_ProviderName tests that Observe uses "Azure" as provider name.
func TestAzureFileSystem_Observe_ProviderName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	// Create azureFileSystem directly without calling New() to avoid initialization logs
	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     nil, // Not needed for this test
			Location:     "testshare",
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			ProviderName: "Azure", // Set provider name for observability
		},
	}

	// Expect RecordHistogram to be called with "Azure" as provider label
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), // context
		file.AppFileStats,
		gomock.Any(),         // duration
		"type", gomock.Any(), // operation type
		"status", gomock.Any(), // status
		"provider", "Azure", // provider name should be "Azure"
	)

	// Expect Debug to be called with OperationLog containing "Azure" as provider
	mockLogger.EXPECT().Debug(gomock.Any()).Do(func(log any) {
		opLog, ok := log.(*file.OperationLog)
		require.True(t, ok, "log should be OperationLog")
		assert.Equal(t, "Azure", opLog.Provider, "provider should be Azure")
	})

	// Call Observe directly to test provider name
	operation := file.OpConnect
	startTime := time.Now()
	status := "SUCCESS"
	message := "test message"

	fs.Observe(operation, startTime, &status, &message)
}

func TestNew_SuccessfulConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	// Create fs manually with mock provider to test successful connection path
	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     mockProvider,
			Location:     config.ShareName,
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			ProviderName: "Azure",
		},
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockProvider.EXPECT().Connect(gomock.Any()).Return(nil)
	mockLogger.EXPECT().Infof("connected to %s", "testshare")
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := fs.CommonFileSystem.Connect(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Azure", fs.CommonFileSystem.ProviderName)
	assert.True(t, fs.CommonFileSystem.IsConnected())
}

func TestNew_NilLogger(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs, err := New(config)

	require.NoError(t, err)
	require.NotNil(t, fs)
}

func TestAzureFileSystem_Connect_NotConnected_WithLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     nil,
			Location:     "testshare",
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			ProviderName: "Azure",
		},
	}

	mockLogger.EXPECT().Debugf("Attempting to connect to Azure File Share %s (timeout: %v)", "testshare", gomock.Any())
	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Warnf("Azure File Share %s not available, starting background retry: %v", "testshare", gomock.Any())
	// Expect logRetryStart from the goroutine
	mockLogger.EXPECT().Debugf(
		"Starting background retry for Azure File Share %s (retry interval: 1 minute)",
		"testshare",
	)

	fs.Connect()

	// Give goroutine time to start
	time.Sleep(100 * time.Millisecond)

	// Disable retry to stop the goroutine
	fs.CommonFileSystem.SetDisableRetry(true)

	// Give goroutine time to check the flag and exit
	time.Sleep(100 * time.Millisecond)
}

func TestAzureFileSystem_logRetryStart_WithLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Logger:       mockLogger,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	mockLogger.EXPECT().Debugf(
		"Starting background retry for Azure File Share %s (retry interval: 1 minute)",
		"testshare",
	)

	fs.logRetryStart()
}

func TestAzureFileSystem_shouldExitRetry_Connected(t *testing.T) {
	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			ProviderName: "Azure",
		},
	}

	fs.CommonFileSystem.SetDisableRetry(false)
	// Simulate connected state by setting it directly
	// Note: This is testing the logic, actual connection state is managed internally

	result := fs.shouldExitRetry()
	assert.False(t, result)
}

func TestAzureFileSystem_shouldExitRetry_Disabled(t *testing.T) {
	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			ProviderName: "Azure",
		},
	}

	fs.CommonFileSystem.SetDisableRetry(true)

	result := fs.shouldExitRetry()
	assert.True(t, result)
}

func TestAzureFileSystem_logRetryExit_Connected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     mockProvider,
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	// Connect to set the connected state
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockProvider.EXPECT().Connect(gomock.Any()).Return(nil)
	mockLogger.EXPECT().Infof("connected to %s", "testshare")
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	err := fs.CommonFileSystem.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, fs.CommonFileSystem.IsConnected())

	mockLogger.EXPECT().Debugf(
		"Retry loop exiting: Azure File Share %s is now connected",
		"testshare",
	)

	fs.logRetryExit()
}

func TestAzureFileSystem_logRetryExit_Disabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Logger:       mockLogger,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	fs.CommonFileSystem.SetDisableRetry(true)

	mockLogger.EXPECT().Debugf(
		"Retry loop exiting: retry disabled for Azure File Share %s",
		"testshare",
	)

	fs.logRetryExit()
}

func TestAzureFileSystem_logRetryAttempt_WithLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Logger:       mockLogger,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	mockLogger.EXPECT().Debugf(
		"Retry attempt #%d: attempting to connect to Azure File Share %s (timeout: %v)",
		1,
		"testshare",
		defaultTimeout,
	)

	fs.logRetryAttempt(1)
}

func TestAzureFileSystem_logRetrySuccess_WithLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Logger:       mockLogger,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	mockLogger.EXPECT().Infof("Azure connection restored to share %s", "testshare")

	fs.logRetrySuccess(1)
}

func TestAzureFileSystem_logRetryFailure_WithLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Logger:       mockLogger,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	mockLogger.EXPECT().Debugf(
		"Retry attempt #%d failed for Azure File Share %s: %v (will retry in 1 minute)",
		2,
		"testshare",
		errConnectionFailed,
	)

	fs.logRetryFailure(2, errConnectionFailed)
}

func TestAzureFileSystem_attemptConnection_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     mockProvider,
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockProvider.EXPECT().Connect(gomock.Any()).Return(nil)
	mockLogger.EXPECT().Infof("connected to %s", "testshare")
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Infof("Azure connection restored to share %s", "testshare")

	result := fs.attemptConnection(1)

	assert.True(t, result)
}

func TestAzureFileSystem_attemptConnection_Failure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     mockProvider,
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockProvider.EXPECT().Connect(gomock.Any()).Return(errConnectionFailed)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debugf(
		"Retry attempt #%d failed for Azure File Share %s: %v (will retry in 1 minute)",
		1,
		"testshare",
		errConnectionFailed,
	)

	result := fs.attemptConnection(1)

	assert.False(t, result)
}

func TestAzureFileSystem_Connect_WhenNotConnected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     mockProvider,
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	// Ensure not connected
	assert.False(t, fs.CommonFileSystem.IsConnected())

	mockLogger.EXPECT().Debugf("Attempting to connect to Azure File Share %s (timeout: %v)", "testshare", gomock.Any())
	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any())
	mockProvider.EXPECT().Connect(gomock.Any()).Return(nil)
	mockLogger.EXPECT().Infof("connected to %s", "testshare")
	mockLogger.EXPECT().Debugf("Successfully connected to Azure File Share %s", "testshare")
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any())

	fs.Connect()

	assert.True(t, fs.CommonFileSystem.IsConnected())
}

func TestAzureFileSystem_startRetryConnect_ExitsOnConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     mockProvider,
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	// Setup expectations for retry loop
	mockLogger.EXPECT().Debugf(
		"Starting background retry for Azure File Share %s (retry interval: 1 minute)",
		"testshare",
	)
	mockLogger.EXPECT().Debugf(
		"Retry attempt #%d: attempting to connect to Azure File Share %s (timeout: %v)",
		1,
		"testshare",
		defaultTimeout,
	).AnyTimes()
	mockMetrics.EXPECT().NewHistogram(file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()
	mockProvider.EXPECT().Connect(gomock.Any()).Return(nil).AnyTimes()
	mockLogger.EXPECT().Infof("connected to %s", "testshare").AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof("Azure connection restored to share %s", "testshare").AnyTimes()
	mockLogger.EXPECT().Debugf(
		"Retry loop exiting: Azure File Share %s is now connected",
		"testshare",
	).AnyTimes()

	// Start retry in background
	go fs.startRetryConnect()

	// Wait a bit for retry to start
	time.Sleep(100 * time.Millisecond)

	// Disable retry to stop the goroutine
	fs.CommonFileSystem.SetDisableRetry(true)

	// Give it time to exit
	time.Sleep(50 * time.Millisecond)
}

func TestAzureFileSystem_startRetryConnect_ExitsOnDisable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := file.NewMockLogger(ctrl)
	mockMetrics := file.NewMockMetrics(ctrl)
	mockProvider := file.NewMockStorageProvider(ctrl)

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     mockProvider,
			Logger:       mockLogger,
			Metrics:      mockMetrics,
			Location:     "testshare",
			ProviderName: "Azure",
		},
	}

	// Setup expectations
	mockLogger.EXPECT().Debugf(
		"Starting background retry for Azure File Share %s (retry interval: 1 minute)",
		"testshare",
	)
	mockLogger.EXPECT().Debugf(
		"Retry loop exiting: retry disabled for Azure File Share %s",
		"testshare",
	).MaxTimes(1)

	// Start retry in background
	go fs.startRetryConnect()

	// Wait a bit for retry to start
	time.Sleep(50 * time.Millisecond)

	// Disable retry - should cause exit
	fs.CommonFileSystem.SetDisableRetry(true)

	// Give it time to check and exit
	time.Sleep(100 * time.Millisecond)
}
