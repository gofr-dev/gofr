package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	errS3ConfigNil       = errors.New("S3 config is nil")
	errInvalidOffset     = errors.New("invalid offset: must be >= 0")
	errEmptySourceOrDest = errors.New("source and destination names cannot be empty")
	errSameSourceAndDest = errors.New("source and destination are the same")

	// ErrRangeNotHonored is returned when a byte-range read was requested but the
	// backend answered with the whole object (no Content-Range header). Filling the
	// caller's buffer from byte 0 in that case would silently serve the wrong bytes,
	// so the read is refused instead.
	ErrRangeNotHonored = errors.New("s3 backend did not honor the requested byte range")
)

// defaultContentDisposition serves objects as an attachment (download) unless the
// caller overrides it.
const defaultContentDisposition = "attachment"

// storageAdapter adapts the AWS S3 client to file.StorageProvider, file.MetadataWriter
// and file.SignedURLProvider.
type storageAdapter struct {
	cfg *Config

	// mu guards client and presigner. Connect may run on the background retry goroutine
	// while request goroutines use the adapter, so both are written once, together, under
	// mu and read through handles.
	mu        sync.RWMutex
	client    *s3.Client
	presigner *s3.PresignClient

	logger datasource.Logger

	// partSize is the multipart part size; zero means minPartSize. Tests shrink it.
	partSize int
}

// Connect builds the S3 client from the configuration and validates access to the bucket.
func (s *storageAdapter) Connect(ctx context.Context) error {
	if _, _, err := s.handles(); err == nil {
		return nil
	}

	if s.cfg == nil {
		return errS3ConfigNil
	}

	// Resolve provider-specific defaults (addressing style, signing region, checksum
	// behavior) for the configured Flavor before building the client.
	prof := resolveProfile(s.cfg)

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion(prof.region(s.cfg)),
		awsConfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(s.cfg.AccessKeyID, s.cfg.SecretAccessKey, s.cfg.SessionToken)),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, applyClientOptions(prof, s.cfg.EndPoint))

	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.cfg.BucketName)}); err != nil {
		return fmt.Errorf("bucket validation failed: %w", err)
	}

	s.mu.Lock()
	if s.client == nil {
		s.client, s.presigner = client, s3.NewPresignClient(client)
	}
	s.mu.Unlock()

	return nil
}

// applyClientOptions returns the s3.Options mutator that translates a resolved
// Flavor profile into concrete client settings: addressing style, base endpoint,
// and upload-checksum behavior. It is separated from Connect so the flavor→SDK
// wiring can be asserted in a unit test without a live S3 endpoint.
//
// An empty endpoint leaves BaseEndpoint unset so the SDK resolves the regional AWS endpoint.
func applyClientOptions(prof profile, endpoint string) func(*s3.Options) {
	return func(o *s3.Options) {
		o.UsePathStyle = prof.usePathStyle

		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}

		// Several S3-compatible providers reject the CRC32 checksum that
		// aws-sdk-go-v2 adds to uploads by default; only calculate it when the
		// request actually requires one.
		if prof.disableUploadChecksum {
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		}
	}
}

// handles returns the connected client and presigner, or errS3ClientNotInitialized while
// the adapter is not connected.
func (s *storageAdapter) handles() (*s3.Client, *s3.PresignClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.client == nil || s.presigner == nil {
		return nil, nil, errS3ClientNotInitialized
	}

	return s.client, s.presigner, nil
}

func (s *storageAdapter) bucket() *string {
	return aws.String(s.cfg.BucketName)
}

// isNotFound reports whether err is S3's answer for a missing object.
func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return true
		case "NoSuchBucket":
			return false
		}
	}

	var respErr *awshttp.ResponseError

	return errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound
}

// isRangeNotSatisfiable reports whether err is a 416, which S3 returns for a range read
// that starts at or beyond the end of the object (including any range of an empty object).
func isRangeNotSatisfiable(err error) bool {
	var respErr *awshttp.ResponseError

	return errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusRequestedRangeNotSatisfiable
}

