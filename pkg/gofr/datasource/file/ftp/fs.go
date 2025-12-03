package ftp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	errInvalidConfig = errors.New("invalid FTP configuration: host and port are required")
)

const defaultTimeout = 10 * time.Second

type fileSystem struct {
	*file.CommonFileSystem
}

// New creates and validates a new FTP file system.
// Returns error if connection fails or configuration is invalid.
func New(config *Config, logger datasource.Logger, metrics file.StorageMetrics) (file.FileSystemProvider, error) {
	if config == nil || config.Host == "" || config.Port <= 0 {
		return nil, errInvalidConfig
	}

	// Set default dial timeout if not specified
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}

	adapter := &storageAdapter{cfg: config}

	location := fmt.Sprintf("%s:%d", config.Host, config.Port)
	if config.RemoteDir != "" && config.RemoteDir != "/" {
		location = fmt.Sprintf("%s:%d%s", config.Host, config.Port, config.RemoteDir)
	}

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider: adapter,
			Location: location,
			Logger:   logger,
			Metrics:  metrics,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Attempt initial connection via CommonFileSystem.Connect
	if err := fs.CommonFileSystem.Connect(ctx); err != nil {
		if logger != nil {
			logger.Warnf("FTP server %s not available, starting background retry: %v", config.Host, err)
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

// startRetryConnect retries connection every 30 seconds until success.
func (f *fileSystem) startRetryConnect() {
	if f.CommonFileSystem.IsConnected() || f.CommonFileSystem.IsRetryDisabled() {
		return
	}

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
				f.CommonFileSystem.Logger.Infof("FTP connection restored to server %s", f.CommonFileSystem.Location)
			}

			return
		}

		// Still failing - log and continue retrying
		if f.CommonFileSystem.Logger != nil {
			f.CommonFileSystem.Logger.Debugf("FTP retry failed, will try again: %v", err)
		}
	}
}
