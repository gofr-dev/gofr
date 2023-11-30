package file

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

type mockClient struct{}

func (mc *mockClient) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	switch *params.Bucket {
	case "test-bucket-gofr":
		return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("Successful fetch"))}, nil
	default:
		return nil, errors.InvalidParam{Param: []string{"bucket"}}
	}
}

func (mc *mockClient) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	switch *params.Bucket {
	case "test-bucket-gofr":
		return nil, nil
	default:
		return nil, errors.InvalidParam{Param: []string{"bucket"}}
	}
}

func Test_NewAWSFile(t *testing.T) {
	cfg := AWSConfig{
		AccessKey: "random-access-key",
		SecretKey: "random-secret-key",
		Bucket:    "test_bucket",
		Region:    "us-east-2",
	}
	filename := "testfile.txt"
	mode := READWRITE
	f := newAWSS3File(&cfg, filename, mode)

	if f.client == nil {
		t.Error("error expected not nil client")
	}
}

func TestAws_fetch(t *testing.T) {
	m := &mockClient{}
	tests := []struct {
		cfg *aws
		err error
	}{
		{&aws{fileName: "aws.txt", fileMode: APPEND, client: m, bucketName: "test-bucket-gofr"}, nil},
		{&aws{fileName: "aws.txt", fileMode: READ, client: m, bucketName: "random-bucket"},
			&errors.Response{StatusCode: http.StatusInternalServerError, Code: "S3_ERROR", Reason: "Incorrect value for parameter: bucket"}},
	}

	for i, tc := range tests {
		l := newLocalFile(tc.cfg.fileName, tc.cfg.fileMode)

		_ = l.Open()

		err := tc.cfg.fetch(l.FD)

		assert.IsType(t, tc.err, err, i)

		_ = l.Close()
	}
}

func TestAws_push(t *testing.T) {
	m := &mockClient{}
	tests := []struct {
		cfg *aws
		err error
	}{
		{&aws{fileName: "aws.txt", fileMode: READWRITE, client: m, bucketName: "random-bucket"},
			&errors.Response{StatusCode: http.StatusInternalServerError, Code: "S3_ERROR", Reason: "Incorrect value for parameter: bucket"}},
		{&aws{fileName: "awstest.txt", fileMode: READ, client: m, bucketName: "test-bucket-gofr"}, nil},
	}

	for i, tc := range tests {
		l := newLocalFile(tc.cfg.fileName, tc.cfg.fileMode)
		_ = l.Open()

		err := tc.cfg.push(l.FD)

		assert.IsType(t, tc.err, err, i)

		_ = l.Close()
	}
}

func Test_list(t *testing.T) {
	m := &mockClient{}
	s := &aws{fileName: "aws.txt", fileMode: READWRITE, client: m, bucketName: "random-bucket"}
	expErr := ErrListingNotSupported
	_, err := s.list("test")
	assert.Equalf(t, expErr, err, "Test case failed.\nExpected: %v, got: %v", expErr, err)
}

func Test_aws_move(t *testing.T) {
	m := &mockClient{}
	s := &aws{fileName: "aws.txt", fileMode: READWRITE, client: m, bucketName: "random-bucket"}
	err := s.move("", "")

	assert.Nil(t, err, "Test failed")
}
