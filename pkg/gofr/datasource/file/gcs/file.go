package gcs

import (
	"errors"
	"io"
	"time"

	"cloud.google.com/go/storage"
)

// GCSFile represents a file in an GCS bucket.
//
//nolint:revive // gcs.GCSFile is repetitive. A better name could have been chosen, but it's too late as it's already exported.
type GCSFile struct {
	conn         gcsClient
	writer       *storage.Writer
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
	ErrNilGCSFileBody      = errors.New("GCS file body is nil")
	ErrSeekNotSupported    = errors.New("seek not supported on GCSFile")
	ErrReadAtNotSupported  = errors.New("readAt not supported on GCSFile")
	ErrWriteAtNotSupported = errors.New("writeAt not supported on GCSFile (read-only)")
)

const (
	msgWriterClosed = "Writer closed successfully"
	msgReaderClosed = "Reader closed successfully"
)

// ====== File interface methods ======

func (f *GCSFile) Read(p []byte) (int, error) {
	if f.body == nil {
		return 0, ErrNilGCSFileBody
	}

	return f.body.Read(p)
}
func (f *GCSFile) Write(p []byte) (int, error) {
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

	st = statusSuccess
	msg = "Write successful"
	f.logger.Logf(msg)

	return n, nil
}

func (f *GCSFile) Close() error {
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

		f.logger.Logf(msg)

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

		f.logger.Logf(msgReaderClosed)

		return nil
	}

	st = statusSuccess

	msg = msgWriterClosed

	return nil
}

func (*GCSFile) Seek(_ int64, _ int) (int64, error) {
	// Not supported: Seek requires reopening with range.
	return 0, ErrSeekNotSupported
}

func (*GCSFile) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, ErrReadAtNotSupported
}

func (*GCSFile) WriteAt(_ []byte, _ int64) (int, error) {
	return 0, ErrWriteAtNotSupported
}

func (f *GCSFile) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
