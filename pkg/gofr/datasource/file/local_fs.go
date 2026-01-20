package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

const dirPerm = 0755

// localProvider implements StorageProvider for local filesystem.
type localProvider struct{}

// localFileSystem is a small adapter that exposes the no-arg Connect()
// required by FileSystemProvider and delegates to CommonFileSystem.
type localFileSystem struct {
	*CommonFileSystem
}

// NewLocalFileSystem creates a FileSystem for local filesystem operations.
func NewLocalFileSystem(logger datasource.Logger) FileSystem {
	provider := &localProvider{}

	cfs := &CommonFileSystem{
		Provider: provider,
		Location: "local",
		Logger:   logger,
		Metrics:  nil,
	}

	return &localFileSystem{CommonFileSystem: cfs}
}

// Implement Connect(ctx context.Context) error for interface compatibility.
func (l *localFileSystem) Connect(ctx context.Context) error {
	return l.CommonFileSystem.Connect(ctx)
}

func (p *localProvider) Connect(_ context.Context) error {
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

// SignedURL is not supported for local filesystems.
func (l *localFileSystem) SignedURL(_ string, _ time.Duration, _ ...*FileOptions) (string, error) {
	return "", fmt.Errorf("SignedURL is not supported for local files")
}

// Update Create to match interface.
func (l *localFileSystem) Create(name string, opts ...*FileOptions) (File, error) {
	return l.CommonFileSystem.Create(name, opts...)
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
