package s3

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	file "gofr.dev/pkg/gofr/datasource/file"
)

func Test_validateSignedURLInput(t *testing.T) {
	tests := []struct {
		name    string
		object  string
		expiry  time.Duration
		opts    *file.FileOptions
		wantErr error
	}{
		{name: "valid without opts", object: "a/b.txt", expiry: time.Hour, wantErr: nil},
		{name: "valid with content type", object: "a/b.txt", expiry: time.Hour,
			opts: &file.FileOptions{ContentType: "text/csv"}, wantErr: nil},
		{name: "empty name", object: "", expiry: time.Hour, wantErr: errEmptyObjectName},
		{name: "zero expiry", object: "a.txt", expiry: 0, wantErr: errExpiryMustBePositive},
		{name: "negative expiry", object: "a.txt", expiry: -time.Second, wantErr: errExpiryMustBePositive},
		{name: "invalid content type - no slash", object: "a.txt", expiry: time.Hour,
			opts: &file.FileOptions{ContentType: "textcsv"}, wantErr: errInvalidContentType},
		{name: "invalid content type - empty subtype", object: "a.txt", expiry: time.Hour,
			opts: &file.FileOptions{ContentType: "text/"}, wantErr: errInvalidContentType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSignedURLInput(tt.object, tt.expiry, tt.opts)

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func Test_sanitizeContentDisposition(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "clean value unchanged", input: `attachment; filename="report.csv"`, want: `attachment; filename="report.csv"`},
		{name: "strips CRLF injection", input: "attachment\r\nX-Evil: 1", want: "attachmentX-Evil: 1"},
		{name: "strips CR", input: "attachment\rfoo", want: "attachmentfoo"},
		{name: "strips LF", input: "attachment\nfoo", want: "attachmentfoo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeContentDisposition(tt.input))
		})
	}
}

func Test_buildPresignGetObjectInput(t *testing.T) {
	t.Run("nil opts sets only bucket and key", func(t *testing.T) {
		in := buildPresignGetObjectInput("bucket", "dir/obj.txt", nil)

		assert.Equal(t, "bucket", *in.Bucket)
		assert.Equal(t, "dir/obj.txt", *in.Key)
		assert.Nil(t, in.ResponseContentType)
		assert.Nil(t, in.ResponseContentDisposition)
	})

	t.Run("opts map to response overrides and sanitize disposition", func(t *testing.T) {
		in := buildPresignGetObjectInput("bucket", "obj.txt", &file.FileOptions{
			ContentType:        "text/csv",
			ContentDisposition: "attachment\r\n",
		})

		require.NotNil(t, in.ResponseContentType)
		require.NotNil(t, in.ResponseContentDisposition)
		assert.Equal(t, "text/csv", *in.ResponseContentType)
		assert.Equal(t, "attachment", *in.ResponseContentDisposition)
	})
}

// mockPresignerFileSystem builds a FileSystem wired with a mocked presigner and a
// logger that accepts any log calls.
func mockPresignerFileSystem(t *testing.T, cfg *Config) (*FileSystem, *Mocks3Presigner) {
	t.Helper()

	ctrl := gomock.NewController(t)
	presigner := NewMocks3Presigner(ctrl)
	logger := NewMockLogger(ctrl)
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()
	logger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	return &FileSystem{config: cfg, presigner: presigner, logger: logger}, presigner
}

func Test_GenerateSignedURL_Success(t *testing.T) {
	fs, presigner := mockPresignerFileSystem(t, &Config{BucketName: "test-bucket"})

	opts := &file.FileOptions{ContentType: "text/csv", ContentDisposition: "attachment"}

	presigner.EXPECT().
		PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
			// Delegation carries the object and response overrides through to the SDK.
			assert.Equal(t, "test-bucket", *in.Bucket)
			assert.Equal(t, "data/file.csv", *in.Key)
			assert.Equal(t, "text/csv", *in.ResponseContentType)

			return &v4.PresignedHTTPRequest{URL: "https://test-bucket.s3.amazonaws.com/data/file.csv?X-Amz-Signature=abc"}, nil
		})

	url, err := fs.GenerateSignedURL(context.Background(), "data/file.csv", time.Hour, opts)

	require.NoError(t, err)
	assert.Contains(t, url, "X-Amz-Signature")
}

