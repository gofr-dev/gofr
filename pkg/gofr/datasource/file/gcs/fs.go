package gcs

import (
	"context"
	"errors"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/option"
)

var (
	errInvalidConfig    = errors.New("invalid GCS configuration: bucket name is required")
	errConnectionFailed = errors.New("failed to connect to GCS")
)

const gcsClientConnected = "GCS Client connected."

type fileSystem struct {
	*file.CommonFileSystem

	client  *storage.Client
	bucket  *storage.BucketHandle
	config  *Config
	logger  datasource.Logger
	metrics file.StorageMetrics

	registerHistogram sync.Once
	connected         bool
	disableRetry      bool
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
func New(config *Config) (file.FileSystemProvider, error) {
	if config == nil || config.BucketName == "" {
		return nil, errInvalidConfig
	}

	fs := &fileSystem{
		config: config,
	}

	// Try initial connection to validate credentials
	if err := fs.tryConnect(context.Background()); err != nil {
		return nil, errors.Join(errConnectionFailed, err)
	}

	return fs, nil
}

// tryConnect attempts to establish a connection and validate credentials.
func (f *fileSystem) tryConnect(ctx context.Context) error {
	var (
		client *storage.Client
		err    error
	)

	if f.logger != nil {
		f.logger.Debugf("connecting to GCS bucket: %s", f.config.BucketName)
	}

	switch {
	case f.config.EndPoint != "":
		// Local emulator mode
		client, err = storage.NewClient(
			ctx,
			option.WithEndpoint(f.config.EndPoint),
			option.WithoutAuthentication(),
		)

	case f.config.CredentialsJSON != "":
		// Direct JSON mode
		client, err = storage.NewClient(
			ctx,
			option.WithCredentialsJSON([]byte(f.config.CredentialsJSON)),
		)

	default:
		// Env var mode (GOOGLE_APPLICATION_CREDENTIALS)
		client, err = storage.NewClient(ctx)
	}

	if err != nil {
		return err
	}

	// Validate connection by checking bucket access
	bucket := client.Bucket(f.config.BucketName)
	if _, err := bucket.Attrs(ctx); err != nil {
		client.Close()

		return err
	}

	f.client = client
	f.bucket = bucket

	return nil
}

func (f *fileSystem) Connect() {
	var msg string

	st := file.StatusError

	startTime := time.Now()

	defer f.observe(file.OpConnect, startTime, &st, &msg)

	f.registerHistogram.Do(func() {
		f.metrics.NewHistogram(
			file.AppFileStats,
			"App GCS Stats - duration of file operations",
			file.DefaultHistogramBuckets()...,
		)
	})

	if f.client != nil && f.bucket != nil {
		provider := &storageAdapter{
			client: f.client,
			bucket: f.bucket,
		}

		f.CommonFileSystem = &file.CommonFileSystem{
			Provider: provider,
			Location: f.config.BucketName,
			Logger:   f.logger,
			Metrics:  f.metrics,
		}

		f.connected = true

		st = file.StatusSuccess
		msg = gcsClientConnected

		f.logger.Infof("connected to GCS bucket %s", f.config.BucketName)

		return
	}

	// Fallback: try to reconnect if somehow client is nil
	f.logger.Debugf("attempting to reconnect to GCS bucket: %s", f.config.BucketName)

	ctx := context.Background()
	if err := f.tryConnect(ctx); err != nil {
		if f.logger != nil {
			f.logger.Errorf("Failed to connect to GCS: %v", err)
		}

		msg = err.Error()

		// Start retry goroutine
		go f.startRetryConnect()

		return
	}

	provider := &storageAdapter{
		client: f.client,
		bucket: f.bucket,
	}

	f.CommonFileSystem = &file.CommonFileSystem{
		Provider: provider,
		Location: f.config.BucketName,
		Logger:   f.logger,
		Metrics:  f.metrics,
	}

	f.connected = true

	st = file.StatusSuccess
	msg = gcsClientConnected

	f.logger.Infof("connected to GCS bucket %s", f.config.BucketName)
}

func (f *fileSystem) startRetryConnect() {
	ticker := time.NewTicker(time.Minute) // retry every 1 minute
	defer ticker.Stop()

	for {
		<-ticker.C

		if f.connected || f.disableRetry {
			return // Already connected
		}

		ctx := context.Background()

		if err := f.tryConnect(ctx); err != nil {
			f.logger.Errorf("Retry: failed to connect to GCS: %v", err)

			continue
		}

		provider := &storageAdapter{
			client: f.client,
			bucket: f.bucket,
		}

		f.CommonFileSystem = &file.CommonFileSystem{
			Provider: provider,
			Location: f.config.BucketName,
			Logger:   f.logger,
			Metrics:  f.metrics,
		}

		f.connected = true

		f.logger.Infof("GCS connection restored to bucket %s", f.config.BucketName)

		break
	}
}

// UseLogger sets the Logger interface for the FTP file system.
func (f *fileSystem) UseLogger(logger any) {
	if l, ok := logger.(datasource.Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the Metrics interface.
func (f *fileSystem) UseMetrics(metrics any) {
	if m, ok := metrics.(file.StorageMetrics); ok {
		f.metrics = m
	}
}

// observe is a helper method for FileSystem-level operations.
func (f *fileSystem) observe(operation string, startTime time.Time, status, message *string) {
	file.ObserveOperation(&file.OperationObservability{
		Context:   context.Background(),
		Logger:    f.logger,
		Metrics:   f.metrics,
		Operation: operation,
		Location:  f.config.BucketName,
		Provider:  "GCS",
		StartTime: startTime,
		Status:    status,
		Message:   message,
	})
}
