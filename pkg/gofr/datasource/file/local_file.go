package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

// localFile is a File backed by an *os.File. It gives files from the local store the
// read, write, seek and close behavior of the os package, and observes each operation
// the same way CommonFile does.
type localFile struct {
	file *os.File

	// mu guards info: the file's metadata from when it was opened, refreshed when it is
	// closed. It is returned once the file can no longer be stat'ed.
	mu   sync.Mutex
	info os.FileInfo

	logger   datasource.Logger
	metrics  StorageMetrics
	location string
}

func newLocalFile(f *os.File, logger datasource.Logger, metrics StorageMetrics, location string) *localFile {
	info, _ := f.Stat()

	return &localFile{file: f, info: info, logger: logger, metrics: metrics, location: location}
}

// Read implements io.Reader.
func (f *localFile) Read(p []byte) (int, error) {
	startTime := time.Now()

	n, err := f.file.Read(p)
	f.observe(OpRead, startTime, err, fmt.Sprintf("Read %d bytes from %q", n, f.Name()))

	return n, err
}

// ReadAt implements io.ReaderAt. A read that reaches the end of the file returns io.EOF
// with the bytes read. As on the other stores, a negative offset or one past the end of
// the file returns ErrOutOfRange.
func (f *localFile) ReadAt(p []byte, off int64) (int, error) {
	startTime := time.Now()

	n, err := f.readAt(p, off)
	f.observe(OpReadAt, startTime, err, fmt.Sprintf("ReadAt %d bytes from %q at offset %d", n, f.Name(), off))

	return n, err
}

func (f *localFile) readAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, ErrOutOfRange
	}

	n, err := f.file.ReadAt(p, off)

	// The os package returns io.EOF for any offset at or past the end. Only a read that
	// starts at the end is a successful read of nothing.
	if n == 0 && errors.Is(err, io.EOF) {
		if info, statErr := f.file.Stat(); statErr == nil && off > info.Size() {
			return 0, ErrOutOfRange
		}
	}

	return n, err
}

// Write implements io.Writer.
func (f *localFile) Write(p []byte) (int, error) {
	startTime := time.Now()

	n, err := f.file.Write(p)
	f.observe(OpWrite, startTime, err, fmt.Sprintf("Wrote %d bytes to %q", n, f.Name()))

	return n, err
}

// WriteAt implements io.WriterAt.
func (f *localFile) WriteAt(p []byte, off int64) (int, error) {
	startTime := time.Now()

	n, err := f.file.WriteAt(p, off)
	f.observe(OpWriteAt, startTime, err, fmt.Sprintf("WriteAt %d bytes to %q at offset %d", n, f.Name(), off))

	return n, err
}

// Seek implements io.Seeker. It moves the offset used by both Read and Write. As in the
// os package, the offset can move past the end of the file, where a later Write extends
// it. As on the other stores, an unknown whence or a negative offset returns an error
// that matches ErrOutOfRange.
func (f *localFile) Seek(offset int64, whence int) (int64, error) {
	startTime := time.Now()

	pos, err := f.file.Seek(offset, whence)
	if err != nil && f.invalidSeek(offset, whence) {
		err = fmt.Errorf("%w: %w", ErrOutOfRange, err)
	}

	f.observe(OpSeek, startTime, err, fmt.Sprintf("Sought to position %d in %q", pos, f.Name()))

	return pos, err
}

// invalidSeek reports whether a failed seek had an unknown whence or would have moved
// the offset before the start of the file. A failed seek leaves the offset unchanged.
func (f *localFile) invalidSeek(offset int64, whence int) bool {
	var base int64

	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		cur, err := f.file.Seek(0, io.SeekCurrent)
		if err != nil {
			return false
		}

		base = cur
	case io.SeekEnd:
		info, err := f.file.Stat()
		if err != nil {
			return false
		}

		base = info.Size()
	default:
		return true
	}

	return base+offset < 0
}

// Close implements io.Closer.
func (f *localFile) Close() error {
	startTime := time.Now()

	// Keep the final metadata for the getters, which cannot stat a closed file.
	if info, err := f.file.Stat(); err == nil {
		f.mu.Lock()
		f.info = info
		f.mu.Unlock()
	}

	err := f.file.Close()
	if errors.Is(err, os.ErrClosed) {
		// Closing a file again is not an error, as on the other stores.
		err = nil
	}

	f.observe(OpClose, startTime, err, fmt.Sprintf("Closed %q successfully", f.Name()))

	return err
}

// ReadAll returns a reader for JSON/CSV files.
func (f *localFile) ReadAll() (RowReader, error) {
	if strings.HasSuffix(f.Name(), ".json") {
		return NewJSONReader(f.file, f.logger)
	}

	return NewTextReader(f.file, f.logger), nil
}

// Name returns the name the file was opened with.
func (f *localFile) Name() string {
	return f.file.Name()
}

// Size returns the current size of the file in bytes.
func (f *localFile) Size() int64 {
	if info := f.stat(); info != nil {
		return info.Size()
	}

	return 0
}

// ModTime returns the last modification time.
func (f *localFile) ModTime() time.Time {
	if info := f.stat(); info != nil {
		return info.ModTime()
	}

	return time.Time{}
}

// IsDir reports whether the file is a directory.
func (f *localFile) IsDir() bool {
	if info := f.stat(); info != nil {
		return info.IsDir()
	}

	return false
}

// Mode returns the file mode bits.
func (f *localFile) Mode() os.FileMode {
	if info := f.stat(); info != nil {
		return info.Mode()
	}

	return 0
}

// Sys returns the underlying data source of the file's metadata.
func (f *localFile) Sys() any {
	if info := f.stat(); info != nil {
		return info.Sys()
	}

	return nil
}

// stat returns the file's current metadata, or the metadata saved when it was opened
// or closed if the file can no longer be stat'ed.
func (f *localFile) stat() os.FileInfo {
	if info, err := f.file.Stat(); err == nil {
		return info
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	return f.info
}

// observe records metrics and logs for an operation. io.EOF is how reads report the
// end of the file, so it is not counted as a failure.
func (f *localFile) observe(operation string, startTime time.Time, err error, successMsg string) {
	st, msg := StatusSuccess, successMsg

	if err != nil && !errors.Is(err, io.EOF) {
		st, msg = StatusError, fmt.Sprintf("%s on %q failed: %v", operation, f.Name(), err)
	}

	ObserveOperation(&OperationObservability{
		Context:   context.Background(),
		Logger:    f.logger,
		Metrics:   f.metrics,
		Operation: operation,
		Location:  f.location,
		Provider:  "FILE",
		StartTime: startTime,
		Status:    &st,
		Message:   &msg,
	})
}
