package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

type fileSystem struct {
	file
	conn    *s3.Client
	config  *Config
	logger  Logger
	metrics Metrics
}

// Config represents the s3 configuration.
type Config struct {
	EndPoint          string // AWS S3 base endpoint
	BucketName        string // AWS Bucket name
	Region            string // AWS Region
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

// Connect takes the configurations and validates the connection
// using the access_key_id and secret_access_key, region and assigns
// the s3 client in the filesystem struct as the connection.
func (f *fileSystem) Connect() {
	var msg string
	st := "ERROR"
	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "CONNECT",
		Location:  location,
		Status:    &st,
		Message:   &msg,
	}, time.Now())

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
		f.logger.Errorf("Failed to load configuration", err)
	}

	// Create the S3 client from config
	s3Client := s3.NewFromConfig(cfg,
		func(o *s3.Options) {
			o.UsePathStyle = true
			o.BaseEndpoint = &f.config.EndPoint
		},
	)

	f.conn = s3Client
	st = "SUCCESS"
	msg = "S3 Client connected."
	f.logger.Logf("Connected to S3 bucket %s", f.config.BucketName)
}

// if no extension is given by default it sets content-type to octet-stream.
// Upload is more feature-rich and robust, ideal for large files or complex upload scenarios
// where multipart support and automatic retries are beneficial.
// PutObject is simpler and suitable for smaller, straightforward uploads without the need for
// multipart handling or advanced features.
// TODO: Automatically detect which function to use .....
func (f *fileSystem) Create(name string) (file_interface.File, error) {
	var msg string
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "CREATE FILE",
		Location:  location,
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
			return nil, errors.New("create parent path before creating a file")
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

	st = "SUCCESS"
	msg = "File creation on S3 successfull."
	f.logger.Logf("File with name %s created.", name)

	return &file{
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

// TODO: Remove method must support versioning of files.
// TODO: Extend the support for directory buckets also.
// Remove method currently supports deletion of unversioned files on general purpose buckets only.
func (f *fileSystem) Remove(name string) error {
	var msg string
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "REMOVE FILE",
		Location:  location,
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

	st = "SUCCESS"
	msg = "File deletion on S3 successfull."
	f.logger.Logf("File with path %q deleted.", name)
	return nil
}

func (f *fileSystem) Open(name string) (file_interface.File, error) {
	var msg string
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "OPEN FILE",
		Location:  location,
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(name),
	})

	if err != nil {
		f.logger.Errorf("Failed to retrieve %q: %v", name, err)
		return nil, err
	}

	st = "SUCCESS"
	msg = fmt.Sprintf("File with path %q retrieved successfully.", name)

	return &file{
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

// OpenFile just calls Open method of the FileSystem.
// It is added so that s3 complies with the generic Filesystem interface of GoFr.
func (f *fileSystem) OpenFile(name string, flag int, perm os.FileMode) (file_interface.File, error) {
	return f.Open(name)
}

func (f *fileSystem) renameDirectory(st *string, msg *string, oldPath, newPath string) error {
	entries, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(oldPath + "/"),
	})

	if err != nil {
		f.logger.Errorf("Error while listing objects: %v", err)
		return err
	}

	for _, obj := range entries.Contents {
		newFilePath := strings.Replace(*obj.Key, oldPath, newPath, 1)
		_, err := f.conn.CopyObject(context.TODO(), &s3.CopyObjectInput{
			Bucket:             aws.String(f.config.BucketName),
			CopySource:         aws.String(f.config.BucketName + "/" + *obj.Key),
			Key:                aws.String(newFilePath),
			ContentType:        aws.String(mime.TypeByExtension(path.Ext(newPath))),
			ContentDisposition: aws.String("attachment"),
		})

		if err != nil {
			*msg = fmt.Sprintf("Failed to copy objects to directory %q", newPath)
			return err
		}
	}

	// deleting objects
	err = f.RemoveAll(oldPath)
	if err != nil {
		*msg = fmt.Sprintf("Failed to remove old objects from the directories %q", oldPath)
		return err
	}

	*st = "SUCCESS"
	*msg = fmt.Sprintf("Directory with path %q successfully renamed to %q", oldPath, newPath)

	return nil
}

func (f *fileSystem) Rename(oldname, newname string) error {
	var msg string
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "RENAME",
		Location:  location,
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	// check if it is a directory
	if path.Ext(oldname) == "" {
		return f.renameDirectory(&st, &msg, oldname, newname)
	}

	// if it is a file , check if both exist at same location or not
	if path.Dir(oldname) != path.Dir(newname) {
		f.logger.Errorf("%q & %q are not in same location", oldname, newname)
		return errors.New("renaming as well as moving file to different location is not allowed")
	}

	// check if they have the same name or not
	if oldname == newname {
		f.logger.Logf("%q & %q are same.", oldname, newname)
		return nil
	}

	// check if they are of the same type or not
	if path.Ext(oldname) != path.Ext(newname) {
		f.logger.Errorf("new file must be same as the old file type")
		return errors.New("Incorrect file type of newname")
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

	st = "SUCCESS"
	msg = "File renamed successfully"

	f.logger.Logf("File with path %q renamed to %q", oldname, newname)
	return nil
}

func (f *fileSystem) Stat(name string) (file_interface.FileInfo, error) {
	var msg string
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "STAT",
		Location:  location,
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	filetype := "file"

	if path.Ext(name) == "" {
		if name[0] == 'b' {
			name = name[1:]
		}
		filetype = "directory"
	}

	res, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(name),
	})

	if err != nil {
		f.logger.Errorf("Error returning file info: %v", err)
		return nil, err
	}

	if filetype == "directory" {
		var size int64
		var lastModified time.Time

		for i := range res.Contents {
			size += *res.Contents[i].Size
			if res.Contents[i].LastModified.After(lastModified) {
				lastModified = *res.Contents[i].LastModified
			}
		}

		// directory exist and first value gives information about the directory
		st = "SUCCESS"
		msg = fmt.Sprintf("Directory with path %q info retrieved successfully", name)

		if res.Contents != nil {
			return &file{
				conn:         f.conn,
				logger:       f.logger,
				metrics:      f.metrics,
				size:         size,
				name:         path.Join(f.config.BucketName, *res.Contents[0].Key),
				lastModified: lastModified,
			}, nil
		}
		return nil, nil
	}

	return &file{
		conn:         f.conn,
		logger:       f.logger,
		metrics:      f.metrics,
		size:         *res.Contents[0].Size,
		name:         path.Join(f.config.BucketName, *res.Contents[0].Key),
		lastModified: *res.Contents[0].LastModified,
	}, nil

}
