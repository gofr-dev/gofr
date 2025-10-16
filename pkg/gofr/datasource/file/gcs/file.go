package gcs

import (
	"context"
	"errors"
	"io"
	"time"

	"gofr.dev/pkg/gofr/datasource/file"
)

type gcsWriter interface {
	Write(p []byte) (int, error)
	Close() error
}
type File struct {
	conn         gcsClient
	writer       gcsWriter
	name         string
	logger       Logger
	metrics      Metrics
	size         int64
	contentType  string
	body         io.ReadCloser
	lastModified time.Time
	isDir        bool
}

var (
	errNilGCSFileBody      = errors.New("gcs file body is nil")
	errSeekNotSupported    = errors.New("seek not supported on GCSFile")
	errReadAtNotSupported  = errors.New("readAt not supported on GCSFile")
	errWriteAtNotSupported = errors.New("writeAt not supported on GCSFile (read-only)")
)

const (
	msgWriterClosed = "Writer closed successfully"
	msgReaderClosed = "Reader closed successfully"
)

// ====== File interface methods ======

func (f *File) Read(p []byte) (int, error) {
	if f.body == nil {
		f.logger.Debug("GCS file body is nil")
		return 0, errNilGCSFileBody
	}

	return f.body.Read(p)
}
func (f *File) Write(p []byte) (int, error) {
	bucketName := getBucketName(f.name)

	var msg string

	st := file.StatusError

	startTime := time.Now()

	defer file.ObserveFileOperation(&file.OperationObservability{
		Context: context.Background(), Logger: f.logger, Metrics: f.metrics, Operation: "WRITE",
		Location: getLocation(bucketName), Provider: "GCS", StartTime: startTime, Status: &st, Message: &msg})

	n, err := f.writer.Write(p)
	if err != nil {
		f.logger.Errorf("failed to write: %v", err)
		msg = err.Error()

		return n, err
	}

	st, msg = file.StatusSuccess, "Write successful"
	f.logger.Debug(msg)

	return n, nil
}

func (f *File) Close() error {
	bucketName := getBucketName(f.name)

	var msg string

	st := file.StatusError

	startTime := time.Now()

	defer file.ObserveFileOperation(&file.OperationObservability{
		Context: context.Background(), Logger: f.logger, Metrics: f.metrics, Operation: "CLOSE",
		Location: getLocation(bucketName), Provider: "GCS", StartTime: startTime, Status: &st, Message: &msg})

	if f.writer != nil {
		err := f.writer.Close()
		if err != nil {
			msg = err.Error()
			return err
		}

		st = file.StatusSuccess

		msg = msgWriterClosed

		f.logger.Debug(msg)

		return nil
	}

	if f.body != nil {
		err := f.body.Close()
		if err != nil {
			msg = err.Error()
			return err
		}

		st = file.StatusSuccess

		msg = msgReaderClosed

		f.logger.Debug(msgReaderClosed)

		return nil
	}

	st = file.StatusSuccess

	msg = msgWriterClosed

	return nil
}

func (*File) Seek(_ int64, _ int) (int64, error) {
	// Not supported: Seek requires reopening with range.
	return 0, errSeekNotSupported
}

func (*File) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, errReadAtNotSupported
}

func (*File) WriteAt(_ []byte, _ int64) (int, error) {
	return 0, errWriteAtNotSupported
}
