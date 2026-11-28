package file

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

// localFile is a File backed by an *os.File. Read, ReadAt, Write, WriteAt, Seek and
// Close go to the open file and are observed like CommonFile operations. The embedded
// CommonFile holds the metadata that the getters return and the reader that ReadAll
// uses, as for the other stores.
type localFile struct {
	CommonFile

	file *os.File

	// readOnly is set for a file opened only for reading, whose offset stays within the file.
	readOnly bool
}

// newLocalFile wraps f, opened with the access mode in flag. A file opened for reading
// takes its metadata from a stat now and serves ReadAll, as before; a file opened only
// for writing starts with empty metadata.
func newLocalFile(f *os.File, flag int, logger datasource.Logger, metrics StorageMetrics, location string) *localFile {
	lf := &localFile{
		CommonFile: CommonFile{name: f.Name(), logger: logger, metrics: metrics, location: location},
		file:       f,
		readOnly:   flag&(os.O_WRONLY|os.O_RDWR) == 0,
	}

	if flag&os.O_WRONLY == 0 {
		lf.body = f

		if info, err := f.Stat(); err == nil {
			lf.size, lf.lastModified, lf.isDir = info.Size(), info.ModTime(), info.IsDir()
		}
	}

	return lf
}

// Read implements io.Reader.
func (f *localFile) Read(p []byte) (int, error) {
	startTime := time.Now()

	n, err := f.file.Read(p)
	f.record(OpRead, startTime, err, fmt.Sprintf("Read %d bytes from %q", n, f.name))

	return n, err
}

// ReadAt implements io.ReaderAt with the bounds of the other stores: a negative offset,
// or one at or past the end of the file, returns ErrOutOfRange, and a read cut short by
// the end of the file returns the bytes read without an error.
func (f *localFile) ReadAt(p []byte, off int64) (int, error) {
	startTime := time.Now()

	n, err := f.readAt(p, off)
	f.record(OpReadAt, startTime, err, fmt.Sprintf("ReadAt %d bytes from %q at offset %d", n, f.name, off))

	return n, err
}

func (f *localFile) readAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, ErrOutOfRange
	}

	n, err := f.file.ReadAt(p, off)
	if errors.Is(err, io.EOF) {
		if n == 0 {
			return 0, ErrOutOfRange
		}

		return n, nil
	}

	return n, err
}

// Write implements io.Writer.
func (f *localFile) Write(p []byte) (int, error) {
	startTime := time.Now()

	n, err := f.file.Write(p)
	f.record(OpWrite, startTime, err, fmt.Sprintf("Wrote %d bytes to %q", n, f.name))

	return n, err
}

// WriteAt implements io.WriterAt.
func (f *localFile) WriteAt(p []byte, off int64) (int, error) {
	startTime := time.Now()

	n, err := f.file.WriteAt(p, off)
	f.record(OpWriteAt, startTime, err, fmt.Sprintf("WriteAt %d bytes to %q at offset %d", n, f.name, off))

	return n, err
}

// Seek implements io.Seeker, moving the offset that Read and Write use. As on the other
// stores, an unknown whence or a negative offset returns an error matching ErrOutOfRange,
// and so does an offset past the end of a file opened only for reading. A file opened for
// writing can move past the end, where a later Write extends it.
func (f *localFile) Seek(offset int64, whence int) (int64, error) {
	startTime := time.Now()

	pos, err := f.seek(offset, whence)
	f.record(OpSeek, startTime, err, fmt.Sprintf("Sought to position %d in %q", pos, f.name))

	return pos, err
}

func (f *localFile) seek(offset int64, whence int) (int64, error) {
	cur, err := f.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	info, err := f.file.Stat()
	if err != nil {
		return 0, err
	}

	var base int64

	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = cur
	case io.SeekEnd:
		base = info.Size()
	default:
		return 0, ErrOutOfRange
	}

	pos := base + offset
	if pos < 0 || f.readOnly && pos > info.Size() {
		return 0, fmt.Errorf("%w: offset %d out of bounds [0, %d]", ErrOutOfRange, pos, info.Size())
	}

	return f.file.Seek(pos, io.SeekStart)
}

// Close implements io.Closer. Closing a file again returns nil, as on the other stores.
func (f *localFile) Close() error {
	startTime := time.Now()

	err := f.file.Close()
	if errors.Is(err, os.ErrClosed) {
		err = nil
	}

	f.record(OpClose, startTime, err, fmt.Sprintf("Closed %q successfully", f.name))

	return err
}

// record observes an operation on the file through the embedded CommonFile.
func (f *localFile) record(operation string, startTime time.Time, err error, successMsg string) {
	st, msg := outcome(operation, f.name, err, successMsg)
	f.observe(operation, startTime, &st, &msg)
}
