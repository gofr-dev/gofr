package azure

import (
	"context"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	errInvalidConfig = errors.New("invalid Azure configuration: share name is required")
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
// Returns error if connection fails.
func New(config *Config, logger datasource.Logger, metrics file.StorageMetrics) (file.FileSystemProvider, error) {
	if config == nil || config.ShareName == "" {
		return nil, errInvalidConfig
	}

	adapter := &storageAdapter{cfg: config}

	fs := &azureFileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider: adapter,
			Location: config.ShareName,
			Logger:   logger,
			Metrics:  metrics,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Use CommonFileSystem.Connect for bookkeeping
	if err := fs.CommonFileSystem.Connect(ctx); err != nil {
		if logger != nil {
			logger.Warnf("Azure File Share %s not available, starting background retry: %v", config.ShareName, err)
		}

		go fs.startRetryConnect()

		return fs, nil
	}

	// Connected successfully
	return fs, nil
}

// Connect tries a single immediate connect via provider; on failure it starts a background retry.
func (f *azureFileSystem) Connect() {
	if f.CommonFileSystem.IsConnected() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	_ = f.CommonFileSystem.Connect(ctx)
}

// startRetryConnect repeatedly calls provider.Connect until success.
func (f *azureFileSystem) startRetryConnect() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if f.CommonFileSystem.IsConnected() || f.CommonFileSystem.IsRetryDisabled() {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)

		err := f.CommonFileSystem.Connect(ctx)

		cancel()

		if err == nil {
			// Success - exit retry loop
			if f.CommonFileSystem.Logger != nil {
				f.CommonFileSystem.Logger.Infof("Azure connection restored to share %s", f.CommonFileSystem.Location)
			}

			return
		}

		// Still failing - log and continue retrying
		if f.CommonFileSystem.Logger != nil {
			f.CommonFileSystem.Logger.Debugf("Azure retry failed, will try again: %v", err)
		}
	}
}
