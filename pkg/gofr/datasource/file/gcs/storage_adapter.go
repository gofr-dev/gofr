package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/iterator"
)

var (
	// Storage adapter errors.
	errGCSClientNotInitialized   = errors.New("GCS client or bucket is not initialized")
	errEmptyObjectName           = errors.New("object name is empty")
	errInvalidOffset             = errors.New("invalid offset: must be >= 0")
	errEmptySourceOrDest         = errors.New("source and destination names cannot be empty")
	errSameSourceAndDest         = errors.New("source and destination are the same")
	errFailedToCreateReader      = errors.New("failed to create reader")
	errFailedToCreateRangeReader = errors.New("failed to create range reader")
	errObjectNotFound            = errors.New("object not found")
	errFailedToGetObjectAttrs    = errors.New("failed to get object attrs")
	errFailedToDeleteObject      = errors.New("failed to delete object")
	errFailedToCopyObject        = errors.New("failed to copy object")
	errFailedToListObjects       = errors.New("failed to list objects")
	errFailedToListDirectory     = errors.New("failed to list directory")
)

const (
	contentTypeDirectory = "application/x-directory"
	dirPermissions       = 0755
	filePermissions      = 0644
)

// storageAdapter adapts GCS client to implement file.StorageProvider.
type storageAdapter struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

// Connect is a no-op for GCS (connection is managed by FileSystem.Connect).
func (*storageAdapter) Connect(context.Context) error {
	return nil
}

// Health checks if the GCS connection is healthy by verifying bucket access.
func (s *storageAdapter) Health(ctx context.Context) error {
	if s.client == nil || s.bucket == nil {
		return errGCSClientNotInitialized
	}

	_, err := s.bucket.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("GCS health check failed: %w", err)
	}

	return nil
}

// Close closes the GCS client connection.
func (s *storageAdapter) Close() error {
	if s.client != nil {
		return s.client.Close()
	}

	return nil
}

// NewReader creates a reader for the given object.
func (s *storageAdapter) NewReader(ctx context.Context, name string) (io.ReadCloser, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	reader, err := s.bucket.Object(name).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateReader, name, err)
	}

	return reader, nil
}

// NewRangeReader creates a range reader for the given object.
func (s *storageAdapter) NewRangeReader(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	if offset < 0 {
		return nil, fmt.Errorf("%w (got: %d)", errInvalidOffset, offset)
	}

	reader, err := s.bucket.Object(name).NewRangeReader(ctx, offset, length)
	if err != nil {
		return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateRangeReader, name, err)
	}

	return reader, nil
}

// NewWriter creates a writer for the given object.
func (s *storageAdapter) NewWriter(ctx context.Context, name string) io.WriteCloser {
	// GCS NewWriter never returns an error (deferred until Write/Close)
	// But we should validate input
	if name == "" {
		// Return a no-op writer that fails on Write
		return &failWriter{err: errEmptyObjectName}
	}

	return s.bucket.Object(name).NewWriter(ctx)
}

// failWriter is a helper for NewWriter validation errors.
type failWriter struct {
	err error
}

func (fw *failWriter) Write([]byte) (int, error) {
	return 0, fw.err
}

func (fw *failWriter) Close() error {
	return fw.err
}

// StatObject returns metadata for the given object.
func (s *storageAdapter) StatObject(ctx context.Context, name string) (*file.ObjectInfo, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	attrs, err := s.bucket.Object(name).Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, fmt.Errorf("%w %q: %w", errObjectNotFound, name, err)
		}

		return nil, fmt.Errorf("%w for %q: %w", errFailedToGetObjectAttrs, name, err)
	}

	return &file.ObjectInfo{
		Name:         attrs.Name,
		Size:         attrs.Size,
		ContentType:  attrs.ContentType,
		LastModified: attrs.Updated,
		IsDir:        attrs.ContentType == contentTypeDirectory,
	}, nil
}

// DeleteObject deletes the object with the given name.
func (s *storageAdapter) DeleteObject(ctx context.Context, name string) error {
	if name == "" {
		return errEmptyObjectName
	}

	attrs, err := s.bucket.Object(name).Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return fmt.Errorf("%w %q: %w", errObjectNotFound, name, err)
		}

		return fmt.Errorf("%w for %q: %w", errFailedToGetObjectAttrs, name, err)
	}

	err = s.bucket.Object(name).If(storage.Conditions{GenerationMatch: attrs.Generation}).Delete(ctx)
	if err != nil {
		return fmt.Errorf("%w %q: %w", errFailedToDeleteObject, name, err)
	}

	return nil
}

// CopyObject copies an object from src to dst.
func (s *storageAdapter) CopyObject(ctx context.Context, src, dst string) error {
	if src == "" || dst == "" {
		return errEmptySourceOrDest
	}

	if src == dst {
		return errSameSourceAndDest
	}

	srcObj := s.bucket.Object(src)
	dstObj := s.bucket.Object(dst)

	_, err := dstObj.CopierFrom(srcObj).Run(ctx)
	if err != nil {
		return fmt.Errorf("%w from %q to %q: %w", errFailedToCopyObject, src, dst, err)
	}

	return nil
}

// ListObjects lists all objects with the given prefix.
func (s *storageAdapter) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	var objects []string

	it := s.bucket.Objects(ctx, &storage.Query{Prefix: prefix})

	for {
		obj, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("%w with prefix %q: %w", errFailedToListObjects, prefix, err)
		}

		objects = append(objects, obj.Name)
	}

	return objects, nil
}

// ListDir lists objects and prefixes (directories) under the given prefix.
func (s *storageAdapter) ListDir(ctx context.Context, prefix string) ([]file.ObjectInfo, []string, error) {
	var objects []file.ObjectInfo

	var prefixes []string

	it := s.bucket.Objects(ctx, &storage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	})

	for {
		obj, err := it.Next()

		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			return nil, nil, fmt.Errorf("%w %q: %w", errFailedToListDirectory, prefix, err)
		}

		if obj.Prefix != "" {
			prefixes = append(prefixes, obj.Prefix)
			continue
		}

		objects = append(objects, file.ObjectInfo{
			Name:         obj.Name,
			Size:         obj.Size,
			ContentType:  obj.ContentType,
			LastModified: obj.Updated,
			IsDir:        false,
		})
	}

	return objects, prefixes, nil
}
