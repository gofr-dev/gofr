package s3

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gofr.dev/pkg/gofr/datasource/file"
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

	if opts != nil {
		return validateContentType(opts.ContentType)
	}

	return nil
}

// validateContentType returns an error when contentType is non-empty and not a
// well-formed "type/subtype" value. An empty contentType is valid: callers fall
// back to a sniffed or default type. It is shared by the signed-URL and
// NewWriterWithOptions paths so both reject malformed Content-Type values alike.
func validateContentType(contentType string) error {
	if contentType == "" {
		return nil
	}

	parts := strings.SplitN(contentType, "/", contentTypePartsCount)
	if len(parts) != contentTypePartsCount || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("%w: %q", errInvalidContentType, contentType)
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
