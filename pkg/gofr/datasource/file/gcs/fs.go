package gcs

import (
	"context"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	errInvalidConfig = errors.New("invalid GCS configuration: bucket name is required")
)

const defaultTimeout = 10 * time.Second

type fileSystem struct {
	*file.CommonFileSystem
}

// Config represents the gcs configuration.
type Config struct {
	EndPoint        string
	BucketName      string
	CredentialsJSON string
	ProjectID       string
}

// New creates and validates a new GCS file system.
// Returns error if connection fails.
func New(config *Config, logger datasource.Logger, metrics file.StorageMetrics) (file.FileSystemProvider, error) {
	if config == nil || config.BucketName == "" {
		return nil, errInvalidConfig
	}

	adapter := &storageAdapter{cfg: config}

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider: adapter,
			Location: config.BucketName,
			Logger:   logger,
			Metrics:  metrics,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// use CommonFileSystem.Connect for bookkeeping (fast-path since adapter is connected)
	if err := fs.CommonFileSystem.Connect(ctx); err != nil {
		if logger != nil {
			logger.Warnf("GCS bucket %s not available, starting background retry: %v", config.BucketName, err)
		}

		go fs.startRetryConnect()

		return fs, nil
	}

	// Connected successfully
	return fs, nil
}

// Connect tries a single immediate connect via provider; on failure it starts a background retry.
func (f *fileSystem) Connect() {
	if f.CommonFileSystem.IsConnected() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	_ = f.CommonFileSystem.Connect(ctx)
}

// startRetryConnect repeatedly calls provider.Connect until success, then delegates to CommonFileSystem.Connect.
// startRetryConnect retries connection every 30 seconds until success.
func (f *fileSystem) startRetryConnect() {
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
				f.CommonFileSystem.Logger.Infof("GCS connection restored to bucket %s", f.CommonFileSystem.Location)
			}

			return
		}

		// Still failing - log and continue retrying
		if f.CommonFileSystem.Logger != nil {
			f.CommonFileSystem.Logger.Debugf("GCS retry failed, will try again: %v", err)
		}
	}
}
