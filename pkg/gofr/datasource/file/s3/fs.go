package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	file_interface "gofr.dev/pkg/gofr/datasource/file"
	"log"
	"mime"
	"os"
	"path"
)

type fileSystem struct {
	file
	// GoFr currently supports the following files types S3 interactions under its FileSystem interface.
	// By Default it is text. Before Creating json file ContentType must be Changed to json.
	// Allowed values are : "application/text", "application/json"
	conn       *s3.Client
	config     *Config
	logger     Logger
	bucketType types.BucketType
	metrics    Metrics
	remoteDir  string // Remote directory path. Base Path for all s3 File Operations.
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

// Connect takes the configurations and creates the bucket using the access_key_id and secret_access_key, region.
// If a bucket already exists then no error is returned.
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

// if no extension is given by default it sets content-type to octet-stream.
// Upload is more feature-rich and robust, ideal for large files or complex upload scenarios
// where multipart support and automatic retries are beneficial.
// PutObject is simpler and suitable for smaller, straightforward uploads without the need for
// multipart handling or advanced features.
// Automatically detect which function to use .....
func (f *fileSystem) Create(name string) (file_interface.File, error) {
	//var msg string
	//st := "ERROR"

	//defer f.sendOperationStats(&FileLog{Operation: "Create", Location: f.remoteDir, Status: &st, Message: &msg}, time.Now())

	_, err := f.conn.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(f.config.BucketName),
		Key:         aws.String(name),
		Body:        bytes.NewReader(make([]byte, 0)),
		ContentType: aws.String(mime.TypeByExtension(path.Ext(name))),
		// this specifies the file must be downloaded before being opened
		ContentDisposition: aws.String("attachment"),
	})

	if err != nil {
		//f.logger.Errorf("Failed to create the file: %v", err)
		return nil, err
	}

	//st = "SUCCESS"
	//msg = "File creation on S3 successfull."
	//f.logger.Logf("File with name %s created.", name)

	return &file{
		conn:    f.conn,
		name:    name,
		logger:  f.logger,
		metrics: f.metrics,
	}, nil
}

func (f *fileSystem) Open(name string) (file_interface.File, error) {
	filePath := path.Join(f.remoteDir, name)

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(filePath),
	})
	if err != nil {
		return nil, err
	}
	return &file{
		conn:         f.conn,
		name:         name,
		logger:       f.logger,
		metrics:      f.metrics,
		body:         res.Body,
		contentType:  *res.ContentType,
		lastModified: *res.LastModified,
		size:         *res.ContentLength,
	}, nil
}

// OpenFile just calls Open method of the FileSystem.
// It is added so that s3 complies with the generic Filesystem interface of GoFr.
func (f *fileSystem) OpenFile(name string, flag int, perm os.FileMode) (file_interface.File, error) {
	return f.Open(name)
}

// TODO: Remove method must support versioning of files.
// TODO: Extend the support for directory buckets also.
// Remove method currently supports deletion of unversioned files on general purpose buckets only.
func (f *fileSystem) Remove(name string) error {
	//var msg string
	//st := "ERROR"

	//defer f.sendOperationStats(&FileLog{Operation: "Remove", Location: f.remoteDir, Status: &st, Message: &msg}, time.Now())

	filePath := path.Join(f.remoteDir, name)

	_, err := f.conn.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(filePath),
	})

	if err != nil {
		//f.logger.Errorf("Error while deleting file: %v", err)
		return err
	}

	//st = "SUCCESS"
	//msg = "File deletion on S3 successfull."
	//f.logger.Logf("File with path %q deleted.", filePath)
	return nil
}

func (f *fileSystem) Rename(oldname, newname string) error {
	//var msg string
	//st := "ERROR"

	//defer f.sendOperationStats(&FileLog{Operation: "Remove", Location: f.remoteDir, Status: &st, Message: &msg}, time.Now())

	if path.Ext(oldname) != path.Ext(newname) {
		//f.logger.Errorf("new file must be same as the old file type")
		return errors.New("Incorrect file type of newname")
	}

	oldPath := path.Join(f.remoteDir, oldname)
	newPath := path.Join(f.remoteDir, newname)

	_, err := f.conn.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket: aws.String(f.config.BucketName),
		// The source object can be up to 5 GB. If the source object is an object that was uploaded by using a multipart upload,
		// the object copy will be a single part object after the source object is copied to the destination bucket.
		CopySource:         aws.String(f.config.BucketName + "/" + oldPath),
		Key:                aws.String(newname),
		ContentType:        aws.String(mime.TypeByExtension(path.Ext(newPath))),
		ContentDisposition: aws.String("attachment"),
	})

	if err != nil {
		//f.logger.Errorf("Error while copying file: %v", err)
		return err
	}

	err = f.Remove(oldname)
	if err != nil {
		//f.logger.Errorf("failed to remove old file %s", oldname)
		return err
	}

	//st = "SUCCESS"
	//msg = "File rename successfully"
	//f.logger.Logf("File %s renames to %s.", oldname, newname)
	return nil
}

func (f *fileSystem) Stat(name string) (file_interface.FileInfo, error) {
	filePath := path.Join(f.remoteDir, name)

	// it is a directory
	if path.Ext(filePath) == "" {
		res, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String(f.config.BucketName),
			Prefix: aws.String(filePath + "/"),
		})
		if err != nil {
			return nil, err
		}

		return &file{
			conn:         f.conn,
			logger:       f.logger,
			metrics:      f.metrics,
			size:         *res.Contents[0].Size,
			name:         filePath,
			lastModified: *res.Contents[0].LastModified,
		}, nil

	}
	// it is a file
	res, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(filePath),
	})
	if err != nil {
		return nil, err
	}

	return &file{
		conn:         f.conn,
		logger:       f.logger,
		metrics:      f.metrics,
		size:         *res.Contents[0].Size,
		name:         *res.Contents[0].Key,
		lastModified: *res.Contents[0].LastModified,
	}, nil

}

//fmt.Printf("List of Objects in %s:\n", "my-bucket")
//for _, object := range output.Contents {
//	fmt.Printf("key=%s size=%d\n", aws.ToString(object.Key), object.Size)
//}
