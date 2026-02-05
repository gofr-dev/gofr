package azure

import (
	"context"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	errInvalidConfig       = errors.New("invalid Azure configuration: share name is required")
	errAccountNameRequired = errors.New("invalid Azure configuration: account name is required")
	errAccountKeyRequired  = errors.New("invalid Azure configuration: account key is required")
)

const defaultTimeout = 10 * time.Second

type azureFileSystem struct {
	*file.CommonFileSystem
}

// Config represents the Azure File Storage configuration.
type Config struct {
	AccountName string // Azure Storage Account name
	AccountKey  string // Azure Storage Account key
	ShareName   string // Azure File Share name
	Endpoint    string // Azure Storage endpoint (optional, defaults to core.windows.net)
}

// New creates and validates a new Azure File Storage file system.
// Returns error if configuration is invalid.
// Connection will be established when Connect() is called.
func New(config *Config) (file.FileSystemProvider, error) {
	if config == nil {
		return nil, errInvalidConfig
	}

	if config.ShareName == "" {
		return nil, errInvalidConfig
	}

	if config.AccountName == "" {
		return nil, errAccountNameRequired
	}

	if config.AccountKey == "" {
		return nil, errAccountKeyRequired
	}

	adapter := &storageAdapter{cfg: config}

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     adapter,
			Location:     config.ShareName,
			ProviderName: "Azure", // Set provider name for observability
		},
	}

	return fs, nil
}

// Connect tries a single immediate connect via provider; on failure it starts a background retry.
func (f *azureFileSystem) Connect() {
	if f.CommonFileSystem.IsConnected() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if f.CommonFileSystem.Logger != nil {
		f.CommonFileSystem.Logger.Debugf("Attempting to connect to Azure File Share %s (timeout: %v)",
			f.CommonFileSystem.Location, defaultTimeout)
	}

	// Use CommonFileSystem.Connect for bookkeeping
	if err := f.CommonFileSystem.Connect(ctx); err != nil {
		if f.CommonFileSystem.Logger != nil {
			f.CommonFileSystem.Logger.Warnf("Azure File Share %s not available, starting background retry: %v", f.CommonFileSystem.Location, err)
		}

		go f.startRetryConnect()

		return
	}

	// Connected successfully
	if f.CommonFileSystem.Logger != nil {
		f.CommonFileSystem.Logger.Debugf("Successfully connected to Azure File Share %s", f.CommonFileSystem.Location)
	}
}

// startRetryConnect repeatedly calls provider.Connect until success.
func (f *azureFileSystem) startRetryConnect() {
	f.logRetryStart()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	retryCount := 0

	for range ticker.C {
		if f.shouldExitRetry() {
			f.logRetryExit()
			return
		}

		retryCount++

		f.logRetryAttempt(retryCount)

		if f.attemptConnection(retryCount) {
			return
		}
	}
}

// logRetryStart logs the start of background retry.
func (f *azureFileSystem) logRetryStart() {
	if f.CommonFileSystem.Logger != nil {
		f.CommonFileSystem.Logger.Debugf(
			"Starting background retry for Azure File Share %s (retry interval: 1 minute)",
			f.CommonFileSystem.Location,
		)
	}
}

// shouldExitRetry checks if retry loop should exit.
func (f *azureFileSystem) shouldExitRetry() bool {
	return f.CommonFileSystem.IsConnected() || f.CommonFileSystem.IsRetryDisabled()
}

// logRetryExit logs the exit reason from retry loop.
func (f *azureFileSystem) logRetryExit() {
	if f.CommonFileSystem.Logger == nil {
		return
	}

	if f.CommonFileSystem.IsConnected() {
		f.CommonFileSystem.Logger.Debugf(
			"Retry loop exiting: Azure File Share %s is now connected",
			f.CommonFileSystem.Location,
		)
	} else {
		f.CommonFileSystem.Logger.Debugf(
			"Retry loop exiting: retry disabled for Azure File Share %s",
			f.CommonFileSystem.Location,
		)
	}
}

// logRetryAttempt logs a retry attempt.
func (f *azureFileSystem) logRetryAttempt(retryCount int) {
	if f.CommonFileSystem.Logger != nil {
		f.CommonFileSystem.Logger.Debugf(
			"Retry attempt #%d: attempting to connect to Azure File Share %s (timeout: %v)",
			retryCount,
			f.CommonFileSystem.Location,
			defaultTimeout,
		)
	}
}

// attemptConnection attempts to connect and returns true if successful.
func (f *azureFileSystem) attemptConnection(retryCount int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := f.CommonFileSystem.Connect(ctx)
	if err == nil {
		f.logRetrySuccess(retryCount)
		return true
	}

	f.logRetryFailure(retryCount, err)

	return false
}

// logRetrySuccess logs successful retry.
func (f *azureFileSystem) logRetrySuccess(_ int) {
	if f.CommonFileSystem.Logger != nil {
		f.CommonFileSystem.Logger.Infof("Azure connection restored to share %s", f.CommonFileSystem.Location)
	}
}

// logRetryFailure logs failed retry attempt.
func (f *azureFileSystem) logRetryFailure(retryCount int, err error) {
	if f.CommonFileSystem.Logger != nil {
		f.CommonFileSystem.Logger.Debugf(
			"Retry attempt #%d failed for Azure File Share %s: %v (will retry in 1 minute)",
			retryCount,
			f.CommonFileSystem.Location,
			err,
		)
	}
}