// wrapErr marks a missing object with os.ErrNotExist so callers (and the shared layer's
// NOT_FOUND status) can recognize it with errors.Is; the original error stays in the chain.
func wrapErr(err error, op, key string) error {
	if isNotFound(err) {
		return fmt.Errorf("%w: %s %q: %w", os.ErrNotExist, op, key, err)
	}

	return fmt.Errorf("%s %q: %w", op, key, err)
}

// NewReader creates a reader for the whole object.
func (s *storageAdapter) NewReader(ctx context.Context, name string) (io.ReadCloser, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	client, _, err := s.handles()
	if err != nil {
		return nil, err
	}

	out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: s.bucket(), Key: aws.String(name)})
	if err != nil {
		return nil, wrapErr(err, "get object", name)
	}

	return out.Body, nil
}

// NewRangeReader creates a reader for length bytes of the object starting at offset.
func (s *storageAdapter) NewRangeReader(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	if offset < 0 {
		return nil, fmt.Errorf("%w (got: %d)", errInvalidOffset, offset)
	}

	if length <= 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	client, _, err := s.handles()
	if err != nil {
		return nil, err
	}

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: s.bucket(),
		Key:    aws.String(name),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)),
	})
	if err != nil {
		if isRangeNotSatisfiable(err) {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}

		return nil, wrapErr(err, "get object range", name)
	}

	// A range request answered with the whole object (no Content-Range) would fill the
	// caller's buffer from byte 0; refuse rather than serve the wrong bytes.
	if out.ContentRange == nil {
		_ = out.Body.Close()

		return nil, fmt.Errorf("%w: %q", ErrRangeNotHonored, name)
	}

	return out.Body, nil
}

// StatObject returns metadata for the object. Backends that omit Content-Length
// (e.g. some Cloudflare R2 responses) report a Size of 0, which the shared layer treats as unknown.
func (s *storageAdapter) StatObject(ctx context.Context, name string) (*file.ObjectInfo, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	client, _, err := s.handles()
	if err != nil {
		return nil, err
	}

	out, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: s.bucket(), Key: aws.String(name)})
	if err != nil {
		return nil, wrapErr(err, "head object", name)
	}

	return &file.ObjectInfo{
		Name:         name,
		Size:         aws.ToInt64(out.ContentLength),
		ContentType:  aws.ToString(out.ContentType),
		LastModified: aws.ToTime(out.LastModified),
		IsDir:        strings.HasSuffix(name, "/"),
	}, nil
}

// DeleteObject deletes the object.
func (s *storageAdapter) DeleteObject(ctx context.Context, name string) error {
	if name == "" {
		return errEmptyObjectName
	}

	client, _, err := s.handles()
	if err != nil {
		return err
	}

	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: s.bucket(), Key: aws.String(name)}); err != nil {
		return wrapErr(err, "delete object", name)
	}

	return nil
}

// CopyObject copies an object from src to dst within the bucket.
func (s *storageAdapter) CopyObject(ctx context.Context, src, dst string) error {
	if src == "" || dst == "" {
		return errEmptySourceOrDest
	}

	if src == dst {
		return errSameSourceAndDest
	}

	client, _, err := s.handles()
	if err != nil {
		return err
	}

	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     s.bucket(),
		Key:        aws.String(dst),
		CopySource: aws.String(s.cfg.BucketName + "/" + escapeKey(src)),
	})
	if err != nil {
		return wrapErr(err, "copy object", src)
	}

	return nil
}

// escapeKey URL-encodes an object key for the x-amz-copy-source header, keeping "/" separators.
func escapeKey(key string) string {
	segments := strings.Split(key, "/")

	for i, seg := range segments {
		// QueryEscape encodes a literal "+" as %2B and a space as "+", which S3 would
		// read back as a space in a path; turn the latter into %20.
		segments[i] = strings.ReplaceAll(url.QueryEscape(seg), "+", "%20")
	}

	return strings.Join(segments, "/")
}

