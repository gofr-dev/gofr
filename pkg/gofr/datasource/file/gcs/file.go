package gcs

import (
	"errors"
	"io"
	"time"

	"cloud.google.com/go/storage"
)

type GCSFile struct {
	conn         gcsClient
	writer       *storage.Writer
	name         string
	offset       int64
	logger       Logger
	metrics      Metrics
	size         int64
	contentType  string
	body         io.ReadCloser
	lastModified time.Time
	isDir        bool
}

// ====== File interface methods ======

func (g *GCSFile) Read(p []byte) (int, error) {
	if g.body == nil {
		return 0, errors.New("GCS file body is nil")
	}
	return g.body.Read(p)
}
func (g *GCSFile) Write(p []byte) (int, error) {
	bucketName := getBucketName(g.name)

	var msg string

	st := statusErr

	defer g.sendOperationStats(&FileLog{
		Operation: "WRITE",
		Location:  getLocation(bucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	n, err := g.writer.Write(p)
	if err != nil {
		g.logger.Errorf("failed to write: %v", err)
		msg = err.Error()
		return n, err
	}
	st = statusSuccess
	msg = "Write successful"
	return n, nil

}

func (g *GCSFile) Close() error {
	bucketName := getBucketName(g.name)
	var msg string
	st := statusErr

	defer g.sendOperationStats(&FileLog{
		Operation: "CLOSE",
		Location:  getLocation(bucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	if g.writer != nil {
		err := g.writer.Close()
		if err != nil {
			msg = err.Error()
			return err
		}
		st = statusSuccess
		msg = "Writer closed successfully"
		return nil
	}

	if g.body != nil {
		err := g.body.Close()
		if err != nil {
			msg = err.Error()
			return err
		}
		st = statusSuccess
		msg = "Reader closed successfully"
		return nil
	}
	st = statusSuccess
	msg = "Writer closed successfully"
	return nil
}

func (g *GCSFile) Seek(offset int64, whence int) (int64, error) {
	// Not supported: Seek requires reopening with range.
	return 0, errors.New("Seek not supported on GCSFile")
}

func (g *GCSFile) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, errors.New("ReadAt not supported on GCSFile")
}

func (g *GCSFile) WriteAt(_ []byte, _ int64) (int, error) {
	return 0, errors.New("WriteAt not supported on GCSFile (read-only)")
}

func (g *GCSFile) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()

	fl.Duration = duration

	g.logger.Debug(fl)
}
