package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

// Error variables for common file operations.
var (
	// errFileNotOpenForReading is returned when attempting to read from a file that wasn't opened for reading.
	errFileNotOpenForReading = errors.New("file not open for reading")

	// errFileNotOpenForWriting is returned when attempting to write to a file that wasn't opened for writing.
	errFileNotOpenForWriting = errors.New("file not open for writing")

	// errWriteAtNotSupported is returned when WriteAt is called on cloud storage.
	errWriteAtNotSupported = errors.New("WriteAt not supported for cloud storage")

	// errWriterNil is returned when NewWriter returns nil.
	errWriterNil = errors.New("NewWriter returned nil")
)

// File permission constants.
const (
	// DefaultFileMode is the standard file permission (0644 = rw-r--r--).
	DefaultFileMode os.FileMode = 0644

	// DefaultDirMode is the standard directory permission (0755 = rwxr-xr-x).
	DefaultDirMode os.FileMode = 0755
)

// CommonFile implements FileInfo for all providers, eliminating redundant metadata getters.
// Providers instantiate this struct when returning file metadata.
type CommonFile struct {
	provider StorageProvider

	name         string
	size         int64
	contentType  string
	lastModified time.Time
	isDir        bool

	body       io.ReadCloser
	writer     io.WriteCloser
	currentPos int64

	logger   datasource.Logger
	metrics  StorageMetrics
	location string
}

// NewCommonFile creates a new file instance for reading.
func NewCommonFile(
	provider StorageProvider,
	name string,
	info *ObjectInfo,
	reader io.ReadCloser,
	logger datasource.Logger,
	metrics StorageMetrics,
	location string,
) *CommonFile {
	return &CommonFile{
		provider:     provider,
		name:         name,
		size:         info.Size,
		contentType:  info.ContentType,
		lastModified: info.LastModified,
		isDir:        info.IsDir,
		body:         reader,
		currentPos:   0,
		logger:       logger,
		metrics:      metrics,
		location:     location,
	}
}

// NewCommonFileWriter creates a new file instance for writing.
func NewCommonFileWriter(
	provider StorageProvider,
	name string,
	writer io.WriteCloser,
	logger datasource.Logger,
	metrics StorageMetrics,
	location string,
) *CommonFile {
	return &CommonFile{
		provider: provider,
		name:     name,
		writer:   writer,
		logger:   logger,
		metrics:  metrics,
		location: location,
	}
}

