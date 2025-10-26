package gcs

import (
	"context"
	"errors"
	"fmt"
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
	errNilGCSFileBody    = errors.New("gcs file body is nil")
	errOffesetOutOfRange = errors.New("offset out of range")
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

func (f *File) check(whence int, offset, length int64, msg *string) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekEnd:
		offset += length
	case io.SeekCurrent:
		offset += f.size
	default:
		return 0, errOffesetOutOfRange
	}

	if offset < 0 || offset > length {
		*msg = fmt.Sprintf("Offset %v out of bounds %v", offset, length)
		return 0, errOffesetOutOfRange
	}

	f.size = offset

	return f.size, nil
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	bucketName := getBucketName(f.name)

	var msg string

	status := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "SEEK",
		Location:  getLocation(bucketName),
		Status:    &status,
		Message:   &msg,
	}, time.Now())

	ctx := context.Background()

	attrs, err := f.conn.StatObject(ctx, f.name)
	if err != nil {
		msg = fmt.Sprintf("could not get object attrs: %v", err)
		f.logger.Errorf(msg)

		return 0, err
	}

	newPos, err := f.check(whence, offset, attrs.Size, &msg)
	if err != nil {
		f.logger.Errorf("Seek failed. Error: %v", err)
		return 0, err
	}

	if f.body != nil {
		_ = f.body.Close()
	}

	reader, err := f.conn.NewRangeReader(ctx, getObjectName(f.name), newPos, -1)
	if err != nil {
		f.logger.Errorf("failed to set new range reader: %v", err)

		return 0, err
	}

	f.body = reader
	f.size = newPos

	status = statusSuccess

	f.logger.Logf("Seek repositioned reader to offset %v for %q", newPos, f.name)

	return newPos, nil
}

func (f *File) ReadAt(p []byte, off int64) (int, error) {
	bucketName := getBucketName(f.name)

	var msg string

	status := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "READ_AT",
		Location:  getLocation(bucketName),
		Status:    &status,
		Message:   &msg,
	}, time.Now())

	ctx := context.Background()

	rdr, err := f.conn.NewRangeReader(ctx, getObjectName(f.name), off, int64(len(p)))
	if err != nil {
		f.logger.Errorf("failed to create range reader: %v", err)

		return 0, err
	}
	defer rdr.Close()

	n, err := io.ReadFull(rdr, p)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		msg = fmt.Sprintf("read failed: %v", err)
		f.logger.Errorf(msg)

		return n, err
	}

	status = statusSuccess

	f.logger.Debugf("ReadAt read %d bytes from offset %d for file %q", n, off, f.name)

	return n, nil
}

func (f *File) WriteAt(p []byte, off int64) (int, error) {
	bucketName := getBucketName(f.name)

	var msg string

	status := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "WRITE_AT",
		Location:  getLocation(bucketName),
		Status:    &status,
		Message:   &msg,
	}, time.Now())

	objectName := getObjectName(f.name)
	ctx := context.Background()
	rdr, err := f.conn.NewReader(ctx, objectName)

	var oldData []byte
	if err == nil {
		oldData, _ = io.ReadAll(rdr)
		_ = rdr.Close()
	}

	if int64(len(oldData)) < off {
		pad := make([]byte, off-int64(len(oldData)))
		oldData = append(oldData, pad...)
	}

	end := off + int64(len(p))
	if end > int64(len(oldData)) {
		newData := make([]byte, end)
		copy(newData, oldData)
		copy(newData[off:], p)
		oldData = newData
	} else {
		copy(oldData[off:end], p)
	}

	w := f.conn.NewWriter(ctx, objectName)
	if _, err := w.Write(oldData); err != nil {
		_ = w.Close()
		msg = fmt.Sprintf("failed to write updated data: %v", err)
		f.logger.Errorf(msg)

		return 0, err
	}

	if err := w.Close(); err != nil {
		msg = fmt.Sprintf("failed to close writer: %v", err)
		f.logger.Errorf(msg)

		return 0, err
	}

	status = statusSuccess

	f.logger.Debugf("WriteAt wrote %d bytes at offset %d in %q", len(p), off, f.name)

	return len(p), nil
}

func (f *File) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
	f.metrics.RecordHistogram(context.Background(), appFTPStats, float64(duration),
		"type", fl.Operation, "status", clean(fl.Status))
}
