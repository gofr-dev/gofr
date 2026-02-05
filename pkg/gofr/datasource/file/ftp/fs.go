package ftp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	errInvalidConfig   = errors.New("invalid FTP configuration: host and port are required")
	errInvalidProvider = errors.New("invalid FTP provider")
)

const defaultTimeout = 10 * time.Second

type fileSystem struct {
	*file.CommonFileSystem
}

// New creates and validates a new FTP file system.
// Returns error if connection fails or configuration is invalid.
func New(config *Config) file.FileSystemProvider {
	if config == nil {
		config = &Config{}
	}

	// Set default dial timeout if not specified
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}

	adapter := &storageAdapter{cfg: config}

	location := buildLocation(config)

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     adapter,
			Location:     location,
			ProviderName: "FTP",
		},
	}

	return fs
}

// buildLocation creates the location string for metrics/logging.
func buildLocation(config *Config) string {
	if config.Host == "" {
		return "ftp://unconfigured"
	}

	port := config.Port
	if port == 0 {
		port = 21 // Default FTP port
	}

	location := fmt.Sprintf("%s:%d", config.Host, port)

	if config.RemoteDir != "" && config.RemoteDir != "/" {
		location = fmt.Sprintf("%s:%d%s", config.Host, port, config.RemoteDir)
	}

	return location
}

// Connect tries a single immediate connect via provider; on failure it starts a background retry.
func (f *fileSystem) Connect() {
	if f.CommonFileSystem.IsConnected() {
		return
	}

	// Validate configuration before attempting connection
	if err := f.validateConfig(); err != nil {
		if f.CommonFileSystem.Logger != nil {
			f.CommonFileSystem.Logger.Errorf("Invalid FTP configuration: %v", err)
		}

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := f.CommonFileSystem.Connect(ctx)
	if err != nil {
		// Log warning if logger is available
		if f.CommonFileSystem.Logger != nil {
			f.CommonFileSystem.Logger.Warnf("FTP server %s not available, starting background retry: %v",
				f.CommonFileSystem.Location, err)
		}

		// Start background retry
		go f.startRetryConnect()

		return
	}

	// Connected successfully
	if f.CommonFileSystem.Logger != nil {
		f.CommonFileSystem.Logger.Infof("FTP connection established to server %s", f.CommonFileSystem.Location)
	}
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

// validateConfig checks if the configuration is valid.
func (f *fileSystem) validateConfig() error {
	adapter, ok := f.CommonFileSystem.Provider.(*storageAdapter)
	if !ok {
		return errInvalidProvider
	}

	cfg := adapter.cfg
	if cfg == nil || cfg.Host == "" || cfg.Port <= 0 {
		return errInvalidConfig
	}

	return nil
}
