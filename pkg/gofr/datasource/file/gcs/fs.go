package gcs

import (
	"context"
	"errors"
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

// New creates and validates a new GCS file system (generic FileSystemProvider).
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

// NewCloudFileSystem creates a GCS filesystem with the CloudFileSystem interface.
// This is a convenience wrapper around New() for users who need compile-time
// verification that cloud-specific features (metadata, signed URLs) are available.
//
// Example:
//
//	cfs, err := gcs.NewCloudFileSystem(&gcs.Config{BucketName: "my-bucket"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// CreateWithOptions available without type assertion
//	file, _ := cfs.CreateWithOptions(ctx, "file.csv", &file.FileOptions{
//	    ContentType: "text/csv",
//	})
//
// Note: New() returns FileSystemProvider which also supports these features
// through type assertion. Use NewCloudFileSystem only when you want the
// CloudFileSystem interface explicitly.
func NewCloudFileSystem(config *Config) (file.CloudFileSystem, error) {
	fs := New(config)

	cfs, ok := fs.(file.CloudFileSystem)
	if !ok {
		return nil, errors.New("provider does not support cloud features")
	}

	return cfs, nil
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
