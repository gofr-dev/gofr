package gcs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/option"
)

var (
	errOperationNotPermitted = errors.New("operation not permitted")
	errWriterTypeAssertion   = errors.New("writer is not of type *storage.Writer")
)

type FileSystem struct {
	file.CommonFileSystem

	conn    file.StorageProvider
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

	var client *storage.Client

	var err error

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

	f.conn = &storageAdapter{
		client: client,
		bucket: client.Bucket(f.config.BucketName),
	}

	f.CommonFileSystem = file.CommonFileSystem{
		Provider: f.conn,
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

		f.conn = &storageAdapter{
			client: client,
			bucket: client.Bucket(f.config.BucketName),
		}

		f.CommonFileSystem = file.CommonFileSystem{
			Provider: f.conn,
			Location: f.config.BucketName,
			Logger:   f.logger,
			Metrics:  f.metrics,
		}

		f.logger.Infof("GCS connection restored to bucket %s", f.config.BucketName)

		break
	}
}

func (f *FileSystem) Create(name string) (file.File, error) {
	var (
		msg string
		st  = file.StatusError
	)

	startTime := time.Now()

	defer f.observe(file.OpCreate, startTime, &st, &msg)

	ctx := context.Background()

	// 1. Check if parent directory exists
	parentPath := path.Dir(name)
	checkPath := "."

	if parentPath != "." {
		checkPath = parentPath + "/"
	}

	if _, err := f.conn.ListObjects(ctx, checkPath); err != nil {
		msg = "Parent directory does not exist"

		f.logger.Errorf("Failed to list parent directory %q: %v", checkPath, err)

		return nil, err
	}

	originalName := name

	for index := 1; ; index++ {
		objs, err := f.conn.ListObjects(ctx, name)
		if err != nil {
			msg = "Error checking existing objects"

			f.logger.Errorf("Failed to list objects for name %q: %v", name, err)

			return nil, err
		}

		if len(objs) == 0 {
			break // Safe to use
		}

		name = file.GenerateCopyName(originalName, index)
	}

	// 3. Open writer
	writer := f.conn.NewWriter(ctx, name)

	if fw, ok := writer.(*failWriter); ok {
		msg = "Invalid writer"
		_ = fw.Close()

		return nil, fmt.Errorf("failed to create writer: %w", fw.err)
	}

	sw, ok := writer.(*storage.Writer)
	if !ok {
		msg = "Failed to assert writer to *storage.Writer"

		f.logger.Errorf("Type assertion failed for writer to *storage.Writer")

		return nil, fmt.Errorf("type assertion failed: %w", errWriterTypeAssertion)
	}

	st = file.StatusSuccess
	msg = "Write stream opened successfully"

	f.logger.Infof("Write stream successfully opened for file %q", name)

	return &File{
		conn:         f.conn,
		writer:       sw,
		name:         name,
		contentType:  sw.ContentType,
		size:         sw.Size,
		lastModified: time.Now(),
		logger:       f.logger,
		metrics:      f.metrics,
	}, nil
}

func (f *FileSystem) Remove(name string) error {
	var msg string

	st := file.StatusError

	startTime := time.Now()

	defer f.observe(file.OpRemove, startTime, &st, &msg)

	ctx := context.Background()

	err := f.conn.DeleteObject(ctx, name)
	if err != nil {
		f.logger.Errorf("Error while deleting file: %v", err)
		msg = err.Error()

		return err
	}

	st = file.StatusSuccess
	msg = "File deletion on GCS successful"

	f.logger.Infof("File with path %q deleted", name)

	return nil
}

func (f *FileSystem) Open(name string) (file.File, error) {
	var msg string

	st := file.StatusError

	startTime := time.Now()

	defer f.observe(file.OpOpen, startTime, &st, &msg)

	ctx := context.Background()

	reader, err := f.conn.NewReader(ctx, name)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			msg = "File not found"

			return nil, file.ErrFileNotFound
		}

		f.logger.Errorf("failed to retrieve %q: %v", name, err)
		msg = err.Error()

		return nil, err
	}

	objInfo, err := f.conn.StatObject(ctx, name)
	if err != nil {
		reader.Close()

		msg = err.Error()

		return nil, err
	}

	st = file.StatusSuccess

	msg = fmt.Sprintf("File with path %q retrieved successfully", name)

	return &File{
		conn:         f.conn,
		name:         name,
		body:         reader,
		logger:       f.logger,
		metrics:      f.metrics,
		size:         objInfo.Size,
		contentType:  objInfo.ContentType,
		lastModified: objInfo.LastModified,
	}, nil
}

func (f *FileSystem) Rename(oldname, newname string) error {
	var msg string

	st := file.StatusError

	startTime := time.Now()

	defer f.observe(file.OpRename, startTime, &st, &msg)

	ctx := context.Background()

	if oldname == newname {
		st = file.StatusSuccess
		msg = "Source and destination names are the same"

		f.logger.Infof("%q & %q are same", oldname, newname)

		return nil
	}

	if path.Dir(oldname) != path.Dir(newname) {
		msg = "Renaming to different location not permitted"

		f.logger.Errorf("%q & %q are not in same location", oldname, newname)

		return fmt.Errorf("%w: renaming as well as moving file to different location is not allowed", errOperationNotPermitted)
	}
	// Copy old object to new
	if err := f.conn.CopyObject(ctx, oldname, newname); err != nil {
		msg = fmt.Sprintf("Error while copying file: %v", err)
		return err
	}

	// Delete old
	err := f.conn.DeleteObject(ctx, oldname)
	if err != nil {
		msg = fmt.Sprintf("failed to remove old file %s", oldname)
		return err
	}

	st = file.StatusSuccess
	msg = "File renamed successfully"

	f.logger.Infof("File with path %q renamed to %q", oldname, newname)

	return nil
}
func (f *FileSystem) OpenFile(name string, _ int, _ os.FileMode) (file.File, error) {
	return f.Open(name)
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
		Location:  getLocation(f.config.BucketName),
		Provider:  "GCS",
		StartTime: startTime,
		Status:    status,
		Message:   message,
	})
}