// Read implements io.Reader.
func (f *CommonFile) Read(p []byte) (int, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer f.observe(OpRead, startTime, &st, &msg)

	if f.body == nil {
		return 0, errFileNotOpenForReading
	}

	n, err := f.body.Read(p)
	f.currentPos += int64(n)

	if err != nil && err != io.EOF {
		msg = fmt.Sprintf("read failed: %v", err)
		return n, err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Read %d bytes from %q", n, f.name)

	return n, err
}

// ReadAt implements io.ReaderAt.
func (f *CommonFile) ReadAt(p []byte, off int64) (int, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer f.observe(OpReadAt, startTime, &st, &msg)

	if off < 0 || off >= f.size {
		msg = fmt.Sprintf("offset %d out of range [0, %d]", off, f.size)
		return 0, ErrOutOfRange
	}

	ctx := context.Background()

	// Create range reader for this specific read
	reader, err := f.provider.NewRangeReader(ctx, f.name, off, int64(len(p)))
	if err != nil {
		msg = fmt.Sprintf("failed to create range reader: %v", err)
		return 0, err
	}
	defer reader.Close()

	n, err := io.ReadFull(reader, p)
	if err != nil && errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		msg = fmt.Sprintf("ReadAt failed: %v", err)
		return n, err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("ReadAt %d bytes from %q at offset %d", n, f.name, off)

	return n, nil
}

// Write implements io.Writer.
func (f *CommonFile) Write(p []byte) (int, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer f.observe(OpWrite, startTime, &st, &msg)

	if f.writer == nil {
		return 0, errFileNotOpenForWriting
	}

	n, err := f.writer.Write(p)
	if err != nil {
		msg = fmt.Sprintf("write failed: %v", err)

		return n, err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Wrote %d bytes to %q", n, f.name)

	return n, nil
}

// WriteAt writes len(p) bytes from p to the file at offset off (supports local filesystem only).
func (f *CommonFile) WriteAt(p []byte, off int64) (int, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer f.observe(OpWriteAt, startTime, &st, &msg)

	if f.writer == nil {
		return 0, errFileNotOpenForWriting
	}

	// Check if writer is an *os.File (local filesystem)
	if osFile, ok := f.writer.(*os.File); ok {
		n, err := osFile.WriteAt(p, off)
		if err != nil {
			msg = fmt.Sprintf("failed to write at offset %d: %v", off, err)

			return n, err
		}

		st = StatusSuccess
		msg = fmt.Sprintf("WriteAt %d bytes to %q at offset %d", n, f.name, off)

		return n, nil
	}

	// Cloud storage doesn't support WriteAt
	return 0, errWriteAtNotSupported
}

// Seek implements io.Seeker.
func (f *CommonFile) Seek(offset int64, whence int) (int64, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer f.observe(OpSeek, startTime, &st, &msg)

	// Calculate new position
	newPos, err := ValidateSeekOffset(whence, offset, f.currentPos, f.size)
	if err != nil {
		msg = fmt.Sprintf("invalid seek offset: %v", err)
		return 0, err
	}

	ctx := context.Background()

	// Close old reader
	if f.body != nil {
		if closeErr := f.body.Close(); closeErr != nil {
			msg = fmt.Sprintf("failed to close old reader: %v", closeErr)
		}
	}

	// Create new range reader from new position
	reader, err := f.provider.NewRangeReader(ctx, f.name, newPos, -1)
	if err != nil {
		msg = fmt.Sprintf("failed to create range reader: %v", err)
		return 0, err
	}

	f.body = reader
	f.currentPos = newPos

	st = StatusSuccess
	msg = fmt.Sprintf("Sought to position %d in %q", newPos, f.name)

	return newPos, nil
}

// Close implements io.Closer.
func (f *CommonFile) Close() error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer f.observe(OpClose, startTime, &st, &msg)

	var errs []error

	// Close reader
	if f.body != nil {
		if err := f.body.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close reader: %w", err))
		}

		f.body = nil
	}

	// Close writer
	if f.writer != nil {
		if err := f.writer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close writer: %w", err))
		}

		f.writer = nil
	}

	if len(errs) > 0 {
		msg = fmt.Sprintf("close failed: %v", errs)

		return errors.Join(errs...)
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Closed %q successfully", f.name)

	return nil
}

// Name returns the base name of the file.
func (f *CommonFile) Name() string {
	return f.name
}

// Size returns the file size in bytes. Returns 0 for directories.
func (f *CommonFile) Size() int64 {
	return f.size
}

// ModTime returns the last modification time.
func (f *CommonFile) ModTime() time.Time {
	return f.lastModified
}

// IsDir returns true if the object is a directory.
// Checks both explicit isDir flag and content type for compatibility.
func (f *CommonFile) IsDir() bool {
	return f.isDir || f.contentType == "application/x-directory"
}

// Mode returns the file mode bits.
func (f *CommonFile) Mode() os.FileMode {
	if f.isDir {
		return DefaultDirMode
	}

	return DefaultFileMode
}

// ReadAll returns a reader for JSON/CSV files.
func (f *CommonFile) ReadAll() (RowReader, error) {
	if f.body == nil {
		return nil, errFileNotOpenForReading
	}

	if strings.HasSuffix(f.name, ".json") {
		return NewJSONReader(f.body, f.logger)
	}

	// Default to text/CSV reader
	return NewTextReader(f.body, f.logger), nil
}

// Sys returns nil (no underlying system-specific data for cloud storage).
func (*CommonFile) Sys() any {
	return nil
}

// observe records metrics and logs for file operations.
func (f *CommonFile) observe(operation string, startTime time.Time, status, message *string) {
	ObserveOperation(&OperationObservability{
		Context:   context.Background(),
		Logger:    f.logger,
		Metrics:   f.metrics,
		Operation: operation,
		Location:  f.location,
		Provider:  "GoFr-File",
		StartTime: startTime,
		Status:    status,
		Message:   message,
	})
}