func Test_GenerateSignedURL_Errors(t *testing.T) {
	t.Run("bucket not configured", func(t *testing.T) {
		fs, _ := mockPresignerFileSystem(t, &Config{})

		_, err := fs.GenerateSignedURL(context.Background(), "obj.txt", time.Hour, nil)
		require.ErrorIs(t, err, errBucketNotConfigured)
	})

	t.Run("presigner not initialized", func(t *testing.T) {
		logger := NewMockLogger(gomock.NewController(t))
		logger.EXPECT().Debug(gomock.Any()).AnyTimes()
		fs := &FileSystem{config: &Config{BucketName: "b"}, logger: logger}

		_, err := fs.GenerateSignedURL(context.Background(), "obj.txt", time.Hour, nil)
		require.ErrorIs(t, err, errS3ClientNotInitialized)
	})

	t.Run("empty object name", func(t *testing.T) {
		fs, _ := mockPresignerFileSystem(t, &Config{BucketName: "b"})

		_, err := fs.GenerateSignedURL(context.Background(), "", time.Hour, nil)
		require.ErrorIs(t, err, errEmptyObjectName)
	})

	t.Run("non-positive expiry", func(t *testing.T) {
		fs, _ := mockPresignerFileSystem(t, &Config{BucketName: "b"})

		_, err := fs.GenerateSignedURL(context.Background(), "obj.txt", 0, nil)
		require.ErrorIs(t, err, errExpiryMustBePositive)
	})

	t.Run("invalid content type", func(t *testing.T) {
		fs, _ := mockPresignerFileSystem(t, &Config{BucketName: "b"})

		_, err := fs.GenerateSignedURL(context.Background(), "obj.txt", time.Hour, &file.FileOptions{ContentType: "bogus"})
		require.ErrorIs(t, err, errInvalidContentType)
	})

	t.Run("canceled context", func(t *testing.T) {
		fs, _ := mockPresignerFileSystem(t, &Config{BucketName: "b"})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := fs.GenerateSignedURL(ctx, "obj.txt", time.Hour, nil)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("presigner error is wrapped", func(t *testing.T) {
		fs, presigner := mockPresignerFileSystem(t, &Config{BucketName: "b"})
		presigner.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errMock)

		_, err := fs.GenerateSignedURL(context.Background(), "obj.txt", time.Hour, nil)
		require.ErrorIs(t, err, errMock)
	})
}

// Test_GenerateSignedURL_RealPresign exercises real SigV4 signing (offline, no
// network) to prove the produced URL is a valid presigned S3 URL.
func Test_GenerateSignedURL_RealPresign(t *testing.T) {
	endpoint := "https://s3.example.com"
	awsCfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secretkey", ""),
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = &endpoint
	})

	logger := NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()

	fs := &FileSystem{
		config:    &Config{BucketName: "my-bucket", Region: "us-east-1", EndPoint: endpoint},
		presigner: s3.NewPresignClient(s3Client),
		logger:    logger,
	}

	url, err := fs.GenerateSignedURL(context.Background(), "reports/q1.csv", 15*time.Minute,
		&file.FileOptions{ContentDisposition: `attachment; filename="q1.csv"`})

	require.NoError(t, err)

	// Path-style host + bucket/key, a V4 signature, the requested TTL, and the
	// response-content-disposition override should all be present.
	assert.True(t, strings.HasPrefix(url, "https://s3.example.com/my-bucket/reports/q1.csv"), "got: %s", url)
	assert.Contains(t, url, "X-Amz-Signature=")
	assert.Contains(t, url, "X-Amz-Credential=")
	assert.Contains(t, url, "X-Amz-Expires=900")
	assert.Contains(t, url, "response-content-disposition=")
}
