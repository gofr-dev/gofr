package s3

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Logger interface is used by s3 package to log information about query execution.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
}

// s3Client defines the interface that is used for gofr-s3 datasource as well as mocks to
// streamline the testing as well as the implementation process.
type s3Client interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
}

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
