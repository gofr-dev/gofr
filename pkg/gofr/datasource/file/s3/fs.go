package s3

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
)

const (
	defaultTimeout = 10 * time.Second
	retryInterval  = time.Minute
)

type fileSystem struct {
	*file.CommonFileSystem
	adapter *storageAdapter
}

// Config represents the s3 configuration.
type Config struct {
	// EndPoint is the S3 base endpoint. It is optional on Amazon S3, where the SDK resolves
	// the regional endpoint from Region; set it for S3-compatible services and local emulators.
	EndPoint        string
	BucketName      string // AWS Bucket name
	Region          string // AWS Region
	AccessKeyID     string // Aws configs
	SecretAccessKey string // Aws configs

	// SessionToken is an optional temporary-credentials session token (e.g. AWS STS
	// credentials or scoped Cloudflare R2 API tokens). Leave empty for long-lived
	// access keys.
	SessionToken string

	// Flavor selects provider-specific defaults for S3-compatible services such as
	// Cloudflare R2, MinIO, DigitalOcean Spaces, and Backblaze B2. An empty Flavor
	// targets Amazon S3. See flavor.go for the available presets.
	Flavor Flavor

	// UsePathStyle overrides the Flavor's addressing-style default when non-nil.
	// Path-style addresses objects as host/bucket/key; virtual-hosted-style uses
	// bucket.host/key.
	UsePathStyle *bool
}

// New initializes a new instance of the S3 file system with the provided configuration.
//
// It returns a file.CloudFileSystem, so callers get compile-time access to the
// cloud-specific capabilities (GenerateSignedURL, CreateWithOptions) in addition to
// the standard FileSystem operations.
func New(config *Config) file.CloudFileSystem {
	if config == nil {
		config = &Config{}
	}

	adapter := &storageAdapter{cfg: config}

	return &fileSystem{
		CommonFileSystem: &file.CommonFileSystem{
			Provider:     adapter,
			Location:     config.BucketName,
			ProviderName: "S3", // Set provider name for observability
		},
		adapter: adapter,
	}
}

// UseLogger sets the logger on both the common file system and the storage adapter.
func (f *fileSystem) UseLogger(logger any) {
	f.CommonFileSystem.UseLogger(logger)

	if l, ok := logger.(datasource.Logger); ok {
		f.adapter.logger = l
	}
}

// Connect tries a single immediate connect; on failure it starts a background retry.
func (f *fileSystem) Connect() {
	if f.IsConnected() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if err := f.CommonFileSystem.Connect(ctx); err != nil {
		if f.Logger != nil {
			f.Logger.Errorf("S3 bucket %s not available, starting background retry: %v", f.Location, err)
		}

		go f.startRetryConnect()

		return
	}

	if f.Logger != nil {
		f.Logger.Infof("S3 connection established to bucket %s", f.Location)
	}
}

// startRetryConnect retries the connection every minute until it succeeds.
func (f *fileSystem) startRetryConnect() {
	if f.IsConnected() || f.IsRetryDisabled() {
		return
	}

	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	for range ticker.C {
		if f.IsConnected() || f.IsRetryDisabled() {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)

		err := f.CommonFileSystem.Connect(ctx)

		cancel()

		if err == nil {
			if f.Logger != nil {
				f.Logger.Infof("S3 connection restored to bucket %s", f.Location)
			}

			return
		}

		if f.Logger != nil {
			f.Logger.Debugf("S3 retry failed, will try again: %v", err)
		}
	}
}
