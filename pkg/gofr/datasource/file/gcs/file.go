package gcs

import (
	"context"
	"errors"
	"io"
	"time"
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
	errNilGCSFileBody      = errors.New("GCS file body is nil")
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
		return 0, errNilGCSFileBody
	}

	return f.body.Read(p)
}
func (f *File) Write(p []byte) (int, error) {
	bucketName := getBucketName(f.name)

	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "WRITE",
		Location:  getLocation(bucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	n, err := f.writer.Write(p)

	if err != nil {
		f.logger.Errorf("failed to write: %v", err)
		msg = err.Error()

		return n, err
	}

	st, msg = statusSuccess, "Write successful"
	f.logger.Debug(msg)

	return n, nil
}

func (f *File) Close() error {
	bucketName := getBucketName(f.name)

	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "CLOSE",
		Location:  getLocation(bucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	if f.writer != nil {
		err := f.writer.Close()
		if err != nil {
			msg = err.Error()
			return err
		}

		st = statusSuccess

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

		st = statusSuccess

		msg = msgReaderClosed

		f.logger.Debug(msgReaderClosed)

		return nil
	}

	st = statusSuccess

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

func (f *File) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
	f.metrics.RecordHistogram(context.Background(), appFTPStats, float64(duration),
		"type", fl.Operation, "status", clean(fl.Status))
}
