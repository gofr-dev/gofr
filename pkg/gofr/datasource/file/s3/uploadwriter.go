package s3

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"gofr.dev/pkg/gofr/datasource"
)

// minPartSize is the smallest part S3 accepts for a multipart upload (except the last one).
const minPartSize = 5 << 20

// uploadWriter streams written bytes to S3 as a multipart upload in parts of partSize,
// so memory is bounded to one part per open file. PutObject cannot take a live stream
// (the SDK must hash and rewind its body), so a small object that never fills a part is
// sent with a single PutObject on Close.
//
// A failed call aborts the multipart upload and is returned by every later Write and Close.
type uploadWriter struct {
	ctx      context.Context
	client   *s3.Client
	input    *s3.PutObjectInput
	logger   datasource.Logger
	partSize int

	buf      bytes.Buffer
	uploadID *string
	parts    []types.CompletedPart
	closed   bool
	err      error
}

var _ io.WriteCloser = (*uploadWriter)(nil)

func newUploadWriter(ctx context.Context, client *s3.Client, input *s3.PutObjectInput,
	partSize int, logger datasource.Logger) *uploadWriter {
	if partSize <= 0 {
		partSize = minPartSize
	}

	return &uploadWriter{ctx: ctx, client: client, input: input, partSize: partSize, logger: logger}
}

// Write buffers p and uploads a part each time the buffer fills.
func (w *uploadWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, os.ErrClosed
	}

	if w.err != nil {
		return 0, w.err
	}

	written := 0

	for len(p) > 0 {
		take := min(w.partSize-w.buf.Len(), len(p))
		w.buf.Write(p[:take])

		p = p[take:]
		written += take

		if w.buf.Len() >= w.partSize {
			if err := w.uploadPart(); err != nil {
				return written, err
			}
		}
	}

	return written, nil
}

// Close uploads what is buffered and finishes the object. A second Close returns the
// first one's result.
func (w *uploadWriter) Close() error {
	if w.closed {
		return w.err
	}

	w.closed = true

	if w.err != nil {
		return w.err
	}

	if w.uploadID == nil {
		w.input.Body = bytes.NewReader(w.buf.Bytes())

		if _, err := w.client.PutObject(w.ctx, w.input); err != nil {
			w.err = wrapErr(err, "put object", aws.ToString(w.input.Key))
		}

		return w.err
	}

	if w.buf.Len() > 0 {
		if err := w.uploadPart(); err != nil {
			return err
		}
	}

	_, err := w.client.CompleteMultipartUpload(w.ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          w.input.Bucket,
		Key:             w.input.Key,
		UploadId:        w.uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: w.parts},
	})
	if err != nil {
		return w.fail(wrapErr(err, "complete multipart upload", aws.ToString(w.input.Key)))
	}

	return nil
}

// uploadPart starts the multipart upload on the first part, then sends the buffer as the next part.
func (w *uploadWriter) uploadPart() error {
	key := aws.ToString(w.input.Key)

	if w.uploadID == nil {
		out, err := w.client.CreateMultipartUpload(w.ctx, &s3.CreateMultipartUploadInput{
			Bucket:             w.input.Bucket,
			Key:                w.input.Key,
			ContentType:        w.input.ContentType,
			ContentDisposition: w.input.ContentDisposition,
			Metadata:           w.input.Metadata,
		})
		if err != nil {
			return w.fail(wrapErr(err, "create multipart upload", key))
		}

		w.uploadID = out.UploadId
	}

	number := int32(len(w.parts) + 1) //nolint:gosec // S3 allows at most 10000 parts

	out, err := w.client.UploadPart(w.ctx, &s3.UploadPartInput{
		Bucket:     w.input.Bucket,
		Key:        w.input.Key,
		UploadId:   w.uploadID,
		PartNumber: aws.Int32(number),
		Body:       bytes.NewReader(w.buf.Bytes()),
	})
	if err != nil {
		return w.fail(wrapErr(err, "upload part", key))
	}

	w.parts = append(w.parts, types.CompletedPart{ETag: out.ETag, PartNumber: aws.Int32(number)})
	w.buf.Reset()

	return nil
}

// fail records err as the sticky error and aborts the multipart upload, best effort.
func (w *uploadWriter) fail(err error) error {
	w.err = err

	if w.uploadID == nil {
		return err
	}

	// The upload's context may be the reason for the failure; abort must still go out.
	_, abortErr := w.client.AbortMultipartUpload(context.WithoutCancel(w.ctx), &s3.AbortMultipartUploadInput{
		Bucket:   w.input.Bucket,
		Key:      w.input.Key,
		UploadId: w.uploadID,
	})
	if abortErr != nil && w.logger != nil {
		w.logger.Debugf("failed to abort multipart upload of %q: %v", aws.ToString(w.input.Key), abortErr)
	}

	return err
}
