package gcs

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	// Storage adapter errors.
	errGCSConfigNil              = errors.New("GCS config is nil")
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
)

// storageAdapter adapts GCS client to implement file.StorageProvider.
type storageAdapter struct {
	cfg    *Config
	client *storage.Client
	bucket *storage.BucketHandle
}

// Connect initializes the GCS client and validates bucket access.
func (s *storageAdapter) Connect(ctx context.Context) error {
	// fast-path
	if s.client != nil && s.bucket != nil {
		return nil
	}

	if s.cfg == nil {
		return errGCSConfigNil
	}

	var (
		client *storage.Client
		err    error
	)

	switch {
	case s.cfg.EndPoint != "":
		client, err = storage.NewClient(ctx, option.WithEndpoint(s.cfg.EndPoint), option.WithoutAuthentication())
	case s.cfg.CredentialsJSON != "":
		client, err = storage.NewClient(ctx, option.WithCredentialsJSON([]byte(s.cfg.CredentialsJSON)))
	default:
		client, err = storage.NewClient(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}

	bucket := client.Bucket(s.cfg.BucketName)
	if _, err := bucket.Attrs(ctx); err != nil {
		_ = client.Close()
		return fmt.Errorf("bucket validation failed: %w", err)
	}

	s.client = client
	s.bucket = bucket

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

// NewWriterWithOptions implements MetadataWriter.
func (s *storageAdapter) NewWriterWithOptions(ctx context.Context, name string, opts *file.FileOptions) io.WriteCloser {
	if name == "" {
		return &failWriter{err: errEmptyObjectName}
	}

	w := s.bucket.Object(name).NewWriter(ctx)

	if opts != nil {
		if opts.ContentType != "" {
			w.ContentType = opts.ContentType
		}
		if opts.ContentDisposition != "" {
			w.ContentDisposition = opts.ContentDisposition
		}
		if opts.Metadata != nil {
			w.Metadata = opts.Metadata
		}
	}

	return w
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

// SignedURL generates a signed URL for the given object with expiry and optional metadata.
func (s *storageAdapter) SignedURL(_ context.Context, name string, expiry time.Duration, opts ...*file.FileOptions) (string, error) {
	if s.cfg == nil || s.cfg.BucketName == "" {
		return "", errors.New("GCS config or bucket name missing")
	}
	if name == "" {
		return "", errEmptyObjectName
	}
	if s.cfg.CredentialsJSON == "" {
		return "", errors.New("GCS credentials required for signed URL")
	}

	// Parse credentials JSON for service account email and private key
	var cred struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if err := json.Unmarshal([]byte(s.cfg.CredentialsJSON), &cred); err != nil {
		return "", fmt.Errorf("failed to parse credentials: %w", err)
	}

	block, _ := pem.Decode([]byte(cred.PrivateKey))
	if block == nil {
		return "", errors.New("invalid private key PEM")
	}

	optsStruct := &storage.SignedURLOptions{
		GoogleAccessID: cred.ClientEmail,
		PrivateKey:     block.Bytes,
		Method:         "GET",
		Expires:        time.Now().Add(expiry),
	}

	// Set response headers if provided
	if len(opts) > 0 && opts[0] != nil {
		if opts[0].ContentType != "" {
			optsStruct.ContentType = opts[0].ContentType
		}
		if opts[0].ContentDisposition != "" {
			optsStruct.Headers = append(optsStruct.Headers, "response-content-disposition="+opts[0].ContentDisposition)
		}
	}

	url, err := storage.SignedURL(s.cfg.BucketName, name, optsStruct)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}
	return url, nil
}
