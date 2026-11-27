package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

const (
	dirPerm = 0755

	// localLocation is the Location reported for the local filesystem.
	localLocation = "local"
)

// localProvider implements StorageProvider for local filesystem.
type localProvider struct{}

// localFileSystem is the local filesystem store. It exposes the no-arg Connect()
// required by FileSystemProvider, uses the os package for file operations, and
// delegates the remaining operations to CommonFileSystem.
type localFileSystem struct {
	*CommonFileSystem
}

// NewLocalFileSystem creates a FileSystem for local filesystem operations.
// It returns a FileSystemProvider so callers can treat it uniformly with other providers.
func NewLocalFileSystem(logger datasource.Logger) FileSystemProvider {
	provider := &localProvider{}

	cfs := &CommonFileSystem{
		Provider: provider,
		Location: localLocation,
		Logger:   logger,
		Metrics:  nil,
	}

	return &localFileSystem{CommonFileSystem: cfs}
}

// Connect implements the no-arg Connect for FileSystemProvider compatibility.
func (l *localFileSystem) Connect() {
	// Local filesystem connect is a no-op but we call CommonFileSystem.Connect
	// with a background context to set up any bookkeeping.
	_ = l.CommonFileSystem.Connect(context.Background())
}

// The methods below replace CommonFileSystem's object-store implementations, which
// list, copy and delete objects by name. On a real filesystem those lose data: names
// resolve against the working directory, a copy onto the same file truncates it, and
// every write-only open truncates. The local store calls the os package directly instead.

// Create creates or truncates the named file like os.Create, creating missing parent
// directories first. As before, a failure is reported by the returned File: its Write
// and Close return the error, and Create itself returns nil.
func (l *localFileSystem) Create(name string) (File, error) {
	startTime := time.Now()

	f, err := createFile(name)
	l.observe(OpCreate, name, startTime, err, fmt.Sprintf("Created %q for writing", name))

	if err != nil {
		return l.failedFile(name, err), nil
	}

	return l.newFile(f, os.O_WRONLY), nil
}

// Open opens the named file for reading like os.Open. A missing file returns
// ErrFileNotFound.
func (l *localFileSystem) Open(name string) (File, error) {
	startTime := time.Now()

	f, err := os.Open(name)
	l.observe(OpOpen, name, startTime, err, fmt.Sprintf("Opened %q for reading", name))

	if err != nil {
		return nil, notFound(err)
	}

	return l.newFile(f, os.O_RDONLY), nil
}

// OpenFile opens the named file with the given flags and permissions like os.OpenFile.
// With O_CREATE, missing parent directories are created first. A missing file returns
// ErrFileNotFound.
//
// If a file opened for writing cannot be opened, OpenFile returns the error together
// with a non-nil File whose Write and Close return that error too. The non-nil File is
// deliberate: as with Create, code that ignores the error gets it from Write and Close
// instead of a nil pointer panic. Unlike Create, the error is also returned, so that an
// O_CREATE|O_EXCL open, as used for a lock file, reports a file that already exists.
// A failed read-only open returns a nil File.
func (l *localFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	startTime := time.Now()

	f, err := openFile(name, flag, perm)
	l.observe(OpOpenFile, name, startTime, err, fmt.Sprintf("Opened %q with flags %d", name, flag))

	if err != nil {
		return l.failedOpen(name, flag, notFound(err))
	}

	return l.newFile(f, flag), nil
}

// RemoveAll removes dirPath and everything under it like os.RemoveAll, and returns nil
// if dirPath does not exist. Like os.RemoveAll, it leaves the working directory as it
// is, even when it removes it.
func (l *localFileSystem) RemoveAll(dirPath string) error {
	startTime := time.Now()

	err := os.RemoveAll(dirPath)
	l.observe(OpRemoveAll, dirPath, startTime, err,
		fmt.Sprintf("Directory %q and all contents deleted successfully", dirPath))

	return err
}

