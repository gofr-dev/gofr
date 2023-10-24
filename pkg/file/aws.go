package file

import (
	"context"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"gofr.dev/pkg/errors"
)

type aws struct {
	fileName string
	fileMode Mode

	client     S3Client
	bucketName string
}

const (
	startOffset   = int64(0)
	defaultWhence = 0
)

// S3Client is an interface for interacting with AWS S3.
type S3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

func newAWSS3File(c *AWSConfig, filename string, mode Mode) *aws {
	awsFile := &aws{}

	awsFile.client = s3.New(s3.Options{
		Credentials: credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, c.Token),
		Region:      c.Region,
	})

	awsFile.bucketName = c.Bucket
	awsFile.fileName = filename
	awsFile.fileMode = mode

	return awsFile
}

// need this implementation as this func is required by FTP
func (s *aws) move(string, string) error {
	return nil
}

func (s *aws) fetch(fd *os.File) error {
	resp, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &s.bucketName,
		Key:    &s.fileName,
	})

	if err != nil {
		return &errors.Response{
			Code:   "S3_ERROR",
			Reason: err.Error(),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	_, err = fd.Write(body)

	return err
}

func (s *aws) push(fd *os.File) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Body:   fd,
		Bucket: &s.bucketName,
		Key:    &s.fileName,
	})
	if err != nil {
		return &errors.Response{
			Code:   "S3_ERROR",
			Reason: err.Error(),
		}
	}

	return nil
}

func (s *aws) list(string) ([]string, error) {
	return nil, ErrListingNotSupported
}
