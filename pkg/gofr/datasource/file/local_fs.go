package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
// directories first.
func (l *localFileSystem) Create(name string) (File, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer l.Observe(OpCreate, startTime, &st, &msg)

	if err := os.MkdirAll(filepath.Dir(name), DefaultDirMode); err != nil {
		msg = fmt.Sprintf("failed to create parent directories of %q: %v", name, err)
		return nil, err
	}

	f, err := os.Create(name)
	if err != nil {
		msg = fmt.Sprintf("failed to create %q: %v", name, err)
		return nil, err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Created %q for writing", name)

	return l.newFile(f), nil
}

// Open opens the named file for reading like os.Open.
func (l *localFileSystem) Open(name string) (File, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer l.Observe(OpOpen, startTime, &st, &msg)

	f, err := os.Open(name)
	if err != nil {
		msg = fmt.Sprintf("failed to open %q: %v", name, err)
		return nil, err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Opened %q for reading", name)

	return l.newFile(f), nil
}

// OpenFile opens the named file with the given flags and permissions like os.OpenFile.
// With O_CREATE, missing parent directories are created first.
func (l *localFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer l.Observe(OpOpenFile, startTime, &st, &msg)

	if flag&os.O_CREATE != 0 {
		if err := os.MkdirAll(filepath.Dir(name), DefaultDirMode); err != nil {
			msg = fmt.Sprintf("failed to create parent directories of %q: %v", name, err)
			return nil, err
		}
	}

	f, err := os.OpenFile(name, flag, perm)
	if err != nil {
		msg = fmt.Sprintf("failed to open %q: %v", name, err)
		return nil, err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Opened %q with flags %d", name, flag)

	return l.newFile(f), nil
}

// RemoveAll removes dirPath and everything under it like os.RemoveAll, and returns nil
// if dirPath does not exist. If the working directory is dirPath or inside it, the
// working directory is moved to dirPath's parent so that relative paths keep working.
func (l *localFileSystem) RemoveAll(dirPath string) error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer l.Observe(OpRemoveAll, startTime, &st, &msg)

	// Both must be read before the removal: Getwd fails once the working directory is gone.
	wd, wdErr := os.Getwd()
	absPath, absErr := filepath.Abs(dirPath)

	if err := os.RemoveAll(dirPath); err != nil {
		msg = fmt.Sprintf("failed to remove %q: %v", dirPath, err)
		return err
	}

	if wdErr == nil && absErr == nil && isSameOrInside(wd, absPath) && !pathExists(wd) {
		if err := os.Chdir(filepath.Dir(absPath)); err != nil {
			msg = fmt.Sprintf("removed %q but failed to leave it: %v", dirPath, err)
			return err
		}
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Directory %q and all contents deleted successfully", dirPath)

	return nil
}

// Rename moves oldname to newname like os.Rename, creating newname's missing parent
// directories first. If the two paths are on different filesystems, a regular file is
// copied and the original removed.
func (l *localFileSystem) Rename(oldname, newname string) error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer l.Observe(OpRename, startTime, &st, &msg)

	// Check the source first so that a failed rename does not leave new directories behind.
	if _, err := os.Lstat(oldname); err != nil {
		msg = fmt.Sprintf("failed to rename %q: %v", oldname, err)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(newname), DefaultDirMode); err != nil {
		msg = fmt.Sprintf("failed to create parent directories of %q: %v", newname, err)
		return err
	}

	err := os.Rename(oldname, newname)
	if isCrossDeviceError(err) {
		err = moveAcrossDevices(oldname, newname, err)
	}

	if err != nil {
		msg = fmt.Sprintf("failed to rename %q to %q: %v", oldname, newname, err)
		return err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Renamed %q to %q successfully", oldname, newname)

	return nil
}

func (l *localFileSystem) newFile(f *os.File) File {
	return newLocalFile(f, l.Logger, l.Metrics, l.Location)
}

// pathExists reports whether path can be stat'ed.
func pathExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// isSameOrInside reports whether path is dir or lies inside it. Both must be absolute.
func isSameOrInside(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)

	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
