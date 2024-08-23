package s3

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	file_interface "gofr.dev/pkg/gofr/datasource/file"
	"log"
	"os"
)

type fileSystem struct {
	file
	// GoFr currently supports the following files types S3 interactions under its FileSystem interface.
	// By Default it is text. Before Creating json file ContentType must be Changed to json.
	ContentType string // Allowed values are : "application/text", "application/json"
	conn        *s3.Client
	config      *Config
	logger      Logger
	bucketType  types.BucketType
	metrics     Metrics
	remoteDir   string // Remote directory path. Base Path for all s3 File Operations.
	// It is "/" by default.
}

// Config represents the s3 configuration.
type Config struct {
	EndPoint   string // AWS S3 endpoint  - not being used in actual aws usecase
	User       string // AWS IAM username - not being used in actual aws usecase
	BucketName string // AWS Bucket name
	Region     string // AWS Region

	BucketType string // Takes two string arguments : "flat" & "directory" depending on what kind of s3 bucket is added.
	// While accessing a flat bucket it is necessary that the objects have key names
	// as relative paths, as we are simulating a filesystem. Any existing file with
	// keys that are not described as path must be considered a file that is directly
	// present in root folder.

	ACCESS_KEY_ID     string // Aws configs
	SECRET_ACCESS_KEY string // Aws configs
}

// New initializes a new instance of FTP fileSystem with provided configuration.
func New(config *Config) file_interface.FileSystemProvider {
	return &fileSystem{config: config}
}

// UseLogger sets the Logger interface for the FTP file system.
func (f *fileSystem) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the Metrics interface.
func (f *fileSystem) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}

// Connect takes the configurations and creates the bucket using the access_key_id and secret_access_key, region
func (f *fileSystem) Connect() {
	// currently the implementation is only for general purpose buckets
	// TODO : Implement for Directory Buckets also
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(f.config.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				f.config.ACCESS_KEY_ID,
				f.config.SECRET_ACCESS_KEY,
				"")),
	)

	if err != nil {

		log.Fatal("failed to load configuration", err)
	}

	// Create the S3 client from config
	s3Client := s3.NewFromConfig(cfg)

	resp, err := s3Client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(f.config.BucketName),
	})
	if err != nil {
		log.Fatal(err)
	}
	f.conn = s3Client

	fmt.Printf("bucket created at %s\n", aws.ToString(resp.Location))
}

func (f *fileSystem) Create(name string) (file_interface.File, error) {
	body2 := []byte(fmt.Sprintf("Hello from localstack 2. This is Golang."))
	_, err := f.conn.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(f.config.BucketName),
		Key:         aws.String(name),
		Body:        bytes.NewReader(body2),
		ContentType: aws.String("application/text"),
		// this specifies the file to must be downloaded before being opened
		ContentDisposition: aws.String("attachment"),
	})

	if err != nil {

		return nil, fmt.Errorf("failed to put object: %w", err)
	}

	return &file{
		conn:    f.conn,
		name:    name,
		logger:  f.logger,
		metrics: f.metrics,
	}, nil
}

func (f *fileSystem) Open(name string) (file_interface.File, error) {
	_, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{Key: aws.String(name)})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (f *fileSystem) OpenFile(name string, flag int, perm os.FileMode) (file_interface.File, error) {
	_, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{Key: aws.String(name)})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (f *fileSystem) Remove(name string) error {
	_, err := f.conn.DeleteObject(context.TODO(), &s3.DeleteObjectInput{})
	if err != nil {
		return err
	}
	return nil
}

func (*fileSystem) Rename(oldname, newname string) error {

	return nil
}

//// Put Keys i.e. Write
//s3Key1 := "key1"
//body1 := []byte(fmt.Sprintf("Hello from localstack 1"))
//_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
//	Bucket:             aws.String("my-bucket"),
//	Key:                aws.String(s3Key1),
//	Body:               bytes.NewReader(body1),
//	ContentType:        aws.String("application/text"),
//	ContentDisposition: aws.String("attachment"),
//})
//if err != nil {
//	return fmt.Errorf("failed to put object: %w", err)
//}
//
//s3Key2 := "key2"
//body2 := []byte(fmt.Sprintf("Hello from localstack 2. This is Golang."))
//_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
//	Bucket:             aws.String("my-bucket"),
//	Key:                aws.String(s3Key2),
//	Body:               bytes.NewReader(body2),
//	ContentType:        aws.String("application/text"),
//	ContentDisposition: aws.String("attachment"),
//})
//if err != nil {
//	return fmt.Errorf("failed to put object: %w", err)
//}
//
//// List Objects i.e. ReadDir
//output, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
//	Bucket: aws.String("my-bucket"),
//})
//if err != nil {
//	log.Fatal(err)
//}
//
//fmt.Printf("List of Objects in %s:\n", "my-bucket")
//for _, object := range output.Contents {
//	fmt.Printf("key=%s size=%d\n", aws.ToString(object.Key), object.Size)
//}
