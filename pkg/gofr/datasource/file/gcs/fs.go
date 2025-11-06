package gcs

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/option"
)

type FileSystem struct {
	*file.CommonFileSystem

	client  *storage.Client
	bucket  *storage.BucketHandle
	config  *Config
	logger  Logger
	metrics Metrics

	registerHistogram sync.Once
	disableRetry      bool
}

// Config represents the gcs configuration.
type Config struct {
	EndPoint        string
	BucketName      string
	CredentialsJSON string
	ProjectID       string
}

func New(config *Config) file.FileSystemProvider {
	return &FileSystem{config: config}
}

func (f *FileSystem) Connect() {
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

	f.logger.Debugf("connecting to GCS bucket: %s", f.config.BucketName)

	ctx := context.Background()

	var (
		client *storage.Client
		err    error
	)

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
		f.logger.Errorf("Failed to connect to GCS: %v", err)
		msg = err.Error()

		if !f.disableRetry {
			go f.startRetryConnect()
		}

		return
	}

	f.client = client
	f.bucket = client.Bucket(f.config.BucketName)

	provider := &storageAdapter{
		client: client,
		bucket: f.bucket,
	}

	f.CommonFileSystem = &file.CommonFileSystem{
		Provider: provider,
		Location: f.config.BucketName,
		Logger:   f.logger,
		Metrics:  f.metrics,
	}

	st = file.StatusSuccess
	msg = "GCS Client connected."

	f.logger.Infof("connected to GCS bucket %s", f.config.BucketName)
}

func (f *FileSystem) startRetryConnect() {
	ticker := time.NewTicker(time.Minute) // retry every 1 minute
	defer ticker.Stop()

	for {
		<-ticker.C

		ctx := context.Background()

		var (
			client *storage.Client
			err    error
		)

		switch {
		case f.config.EndPoint != "":
			client, err = storage.NewClient(
				ctx,
				option.WithEndpoint(f.config.EndPoint),
				option.WithoutAuthentication(),
			)
		case f.config.CredentialsJSON != "":
			client, err = storage.NewClient(
				ctx,
				option.WithCredentialsJSON([]byte(f.config.CredentialsJSON)),
			)
		default:
			client, err = storage.NewClient(ctx)
		}

		if err != nil {
			f.logger.Errorf("Retry: failed to connect to GCS: %v", err)
			continue
		}

		f.client = client
		f.bucket = client.Bucket(f.config.BucketName)

		provider := &storageAdapter{
			client: client,
			bucket: f.bucket,
		}

		f.CommonFileSystem = &file.CommonFileSystem{
			Provider: provider,
			Location: f.config.BucketName,
			Logger:   f.logger,
			Metrics:  f.metrics,
		}

		f.logger.Infof("GCS connection restored to bucket %s", f.config.BucketName)

		break
	}
}

// UseLogger sets the Logger interface for the FTP file system.
func (f *FileSystem) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the Metrics interface.
func (f *FileSystem) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}

// observe is a helper method for FileSystem-level operations.
func (f *FileSystem) observe(operation string, startTime time.Time, status, message *string) {
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
