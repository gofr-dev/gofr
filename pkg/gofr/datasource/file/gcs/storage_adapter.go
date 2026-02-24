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

	// Signed URL errors.
	errGCSBucketNotConfigured  = errors.New("GCS bucket name is not configured")
	errInvalidPrivateKeyPEM    = errors.New("invalid private key PEM")
	errInvalidPrivateKeyFormat = errors.New("invalid private key format")
	errExpiryMustBePositive    = errors.New("expiry duration must be positive")
	errExpiryTooLong           = errors.New("expiry cannot exceed 7 days for GCS signed URLs")
	errInvalidContentType      = errors.New("invalid Content-Type format")
)

const (
	contentTypeDirectory  = "application/x-directory"
	contentTypePartsCount = 2
	maxGCSSignedURLExpiry = 7 * 24 * time.Hour // 7 days// type/subtype
)

// storageAdapter adapts GCS client to implement file.StorageProvider.
type storageAdapter struct {
	cfg    *Config
	client *storage.Client
	bucket *storage.BucketHandle

	// saEmail and saPrivateKey hold parsed service-account credentials for signed URL
	// generation. They are populated once during Connect() and reused on every call to
	// SignedURL(), avoiding repeated JSON+PEM parsing per request.
	// When empty, bucket.SignedURL falls back to the client's ambient credentials
	// (Workload Identity, Application Default Credentials, etc.).
	saEmail      string
	saPrivateKey []byte

	// credParseErr holds any error encountered when parsing CredentialsJSON during Connect().
	// Connect() does not fail on a parse error so that users who never call GenerateSignedURL
	// are unaffected. The error is returned lazily from SignedURL() if it is non-nil.
	credParseErr error
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
		client, err = storage.NewClient(ctx, option.WithCredentialsJSON([]byte(s.cfg.CredentialsJSON))) //nolint:staticcheck // deprecated
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

	// Parse and cache service-account credentials for signed URL signing.
	// Done once at connect time so every SignedURL() call reuses the result instead of
	// repeating JSON unmarshal + PEM decode + key parse on the hot path.
	// When CredentialsJSON is absent (Workload Identity / ADC), saEmail and saPrivateKey
	// remain empty and bucket.SignedURL will use the client's ambient credentials.
	// Parse and cache service-account credentials for signed URL signing.
	// If parsing fails we do NOT abort Connect() — the GCS client is already valid and
	// all non-signed-URL operations will work normally. The parse error is stored and
	// returned lazily from SignedURL() so only callers of that method are affected.
	if s.cfg.CredentialsJSON != "" {
		email, privateKey, parseErr := parseServiceAccountCredentials(s.cfg.CredentialsJSON)
		if parseErr != nil {
			s.credParseErr = fmt.Errorf("credentials cannot be used for signed URLs: %w", parseErr)
		} else {
			s.saEmail = email
			s.saPrivateKey = privateKey
		}
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

	if s.bucket == nil {
		return &failWriter{err: errGCSClientNotInitialized}
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
		parts := strings.SplitN(opts.ContentType, "/", contentTypePartsCount)
		if len(parts) != contentTypePartsCount || parts[0] == "" || parts[1] == "" {
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
			return fmt.Errorf("%w: PKCS8: %s, PKCS1: %s", errInvalidPrivateKeyFormat, err.Error(), err2.Error())
		}
	}

	return nil
}

// buildSignedURLOptions constructs GCS SignedURLOptions with optional metadata.
// When email and privateKey are empty (Workload Identity / ADC deployments), the
// GoogleAccessID and PrivateKey fields are intentionally left unset so that
// bucket.SignedURL falls back to IAM-based signing via the client's credentials.
func buildSignedURLOptions(email string, privateKey []byte, expiry time.Duration, opts *file.FileOptions) *storage.SignedURLOptions {
	signedOpts := &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
		// V4 is the current recommended signing scheme; it supports additional
		// query parameters (e.g., response-content-disposition) and is required
		// for credentials that delegate to IAM signBlob (Workload Identity).
		Scheme: storage.SigningSchemeV4,
	}

	// Only populate explicit HMAC credentials when available.
	if email != "" {
		signedOpts.GoogleAccessID = email
	}

	if len(privateKey) > 0 {
		signedOpts.PrivateKey = privateKey
	}

	if opts == nil {
		return signedOpts
	}

	if opts.ContentType != "" {
		signedOpts.ContentType = opts.ContentType
	}

	if opts.ContentDisposition != "" {
		if signedOpts.QueryParameters == nil {
			signedOpts.QueryParameters = make(url.Values)
		}

		signedOpts.QueryParameters.Set("response-content-disposition", sanitizeContentDisposition(opts.ContentDisposition))
	}

	return signedOpts
}

// sanitizeContentDisposition removes newline characters to prevent header injection.
func sanitizeContentDisposition(value string) string {
	sanitized := strings.ReplaceAll(value, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\n", "")

	return sanitized
}

// SignedURL generates a time-limited signed URL for the given object.
//
// Signing strategy (in priority order):
//  1. Explicit HMAC: when CredentialsJSON was provided at construction time, the
//     parsed private key cached during Connect() is used for local RSA signing — no
//     additional network calls are needed.
//  2. Ambient credentials: when no CredentialsJSON is provided (e.g., Workload Identity
//     on GKE/Cloud Run, Application Default Credentials), bucket.SignedURL delegates
//     signing to the IAM signBlob API using the client's existing auth token.
//
// The signed URL always uses V4 signing and defaults to the GET method.
func (s *storageAdapter) SignedURL(ctx context.Context, name string, expiry time.Duration, opts *file.FileOptions) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if s.cfg == nil || s.cfg.BucketName == "" {
		return "", errGCSBucketNotConfigured
	}

	if s.bucket == nil {
		return "", errGCSClientNotInitialized
	}

	if err := validateSignedURLInput(name, expiry, opts); err != nil {
		return "", err
	}

	// Return any credential parse error that was deferred from Connect().
	if s.credParseErr != nil {
		return "", s.credParseErr
	}

	// saEmail and saPrivateKey are populated during Connect() when CredentialsJSON is
	// provided. When empty, bucket.SignedURL uses IAM-based signing (Workload Identity).
	signedOpts := buildSignedURLOptions(s.saEmail, s.saPrivateKey, expiry, opts)

	signedURL, err := s.bucket.SignedURL(name, signedOpts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return rewriteSignedURLEndpoint(signedURL, s.cfg.EndPoint), nil
}

// rewriteSignedURLEndpoint replaces the scheme and host of a GCS signed URL with those
// of a custom endpoint. This is used to redirect signed URLs to a local emulator
// (e.g., fake-gcs-server) during integration tests without changing the path or query.
func rewriteSignedURLEndpoint(signedURL, endpoint string) string {
	if endpoint == "" {
		return signedURL
	}

	ep, err := url.Parse(endpoint)
	if err != nil {
		return signedURL
	}

	parsed, err := url.Parse(signedURL)
	if err != nil {
		return signedURL
	}

	parsed.Scheme = ep.Scheme
	parsed.Host = ep.Host

	return parsed.String()
}
