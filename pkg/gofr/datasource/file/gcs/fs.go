package gcs

import (
	"context"
	"os"
	"time"

	"gofr.dev/pkg/gofr/datasource/file"
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
func New(config *Config) file.FileSystemProvider {
	if config == nil {
		config = &Config{}
	}

	adapter := &storageAdapter{cfg: config}

	fs := &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     adapter,
			Location:     config.BucketName,
			ProviderName: "GCS", // Set provider name for observability
		},
	}

	return fs
}

// Connect tries a single immediate connect via provider; on failure it starts a background retry.
func (f *fileSystem) Connect() {
	if f.CommonFileSystem.IsConnected() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := f.CommonFileSystem.Connect(ctx)
	if err != nil {
		if f.CommonFileSystem.Logger != nil {
			f.CommonFileSystem.Logger.Errorf("GCS bucket %s not available, starting background retry: %v",
				f.CommonFileSystem.Location, err)
		}

		// Start background retry
		go f.startRetryConnect()

		return
	}

	// Connected successfully
	if f.CommonFileSystem.Logger != nil {
		f.CommonFileSystem.Logger.Infof("GCS connection established to bucket %s", f.CommonFileSystem.Location)
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

func (f *fileSystem) Create(name string, opts ...*file.FileOptions) (file.File, error) {
	ctx := context.Background()
	var opt *file.FileOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	writer := f.Provider.(*storageAdapter).NewWriterWithOptions(ctx, name, opt)
	return file.NewCommonFileWriter(
		f.Provider,
		name,
		writer,
		f.Logger,
		f.Metrics,
		f.Location,
	), nil
}

func (f *fileSystem) SignedURL(name string, expiry time.Duration, opts ...*file.FileOptions) (string, error) {
	ctx := context.Background()
	return f.Provider.(*storageAdapter).SignedURL(ctx, name, expiry, opts...)
}
