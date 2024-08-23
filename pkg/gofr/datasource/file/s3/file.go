package s3

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	file_interface "gofr.dev/pkg/gofr/datasource/file"
	"io"
	"time"
)

type file struct {
	conn         *s3.Client
	name         string
	logger       Logger
	metrics      Metrics
	size         int64
	contentType  string
	body         io.ReadCloser
	lastModified time.Time
}

func (f *file) ReadAll() (file_interface.RowReader, error) {
	return nil, nil
}

func (f *file) Name() string {
	return f.name
}
func (f *file) Close() error {
	return nil
}

func (f *file) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (f *file) ReadAt(p []byte, offset int64) (n int, err error) {
	return 0, nil
}

func (f *file) Write(p []byte) (n int, err error) {
	return 0, nil
}

func (f *file) WriteAt(p []byte, offset int64) (n int, err error) {
	return 0, nil
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}
