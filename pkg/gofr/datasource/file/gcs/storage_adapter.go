package gcs

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
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

	// Signed URL errors
	errGCSConfigMissing        = errors.New("GCS config or bucket name missing")
	errGCSCredentialsMissing   = errors.New("GCS credentials required for signed URL")
	errInvalidPrivateKeyPEM    = errors.New("invalid private key PEM")
	errInvalidPrivateKeyFormat = errors.New("invalid private key format")
	errExpiryMustBePositive    = errors.New("expiry duration must be positive")
	errExpiryTooLong           = errors.New("expiry cannot exceed 7 days for GCS signed URLs")
	errInvalidContentType      = errors.New("invalid Content-Type format")
)

const (
	contentTypeDirectory  = "application/x-directory"
	maxGCSSignedURLExpiry = 7 * 24 * time.Hour // 7 days
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

// validateSignedURLInput validates input parameters for signed URL generation.
func validateSignedURLInput(name string, expiry time.Duration, opts *file.FileOptions) error {
	if name == "" {
		return errEmptyObjectName
	}

	if expiry <= 0 {
		return errExpiryMustBePositive
	}

	if expiry > maxGCSSignedURLExpiry {
		return errExpiryTooLong
	}

	if opts != nil && opts.ContentType != "" {
		if !strings.Contains(opts.ContentType, "/") {
			return fmt.Errorf("%w: %q", errInvalidContentType, opts.ContentType)
		}
	}

	return nil
}

// parseServiceAccountCredentials extracts email and private key from credentials JSON.
func parseServiceAccountCredentials(credentialsJSON string) (email string, privateKey []byte, err error) {
	var cred struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}

	if err := json.Unmarshal([]byte(credentialsJSON), &cred); err != nil {
		return "", nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	block, _ := pem.Decode([]byte(cred.PrivateKey))
	if block == nil {
		return "", nil, errInvalidPrivateKeyPEM
	}

	// Validate key format
	if err := validatePrivateKey(block.Bytes); err != nil {
		return "", nil, err
	}

	return cred.ClientEmail, block.Bytes, nil
}

// validatePrivateKey checks if the private key can be parsed as PKCS8 or PKCS1.
func validatePrivateKey(keyBytes []byte) error {
	if _, err := x509.ParsePKCS8PrivateKey(keyBytes); err != nil {
		if _, err2 := x509.ParsePKCS1PrivateKey(keyBytes); err2 != nil {
			return fmt.Errorf("%w: PKCS8: %v, PKCS1: %v", errInvalidPrivateKeyFormat, err, err2)
		}
	}
	return nil
}

// buildSignedURLOptions constructs GCS SignedURLOptions with optional metadata.
func buildSignedURLOptions(email string, privateKey []byte, expiry time.Duration, opts *file.FileOptions) *storage.SignedURLOptions {
	optsStruct := &storage.SignedURLOptions{
		GoogleAccessID: email,
		PrivateKey:     privateKey,
		Method:         "GET",
		Expires:        time.Now().Add(expiry),
	}

	if opts == nil {
		return optsStruct
	}

	// Set Content-Type
	if opts.ContentType != "" {
		optsStruct.ContentType = opts.ContentType
	}

	// Set Content-Disposition via query parameters
	if opts.ContentDisposition != "" {
		if optsStruct.QueryParameters == nil {
			optsStruct.QueryParameters = make(url.Values)
		}
		// Sanitize to prevent header injection
		sanitized := sanitizeContentDisposition(opts.ContentDisposition)
		optsStruct.QueryParameters.Set("response-content-disposition", sanitized)
	}

	return optsStruct
}

// sanitizeContentDisposition removes newline characters to prevent header injection.
func sanitizeContentDisposition(value string) string {
	sanitized := strings.ReplaceAll(value, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\n", "")
	return sanitized
}

// SignedURL generates a signed URL for the given object with expiry and optional metadata.
// Accepts a single *file.FileOptions to match the SignedURLProvider signature.
func (s *storageAdapter) SignedURL(ctx context.Context, name string, expiry time.Duration, opts *file.FileOptions) (string, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate config
	if s.cfg == nil || s.cfg.BucketName == "" {
		return "", errGCSConfigMissing
	}

	if s.cfg.CredentialsJSON == "" {
		return "", errGCSCredentialsMissing
	}

	// Validate inputs
	if err := validateSignedURLInput(name, expiry, opts); err != nil {
		return "", err
	}

	// Parse credentials
	email, privateKey, err := parseServiceAccountCredentials(s.cfg.CredentialsJSON)
	if err != nil {
		return "", err
	}

	// Build signed URL options
	signedOpts := buildSignedURLOptions(email, privateKey, expiry, opts)

	// Generate signed URL
	signedURL, err := storage.SignedURL(s.cfg.BucketName, name, signedOpts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	// If a custom endpoint is configured (e.g., fake-gcs-server emulator),
	// rewrite the signed URL to use its scheme+host so the signed URL points to emulator.
	if s.cfg.EndPoint != "" {
		if ep, err := url.Parse(s.cfg.EndPoint); err == nil {
			if parsed, err := url.Parse(signedURL); err == nil {
				parsed.Scheme = ep.Scheme
				parsed.Host = ep.Host
				signedURL = parsed.String()
			}
		}
	}
	return signedURL, nil
}
