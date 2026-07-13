package s3

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	file "gofr.dev/pkg/gofr/datasource/file"
)

// contentTypePartsCount is the number of parts in a well-formed "type/subtype"
// Content-Type value.
const contentTypePartsCount = 2

var (
	errEmptyObjectName        = errors.New("object name is empty")
	errExpiryMustBePositive   = errors.New("expiry duration must be positive")
	errInvalidContentType     = errors.New("invalid Content-Type format")
	errBucketNotConfigured    = errors.New("S3 bucket name is not configured")
	errS3ClientNotInitialized = errors.New("S3 client is not initialized; call Connect first")
)

// GenerateSignedURL returns a time-limited, pre-signed HTTPS URL that grants
// temporary GET (download) access to the named object without requiring the caller
// to hold AWS credentials.
//
// The URL is signed locally using AWS Signature Version 4 with the credentials
// supplied at construction time; no network round-trip is made. It works against
// Amazon S3 and every S3-compatible provider configured through Config.Flavor
// (Cloudflare R2, MinIO, DigitalOcean Spaces, Backblaze B2, ...). For R2/MinIO and
// similar providers the URL host follows the addressing style resolved from the
// Flavor, so links resolve correctly.
//
// opts may set ContentType and ContentDisposition, which become the
// response-content-type and response-content-disposition query parameters so the
// downloaded response carries the requested headers. opts may be nil.
func (f *FileSystem) GenerateSignedURL(ctx context.Context, name string, expiry time.Duration,
	opts *file.FileOptions) (string, error) {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "GENERATE SIGNED URL",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	select {
	case <-ctx.Done():
		msg = ctx.Err().Error()
		return "", ctx.Err()
	default:
	}

	// f.config is guaranteed non-nil by New; only the bucket name needs checking.
	if f.config.BucketName == "" {
		msg = errBucketNotConfigured.Error()
		return "", errBucketNotConfigured
	}

	if f.presigner == nil {
		msg = errS3ClientNotInitialized.Error()
		return "", errS3ClientNotInitialized
	}

	if err := validateSignedURLInput(name, expiry, opts); err != nil {
		msg = err.Error()
		return "", err
	}

	req, err := f.presigner.PresignGetObject(ctx, buildPresignGetObjectInput(f.config.BucketName, name, opts),
		s3.WithPresignExpires(expiry))
	if err != nil {
		msg = fmt.Sprintf("failed to generate signed URL: %v", err)
		return "", fmt.Errorf("failed to generate signed URL for %q: %w", name, err)
	}

	st = statusSuccess
	msg = fmt.Sprintf("generated signed URL for %q (expires in %v)", name, expiry)

	return req.URL, nil
}

// buildPresignGetObjectInput assembles the GetObject request that is presigned.
// ContentType and ContentDisposition, when present, are applied as response-header
// overrides so the eventual download carries them.
func buildPresignGetObjectInput(bucket, name string, opts *file.FileOptions) *s3.GetObjectInput {
	in := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(name),
	}

	if opts == nil {
		return in
	}

	if opts.ContentType != "" {
		in.ResponseContentType = aws.String(opts.ContentType)
	}

	if opts.ContentDisposition != "" {
		in.ResponseContentDisposition = aws.String(sanitizeContentDisposition(opts.ContentDisposition))
	}

	return in
}

// validateSignedURLInput validates the input parameters for signed URL generation.
func validateSignedURLInput(name string, expiry time.Duration, opts *file.FileOptions) error {
	if name == "" {
		return errEmptyObjectName
	}

	if expiry <= 0 {
		return errExpiryMustBePositive
	}

	if opts != nil && opts.ContentType != "" {
		parts := strings.SplitN(opts.ContentType, "/", contentTypePartsCount)
		if len(parts) != contentTypePartsCount || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("%w: %q", errInvalidContentType, opts.ContentType)
		}
	}

	return nil
}

// sanitizeContentDisposition removes newline characters to prevent header injection
// via the response-content-disposition query parameter.
func sanitizeContentDisposition(value string) string {
	sanitized := strings.ReplaceAll(value, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\n", "")

	return sanitized
}
