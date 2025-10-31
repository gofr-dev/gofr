package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/datasource/file"
)

type gcsWriter interface {
	Write(p []byte) (int, error)
	Close() error
}
type File struct {
	conn         file.StorageProvider
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
	errNilGCSFileBody = errors.New("gcs file body is nil")
)

// ====== File interface methods ======

func (f *File) Read(p []byte) (int, error) {
	var msg string

	st := file.StatusError
	startTime := time.Now()

	defer f.observe(file.OpRead, startTime, &st, &msg)

	if f.body == nil {
		msg = "GCS file body is nil"

		f.logger.Error(msg)

		return 0, errNilGCSFileBody
	}

	n, err := f.body.Read(p)

	if err != nil && !errors.Is(err, io.EOF) {
		msg = fmt.Sprintf("Read failed: %v", err)

		f.logger.Errorf(msg)

		return n, err
	}

	st = file.StatusSuccess
	msg = fmt.Sprintf("Read %d bytes", n)

	return n, err
}

func (f *File) Write(p []byte) (int, error) {
	var msg string

	st := file.StatusError

	startTime := time.Now()

	defer f.observe(file.OpWrite, startTime, &st, &msg)

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
	var msg string

	st := file.StatusError

	startTime := time.Now()

	defer f.observe(file.OpClose, startTime, &st, &msg)

	if f.writer != nil {
		err := f.writer.Close()
		if err != nil {
			msg = err.Error()
			return err
		}

		st = file.StatusSuccess

		msg = file.MsgWriterClosed

		return nil
	}

	if f.body != nil {
		err := f.body.Close()
		if err != nil {
			msg = err.Error()
			return err
		}

		st = file.StatusSuccess

		msg = file.MsgReaderClosed

		f.logger.Debug(file.MsgReaderClosed)

		return nil
	}

	st = file.StatusSuccess

	msg = file.MsgWriterClosed

	return nil
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	var msg string

	status := file.StatusError

	defer f.observe(file.OpSeek, time.Now(), &status, &msg)

	ctx := context.Background()

	attrs, err := f.conn.StatObject(ctx, f.name)
	if err != nil {
		msg = fmt.Sprintf("could not get object attrs: %v", err)
		f.logger.Errorf(msg)

		return 0, err
	}

	newPos, err := file.ValidateSeekOffset(whence, offset, f.size, attrs.Size)
	if err != nil {
		f.logger.Errorf("Seek failed. Error: %v", err)
		return 0, err
	}

	if f.body != nil {
		_ = f.body.Close()
	}

	reader, err := f.conn.NewRangeReader(ctx, file.GetObjectName(f.name), newPos, -1)
	if err != nil {
		f.logger.Errorf("failed to set new range reader: %v", err)

		return 0, err
	}

	f.body = reader
	f.size = newPos

	status = file.StatusSuccess

	f.logger.Infof("Seek repositioned reader to offset %v for %q", newPos, f.name)

	return newPos, nil
}

func (f *File) ReadAt(p []byte, off int64) (int, error) {
	var msg string

	status := file.StatusError

	defer f.observe(file.OpReadAt, time.Now(), &status, &msg)

	ctx := context.Background()

	rdr, err := f.conn.NewRangeReader(ctx, file.GetObjectName(f.name), off, int64(len(p)))
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

	status = file.StatusSuccess

	f.logger.Debugf("ReadAt read %d bytes from offset %d for file %q", n, off, f.name)

	return n, nil
}

func (f *File) WriteAt(p []byte, off int64) (int, error) {
	var msg string

	status := file.StatusError

	defer f.observe(file.OpWriteAt, time.Now(), &status, &msg)

	objectName := file.GetObjectName(f.name)
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

	status = file.StatusSuccess

	f.logger.Debugf("WriteAt wrote %d bytes at offset %d in %q", len(p), off, f.name)

	return len(p), nil
}

func (f *File) ReadAll() (file.RowReader, error) {
	var msg string

	st := file.StatusError
	startTime := time.Now()

	defer f.observe(file.OpReadAll, startTime, &st, &msg)

	if f.body == nil {
		msg = "file body is nil"

		f.logger.Error(msg)

		return nil, errNilGCSFileBody
	}

	if strings.HasSuffix(f.name, ".json") {
		reader, err := file.NewJSONReader(f.body)
		if err != nil {
			msg = fmt.Sprintf("failed to create JSON reader: %v", err)
			f.logger.Errorf(msg)

			return nil, err
		}

		st = file.StatusSuccess
		msg = "JSON reader created successfully"

		return reader, nil
	}

	// Default to text/CSV reader
	reader := file.NewTextReader(f.body)
	st = file.StatusSuccess
	msg = "Text reader created successfully"

	return reader, nil
}

func getLocation(bucket string) string {
	return filepath.Join(string(filepath.Separator), bucket)
}

func (f *File) Name() string {
	return filepath.Base(f.name)
}

func (f *File) Size() int64 {
	return f.size
}

func (f *File) ModTime() time.Time {
	return f.lastModified
}

func (f *File) IsDir() bool {
	return f.isDir || f.contentType == "application/x-directory"
}

func (f *File) Mode() os.FileMode {
	if f.IsDir() {
		return dirPermissions | os.ModeDir
	}

	return filePermissions
}

// Sys returns nil (no underlying system-specific data).
func (*File) Sys() any {
	return nil
}

// observe is a helper method to reduce boilerplate for file operation observability.
func (f *File) observe(operation string, startTime time.Time, status, message *string) {
	bucketName := file.GetBucketName(f.name)

	file.ObserveOperation(&file.OperationObservability{
		Context:   context.Background(),
		Logger:    f.logger,
		Metrics:   f.metrics,
		Operation: operation,
		Location:  getLocation(bucketName),
		Provider:  "GCS",
		StartTime: startTime,
		Status:    status,
		Message:   message,
	})
}
