package file

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"gofr.dev/pkg/gofr/datasource"
)

const dirPerm = 0755

// localProvider implements StorageProvider for local filesystem.
type localProvider struct{}

// NewLocalFileSystem creates a FileSystemProvider for local filesystem operations.
func NewLocalFileSystem(logger datasource.Logger) FileSystemProvider {
	provider := &localProvider{}

	return &CommonFileSystem{
		Provider: provider,
		Location: "local",
		Logger:   logger,
		Metrics:  nil,
	}
}

// ============= StorageProvider Implementation =============

func (*localProvider) Connect(context.Context) error {
	return nil // Local FS is always "connected"
}

func (*localProvider) Health(context.Context) error {
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

	defer srcFile.Close()

	if mkdirErr := os.MkdirAll(filepath.Dir(dst), DefaultDirMode); mkdirErr != nil {
		return mkdirErr
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

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
