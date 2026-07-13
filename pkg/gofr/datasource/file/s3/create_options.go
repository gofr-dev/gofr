package s3

import (
	"bytes"
	"context"
	"mime"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	file "gofr.dev/pkg/gofr/datasource/file"
)

// defaultContentDisposition serves objects as an attachment (download) unless the
// caller overrides it, matching Create's behavior.
const defaultContentDisposition = "attachment"

// CreateWithOptions creates an empty object at name, applying the metadata carried
// in opts. It mirrors Create but lets the caller override the Content-Type and
// Content-Disposition and attach user metadata, which is why the S3 provider
// satisfies file.CloudFileSystem.
//
// When opts is nil the behavior matches Create: the Content-Type is inferred from
// the file extension and the Content-Disposition defaults to "attachment".
func (f *FileSystem) CreateWithOptions(ctx context.Context, name string, opts *file.FileOptions) (file.File, error) {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "CREATE FILE WITH OPTIONS",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	contentType := mime.TypeByExtension(path.Ext(name))
	// Default to attachment so the file must be downloaded before being opened,
	// matching Create's behavior.
	contentDisposition := defaultContentDisposition

	var metadata map[string]string

	if opts != nil {
		if opts.ContentType != "" {
			contentType = opts.ContentType
		}

		if opts.ContentDisposition != "" {
			contentDisposition = sanitizeContentDisposition(opts.ContentDisposition)
		}

		metadata = opts.Metadata
	}

	_, err := f.conn.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(f.config.BucketName),
		Key:                aws.String(name),
		Body:               bytes.NewReader(make([]byte, 0)),
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String(contentDisposition),
		Metadata:           metadata,
	})
	if err != nil {
		f.logger.Errorf("failed to create the file: %v", err)
		msg = err.Error()

		return nil, err
	}

	res, err := f.conn.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(name),
	})
	if err != nil {
		f.logger.Errorf("failed to retrieve %q: %v", name, err)
		msg = err.Error()

		return nil, err
	}

	st = statusSuccess
	msg = "file creation on S3 successful"

	f.logger.Logf("file with name %s created with options", name)

	return &S3File{
		conn:         f.conn,
		name:         path.Join(f.config.BucketName, name),
		logger:       f.logger,
		metrics:      f.metrics,
		body:         res.Body,
		contentType:  *res.ContentType,
		lastModified: *res.LastModified,
		size:         *res.ContentLength,
	}, nil
}