// Rename moves oldname to newname like os.Rename, creating newname's missing parent
// directories first. If the two paths are on different filesystems, a regular file is
// copied and the original removed.
func (l *localFileSystem) Rename(oldname, newname string) error {
	startTime := time.Now()

	err := renameFile(oldname, newname)
	l.observe(OpRename, oldname, startTime, err, fmt.Sprintf("Renamed %q to %q successfully", oldname, newname))

	return err
}

// observe records an operation of the local store like CommonFileSystem.Observe.
func (l *localFileSystem) observe(operation, name string, startTime time.Time, err error, successMsg string) {
	st, msg := outcome(operation, name, err, successMsg)
	l.Observe(operation, startTime, &st, &msg)
}

// outcome returns the status and message to observe for an operation on name that
// returned err. io.EOF is how a read reports the end of a file, not a failure.
func outcome(operation, name string, err error, successMsg string) (status, message string) {
	if err != nil && !errors.Is(err, io.EOF) {
		return StatusError, fmt.Sprintf("%s on %q failed: %v", operation, name, err)
	}

	return StatusSuccess, successMsg
}

func (l *localFileSystem) newFile(f *os.File, flag int) File {
	return newLocalFile(f, flag, l.Logger, l.Metrics, l.Location)
}

// failedFile returns the File that reports a failed create, as earlier releases did:
// its Write and Close return err.
func (l *localFileSystem) failedFile(name string, err error) File {
	return NewCommonFileWriter(l.Provider, name, &failWriter{err: err}, l.Logger, l.Metrics, l.Location)
}

// failedOpen returns the result of an OpenFile call that failed with err: a failedFile
// when the file was opened for writing, so that code ignoring the error does not
// dereference a nil File, and nil otherwise.
func (l *localFileSystem) failedOpen(name string, flag int, err error) (File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		return l.failedFile(name, err), err
	}

	return nil, err
}

// notFound returns ErrFileNotFound itself for an error that reports a missing file, so
// that callers comparing with == keep matching it, and err otherwise.
func notFound(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return ErrFileNotFound
	}

	return err
}

// createFile creates or truncates name like os.Create, creating missing parent
// directories first.
func createFile(name string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(name), DefaultDirMode); err != nil {
		return nil, err
	}

	return os.Create(name)
}

// openFile opens name like os.OpenFile, first creating missing parent directories when
// flag has O_CREATE.
func openFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if flag&os.O_CREATE != 0 {
		if err := os.MkdirAll(filepath.Dir(name), DefaultDirMode); err != nil {
			return nil, err
		}
	}

	return os.OpenFile(name, flag, perm)
}

// renameFile moves oldname to newname like os.Rename, creating newname's missing parent
// directories first, and copies a regular file across filesystems.
func renameFile(oldname, newname string) error {
	// Check the source first so that a failed rename does not leave new directories behind.
	if _, err := os.Lstat(oldname); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(newname), DefaultDirMode); err != nil {
		return err
	}

	err := os.Rename(oldname, newname)
	if isCrossDeviceError(err) {
		return moveAcrossDevices(oldname, newname, err)
	}

	return err
}

// moveAcrossDevices moves a regular file to another filesystem, where os.Rename cannot:
// it replaces dst with a copy of src and then removes src. For anything other than a
// regular file (a directory, symlink or FIFO) it returns renameErr unchanged.
func moveAcrossDevices(src, dst string, renameErr error) error {
	// Lstat, not Open: opening would follow a symlink, or block on a FIFO.
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return renameErr
	}

	// Both paths can name the same file through different mounts, such as a bind mount
	// or overlapping container volumes. Renaming a file onto itself does nothing, while
	// copying it over itself and removing src would lose it.
	if dstInfo, err := os.Lstat(dst); err == nil && os.SameFile(info, dstInfo) {
		return nil
	}

	if err := replaceWithCopy(src, info, dst); err != nil {
		return err
	}

	return os.Remove(src)
}

