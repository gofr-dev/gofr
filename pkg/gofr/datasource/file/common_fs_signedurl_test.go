package file_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/datasource/file"
)

var errNotImplemented = errors.New("not implemented")

// basicProviderWithoutSignedURL implements only StorageProvider, NOT SignedURLProvider.
type basicProviderWithoutSignedURL struct{}

func (*basicProviderWithoutSignedURL) Connect(_ context.Context) error { return nil }
func (*basicProviderWithoutSignedURL) NewReader(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errNotImplemented
}
func (*basicProviderWithoutSignedURL) NewRangeReader(_ context.Context, _ string, _, _ int64) (io.ReadCloser, error) {
	return nil, errNotImplemented
}
func (*basicProviderWithoutSignedURL) NewWriter(_ context.Context, _ string) io.WriteCloser {
	return &nopWriteCloser{}
}
func (*basicProviderWithoutSignedURL) DeleteObject(_ context.Context, _ string) error { return nil }
func (*basicProviderWithoutSignedURL) CopyObject(_ context.Context, _, _ string) error {
	return nil
}
func (*basicProviderWithoutSignedURL) StatObject(_ context.Context, name string) (*file.ObjectInfo, error) {
	return &file.ObjectInfo{Name: name, Size: 10}, nil
}
func (*basicProviderWithoutSignedURL) ListObjects(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (*basicProviderWithoutSignedURL) ListDir(_ context.Context, _ string) ([]file.ObjectInfo, []string, error) {
	return nil, nil, nil
}

// fakeProvider implements the subset of StorageProvider + SignedURLProvider + MetadataWriter
// so we can test CommonFileSystem behavior without real GCS.
type fakeProvider struct {
	calledNewWriterWithOptions bool
	calledSignedURL            bool
	signedURLError             error
	lastOpts                   *file.FileOptions
	returnNilWriter            bool
}

func (*fakeProvider) Connect(_ context.Context) error {
	return nil
}

func (*fakeProvider) NewReader(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errNotImplemented
}

func (*fakeProvider) NewRangeReader(_ context.Context, _ string, _, _ int64) (io.ReadCloser, error) {
	return nil, errNotImplemented
}

func (*fakeProvider) NewWriter(_ context.Context, _ string) io.WriteCloser {
	return &nopWriteCloser{}
}
func (*fakeProvider) DeleteObject(_ context.Context, _ string) error  { return nil }
func (*fakeProvider) CopyObject(_ context.Context, _, _ string) error { return nil }
func (*fakeProvider) StatObject(_ context.Context, name string) (*file.ObjectInfo, error) {
	return &file.ObjectInfo{Name: name, Size: 10}, nil
}
func (*fakeProvider) ListObjects(_ context.Context, _ string) ([]string, error) { return nil, nil }
func (*fakeProvider) ListDir(_ context.Context, _ string) ([]file.ObjectInfo, []string, error) {
	return nil, nil, nil
}

// MetadataWriter.
func (f *fakeProvider) NewWriterWithOptions(_ context.Context, _ string, opts *file.FileOptions) io.WriteCloser {
	f.calledNewWriterWithOptions = true
	f.lastOpts = opts

	if f.returnNilWriter {
		return nil
	}

	return &nopWriteCloser{}
}

func (f *fakeProvider) SignedURL(_ context.Context, _ string, _ time.Duration, _ *file.FileOptions) (string, error) {
	f.calledSignedURL = true
	if f.signedURLError != nil {
		return "", f.signedURLError
	}

	return "https://signed.example/obj", nil
}

// nopWriteCloser is a simple write closer used for tests.
type nopWriteCloser struct{}

func (*nopWriteCloser) Write([]byte) (int, error) { return 0, nil }
func (*nopWriteCloser) Close() error              { return nil }

func TestCreateWithOptions_UsesProviderNewWriterWithOptions(t *testing.T) {
	fp := &fakeProvider{}
	cfs := &file.CommonFileSystem{Provider: fp, Location: "b", ProviderName: "FAKE"}

	opts := &file.FileOptions{ContentType: "text/csv"}

	_, err := cfs.CreateWithOptions(context.Background(), "obj", opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !fp.calledNewWriterWithOptions {
		t.Error("expected NewWriterWithOptions to be called")
	}

	if fp.lastOpts.ContentType != "text/csv" {
		t.Errorf("expected ContentType 'text/csv', got %q", fp.lastOpts.ContentType)
	}
}

func TestGenerateSignedURL_DelegatesToProvider(t *testing.T) {
	fp := &fakeProvider{}
	cfs := &file.CommonFileSystem{Provider: fp, Location: "b", ProviderName: "FAKE"}

	signed, err := cfs.GenerateSignedURL(context.Background(), "obj", time.Hour, &file.FileOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if signed != "https://signed.example/obj" {
		t.Fatalf("unexpected signed url: %s", signed)
	}
}

func TestGenerateSignedURL_ProviderDoesNotSupport(t *testing.T) {
	basicProvider := &basicProviderWithoutSignedURL{} // Doesn't implement SignedURLProvider
	cfs := &file.CommonFileSystem{Provider: basicProvider, ProviderName: "BASIC"}

	_, err := cfs.GenerateSignedURL(context.Background(), "obj", time.Hour, nil)
	if !errors.Is(err, file.ErrSignedURLsNotSupported) {
		t.Errorf("expected ErrSignedURLsNotSupported, got %v", err)
	}
}

func TestCreateWithOptions_NilWriterReturnsError(t *testing.T) {
	fp := &fakeProvider{returnNilWriter: true}
	cfs := &file.CommonFileSystem{Provider: fp, Location: "b", ProviderName: "FAKE"}

	_, err := cfs.CreateWithOptions(context.Background(), "obj", nil)
	if err == nil {
		t.Fatal("expected error when NewWriterWithOptions returns nil, got nil")
	}
}