// ListObjects lists the keys of every object under prefix, following pagination.
func (s *storageAdapter) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	client, _, err := s.handles()
	if err != nil {
		return nil, err
	}

	var keys []string

	pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: s.bucket(), Prefix: optional(prefix)})

	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, wrapErr(err, "list objects", prefix)
		}

		for i := range page.Contents {
			keys = append(keys, aws.ToString(page.Contents[i].Key))
		}
	}

	return keys, nil
}

// ListDir lists the objects and sub-prefixes directly under prefix, following pagination.
func (s *storageAdapter) ListDir(ctx context.Context, prefix string) ([]file.ObjectInfo, []string, error) {
	client, _, err := s.handles()
	if err != nil {
		return nil, nil, err
	}

	var (
		objects  []file.ObjectInfo
		prefixes []string
	)

	pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket:    s.bucket(),
		Prefix:    optional(prefix),
		Delimiter: aws.String("/"),
	})

	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, nil, wrapErr(err, "list directory", prefix)
		}

		for i := range page.Contents {
			obj := &page.Contents[i]

			objects = append(objects, file.ObjectInfo{
				Name:         aws.ToString(obj.Key),
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
			})
		}

		for i := range page.CommonPrefixes {
			prefixes = append(prefixes, aws.ToString(page.CommonPrefixes[i].Prefix))
		}
	}

	return objects, prefixes, nil
}

func optional(v string) *string {
	if v == "" {
		return nil
	}

	return aws.String(v)
}

// NewWriter returns a writer that uploads the object in parts as they fill. The object
// exists only after Close succeeds; see uploadWriter.
func (s *storageAdapter) NewWriter(ctx context.Context, name string) io.WriteCloser {
	return s.NewWriterWithOptions(ctx, name, nil)
}

// NewWriterWithOptions implements file.MetadataWriter. With nil opts the Content-Type is
// inferred from the extension and the Content-Disposition defaults to "attachment".
func (s *storageAdapter) NewWriterWithOptions(ctx context.Context, name string, opts *file.FileOptions) io.WriteCloser {
	if name == "" {
		return &failWriter{err: errEmptyObjectName}
	}

	client, _, err := s.handles()
	if err != nil {
		return &failWriter{err: err}
	}

	input := &s3.PutObjectInput{
		Bucket:             s.bucket(),
		Key:                aws.String(name),
		ContentType:        optional(mime.TypeByExtension(path.Ext(name))),
		ContentDisposition: aws.String(defaultContentDisposition),
	}

	if opts != nil {
		if err := validateContentType(opts.ContentType); err != nil {
			return &failWriter{err: err}
		}

		if opts.ContentType != "" {
			input.ContentType = aws.String(opts.ContentType)
		}

		if opts.ContentDisposition != "" {
			input.ContentDisposition = aws.String(sanitizeContentDisposition(opts.ContentDisposition))
		}

		input.Metadata = opts.Metadata
	}

	return newUploadWriter(ctx, client, input, s.partSize, s.logger)
}

// SignedURL implements file.SignedURLProvider. The URL is signed locally with SigV4; no
// network round-trip is made, and its host follows the addressing style of the client.
func (s *storageAdapter) SignedURL(ctx context.Context, name string, expiry time.Duration, opts *file.FileOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if s.cfg == nil || s.cfg.BucketName == "" {
		return "", errBucketNotConfigured
	}

	_, presigner, err := s.handles()
	if err != nil {
		return "", err
	}

	if vErr := validateSignedURLInput(name, expiry, opts); vErr != nil {
		return "", vErr
	}

	req, err := presigner.PresignGetObject(ctx, buildPresignGetObjectInput(s.cfg.BucketName, name, opts),
		s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL for %q: %w", name, err)
	}

	return req.URL, nil
}

// failWriter reports a validation error from Write and Close.
type failWriter struct {
	err error
}

func (fw *failWriter) Write([]byte) (int, error) { return 0, fw.err }

func (fw *failWriter) Close() error { return fw.err }