// replaceWithCopy copies src into a temporary file next to dst, with info's permissions
// and modification time, and renames it over dst so that dst is never seen half-written.
// src is closed before it returns, because Windows cannot remove an open file.
func replaceWithCopy(src string, info os.FileInfo, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() { _ = in.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}

	if err := fillTempFile(tmp, in, info); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}

	if err := os.Rename(tmp.Name(), dst); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}

	return nil
}

// fillTempFile writes in to tmp, applies info's permissions and modification time, and
// syncs and closes tmp. The sync matters because src is removed afterwards.
func fillTempFile(tmp *os.File, in io.Reader, info os.FileInfo) error {
	defer func() { _ = tmp.Close() }()

	if _, err := io.Copy(tmp, in); err != nil {
		return err
	}

	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return err
	}

	if err := tmp.Sync(); err != nil {
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Chtimes(tmp.Name(), info.ModTime(), info.ModTime())
}

func (*localProvider) Connect(_ context.Context) error {
	return nil
}

func (*localProvider) Health(_ context.Context) error {
	return nil // Local FS is always healthy
}

func (*localProvider) Close() error {
	return nil // Nothing to close
}

func (*localProvider) NewReader(_ context.Context, name string) (io.ReadCloser, error) {
	return os.Open(name)
}

func (*localProvider) NewRangeReader(_ context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}

	if length > 0 {
		return &limitedReadCloser{f, length}, nil
	}

	return f, nil
}

func (*localProvider) NewWriter(_ context.Context, name string) io.WriteCloser {
	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(name), dirPerm); err != nil {
		return &failWriter{err: err}
	}

	f, err := os.Create(name)
	if err != nil {
		return &failWriter{err: err}
	}

	return f
}

func (*localProvider) StatObject(_ context.Context, name string) (*ObjectInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Name:         info.Name(),
		Size:         info.Size(),
		ContentType:  "", // Local FS doesn't store content type
		LastModified: info.ModTime(),
		IsDir:        info.IsDir(),
	}, nil
}

func (*localProvider) DeleteObject(_ context.Context, name string) error {
	return os.Remove(name)
}

func (*localProvider) CopyObject(_ context.Context, src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() { _ = srcFile.Close() }()

	if mkdirErr := os.MkdirAll(filepath.Dir(dst), DefaultDirMode); mkdirErr != nil {
		return mkdirErr
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)

	return err
}

func (*localProvider) ListObjects(_ context.Context, prefix string) ([]string, error) {
	entries, err := os.ReadDir(prefix)
	if err != nil {
		return nil, err
	}

	var objects []string

	for _, entry := range entries {
		if !entry.IsDir() {
			objects = append(objects, entry.Name())
		}
	}

	return objects, nil
}

func (*localProvider) ListDir(_ context.Context, prefix string) ([]ObjectInfo, []string, error) {
	entries, err := os.ReadDir(prefix)
	if err != nil {
		return nil, nil, err
	}

	var (
		objects []ObjectInfo
		dirs    []string
	)

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, entry.Name()+"/")
		} else {
			objects = append(objects, ObjectInfo{
				Name:         entry.Name(),
				Size:         info.Size(),
				LastModified: info.ModTime(),
				IsDir:        false,
			})
		}
	}

	return objects, dirs, nil
}

// Deprecated: New is a backward-compatible wrapper for NewLocalFileSystem and will be
// removed in a future major version. Use NewLocalFileSystem instead.
func New(logger datasource.Logger) FileSystem {
	return NewLocalFileSystem(logger)
}

// ============= Helper Types =============

// limitedReadCloser wraps a ReadCloser with a byte limit.
type limitedReadCloser struct {
	rc        io.ReadCloser
	remaining int64
}

func (l *limitedReadCloser) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		return 0, io.EOF
	}

	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}

	n, err := l.rc.Read(p)
	l.remaining -= int64(n)

	return n, err
}

func (l *limitedReadCloser) Close() error {
	return l.rc.Close()
}

// failWriter is a WriteCloser that always returns an error.
type failWriter struct {
	err error
}

func (f *failWriter) Write([]byte) (int, error) {
	return 0, f.err
}

func (f *failWriter) Close() error {
	return f.err
}
