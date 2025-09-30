package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	file "gofr.dev/pkg/gofr/datasource/file"
)

const (
	typeFile      = "file"
	typeDirectory = "directory"
)

var (
	errIncorrectFileType = errors.New("incorrect file type")
)

// client struct embeds the *s3.Client.
type client struct {
	*s3.Client
}

type FileSystem struct {
	s3File  S3File
	conn    s3Client
	config  *Config
	logger  Logger
	metrics Metrics
}

// Config represents the s3 configuration.
type Config struct {
	EndPoint        string // AWS S3 base endpoint
	BucketName      string // AWS Bucket name
	Region          string // AWS Region
	AccessKeyID     string // Aws configs
	SecretAccessKey string // Aws configs
}

// New initializes a new instance of FTP fileSystem with provided configuration.
func New(config *Config) file.FileSystemProvider {
	return &FileSystem{config: config}
}

// UseLogger sets the Logger interface for the FTP file system.
func (f *FileSystem) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the Metrics interface.
func (f *FileSystem) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}

// Connect initializes and validates the connection to the S3 service.
//
// This method sets up the S3 client using the provided configuration, including access key, secret key, region, and base endpoint.
// It loads the AWS configuration and creates an S3 client, which is then assigns it to the `fileSystem` struct.
// This method also logs the outcome of the connection attempt.
func (f *FileSystem) Connect() {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "CONNECT",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	f.logger.Debugf("connecting to S3 bucket: %s", f.config.BucketName)

	// Load the AWS configuration
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(f.config.Region),
		awsConfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				f.config.AccessKeyID,
				f.config.SecretAccessKey,
				"")), // "" is the session token. Currently, we do not handle connections through session token.
	)
	if err != nil {
		f.logger.Errorf("failed to load configuration: %v", err)
		return
	}

	// Create the S3 client from config
	s3Client := s3.NewFromConfig(cfg,
		func(o *s3.Options) {
			o.UsePathStyle = true
			o.BaseEndpoint = &f.config.EndPoint
		},
	)

	f.conn = client{s3Client}
	st = statusSuccess
	msg = "S3 Client connected."

	f.logger.Logf("connected to S3 bucket %s", f.config.BucketName)
}

// Create creates a new file in the S3 bucket.
//
// This method creates an empty file at the specified path in the S3 bucket. It first checks if the parent directory exists;
// if the parent directory does not exist, it returns an error. After creating the file, it retrieves the file metadata
// and returns a `file` object representing the newly created file.
func (f *FileSystem) Create(name string) (file.File, error) {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "CREATE FILE",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	parentPath := path.Dir(name)

	// if parentPath is not empty, we check if it exists or not.
	if parentPath != "." {
		res2, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String(f.config.BucketName),
			Prefix: aws.String(parentPath + "/"),
		})
		if err != nil {
			return nil, err
		}

		if len(res2.Contents) == 0 {
			f.logger.Errorf("Parentpath %q does not exist", parentPath)
			return nil, fmt.Errorf("%w: create parent path before creating a file", ErrOperationNotPermitted)
		}
	}

	_, err := f.conn.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(f.config.BucketName),
		Key:         aws.String(name),
		Body:        bytes.NewReader(make([]byte, 0)),
		ContentType: aws.String(mime.TypeByExtension(path.Ext(name))),
		// this specifies the file must be downloaded before being opened
		ContentDisposition: aws.String("attachment"),
	})
	if err != nil {
		f.logger.Errorf("Failed to create the file: %v", err)
		return nil, err
	}

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(name),
	})
	if err != nil {
		f.logger.Errorf("Failed to retrieve %q: %v", name, err)
		return nil, err
	}

	st = statusSuccess
	msg = "File creation on S3 successful."

	f.logger.Logf("File with name %s created.", name)

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

// Remove deletes a file from the S3 bucket.
//
// This method deletes the specified file from the S3 bucket. Currently, it supports the deletion of unversioned files
// from general-purpose buckets only. Directory buckets and versioned files are not supported for deletion by this method.
func (f *FileSystem) Remove(name string) error {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "REMOVE FILE",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	_, err := f.conn.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(name),
	})
	if err != nil {
		f.logger.Errorf("Error while deleting file: %v", err)
		return err
	}

	st = statusSuccess
	msg = "File deletion on S3 successful"

	f.logger.Logf("File with path %q deleted", name)

	return nil
}

// Open retrieves a file from the S3 bucket and returns a `file` object representing it.
//
// This method fetches the specified file from the S3 bucket and returns a `file` object with its content and metadata.
// If the file cannot be retrieved, it returns an error.
func (f *FileSystem) Open(name string) (file.File, error) {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "OPEN FILE",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(name),
	})
	if err != nil {
		f.logger.Errorf("failed to retrieve %q: %v", name, err)
		return nil, err
	}

	st = statusSuccess
	msg = fmt.Sprintf("File with path %q retrieved successfully", name)

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

// OpenFile is a wrapper for the Open method to comply with the generic FileSystem interface.
//
// This method calls the `Open` method of the `fileSystem` struct to retrieve a file. It is provided to align with the
// FileSystem interface requirements in the GoFr framework.
func (f *FileSystem) OpenFile(name string, _ int, _ os.FileMode) (file.File, error) {
	return f.Open(name)
}

// Rename changes the name of a file or directory within the S3 bucket.
//
// This method handles both files and directories. It ensures that:
// - The new name does not move the file to a different directory.
// - The file types of the old and new names match.
//
// If the old and new names are the same, no operation is performed.
func (f *FileSystem) Rename(oldname, newname string) error {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "RENAME",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	// check if they have the same name or not
	if oldname == newname {
		f.logger.Logf("%q & %q are same", oldname, newname)
		return nil
	}

	// check if both exist at same location or not
	if path.Dir(oldname) != path.Dir(newname) {
		f.logger.Errorf("%q & %q are not in same location", oldname, newname)
		return fmt.Errorf("%w: renaming as well as moving file to different location is not allowed", ErrOperationNotPermitted)
	}

	// check if it is a directory
	if path.Ext(oldname) == "" {
		return f.renameDirectory(&st, &msg, oldname, newname)
	}

	// check if they are of the same type or not
	if path.Ext(oldname) != path.Ext(newname) {
		f.logger.Errorf("new file must be same as the old file type")
		return fmt.Errorf("%w: new filename must match the old file's type", errIncorrectFileType)
	}

	_, err := f.conn.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket: aws.String(f.config.BucketName),
		// The source object can be up to 5 GB.
		// If the source object is an object that was uploaded by using a multipart upload, the object copy
		// will be a single part object after the source object is copied to the destination bucket.
		CopySource:         aws.String(f.config.BucketName + "/" + oldname),
		Key:                aws.String(newname),
		ContentType:        aws.String(mime.TypeByExtension(path.Ext(newname))),
		ContentDisposition: aws.String("attachment"),
	})
	if err != nil {
		msg = fmt.Sprintf("Error while copying file: %v", err)
		return err
	}

	err = f.Remove(oldname)
	if err != nil {
		msg = fmt.Sprintf("failed to remove old file %s", oldname)
		return err
	}

	st = statusSuccess
	msg = "File renamed successfully"

	f.logger.Logf("File with path %q renamed to %q", oldname, newname)

	return nil
}
