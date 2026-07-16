package s3

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	file "gofr.dev/pkg/gofr/datasource/file"
)

func Test_New_ReturnsCloudFileSystem(t *testing.T) {
	fs := New(&Config{BucketName: "test-bucket"})
	require.NotNil(t, fs)

	// The concrete type must satisfy the cloud capabilities at runtime too, so
	// consumers can discover them via file.AsCloud.
	cfs, ok := file.AsCloud(fs)
	assert.True(t, ok)
	assert.NotNil(t, cfs)
}

func Test_New_NilConfigDoesNotPanic(t *testing.T) {
	fs := New(nil)
	assert.NotNil(t, fs)
}

func Test_CreateWithOptions_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().
		PutObject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			// The caller-supplied metadata and overrides reach the SDK.
			assert.Equal(t, "text/csv", *in.ContentType)
			assert.Equal(t, `inline; filename="a.csv"`, *in.ContentDisposition)
			assert.Equal(t, map[string]string{"owner": "reports"}, in.Metadata)

			return &s3.PutObjectOutput{}, nil
		})

	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("")),
		ContentLength: aws.Int64(0),
		ContentType:   aws.String("text/csv"),
		LastModified:  aws.Time(time.Now()),
	}, nil)

	_, err := fs.CreateWithOptions(context.Background(), "a.csv", &file.FileOptions{
		ContentType:        "text/csv",
		ContentDisposition: `inline; filename="a.csv"`,
		Metadata:           map[string]string{"owner": "reports"},
	})
	require.NoError(t, err)
}

func Test_CreateWithOptions_NilOptsUsesDefaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().
		PutObject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			assert.Equal(t, "attachment", *in.ContentDisposition)
			assert.Nil(t, in.Metadata)

			return &s3.PutObjectOutput{}, nil
		})

	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("")),
		ContentLength: aws.Int64(0),
		ContentType:   aws.String("text/plain"),
		LastModified:  aws.Time(time.Now()),
	}, nil)

	_, err := fs.CreateWithOptions(context.Background(), "notes.txt", nil)
	require.NoError(t, err)
}

func Test_CreateWithOptions_InvalidContentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	// A malformed Content-Type is rejected up front, for parity with the
	// signed-URL path. No PutObject expectation is set, so any S3 call fails the test.
	_, err := fs.CreateWithOptions(context.Background(), "a.csv", &file.FileOptions{
		ContentType: "notavalidtype",
	})
	require.ErrorIs(t, err, errInvalidContentType)
}

func Test_CreateWithOptions_PutError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(nil, errMock)

	_, err := fs.CreateWithOptions(context.Background(), "a.csv", nil)
	require.ErrorIs(t, err, errMock)
}
