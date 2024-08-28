package s3

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

// MkDir at root level creating directory.then creating files.
func (f *fileSystem) Mkdir(name string, perm os.FileMode) error {
	var msg string
	st := "ERROR"
	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "MkDirAll",
		Location:  location,
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	directories := strings.Split(name, string(filepath.Separator))
	var currentdir string

	for _, dir := range directories {
		currentdir = path.Join(currentdir, dir)
		_, err := f.conn.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(f.config.BucketName),
			Key:    aws.String(currentdir + "/"),
		})

		if err != nil {
			return err
		}
	}

	st = "SUCCESS"
	msg = "File created successfully"

	return nil
}

// MkDirAll just calls MkDir as aws s3 buckets do not functional on directory or file levels but have a flat structure.
func (f *fileSystem) MkdirAll(name string, perm os.FileMode) error {
	return f.Mkdir(name, perm)
}

func (f *fileSystem) RemoveAll(name string) error {
	var msg string
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{Operation: "RemoveAll", Location: location, Status: &st, Message: &msg}, time.Now())
	if path.Ext(name) != "" {
		f.logger.Errorf("RemoveAll supports deleting directories only. Use Remove instead.")
		return errors.New("invalid argument type. Enter a valid directory name")
	}

	res, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(name + "/"),
	})

	if err != nil {
		return err
	}

	objects := make([]types.ObjectIdentifier, len(res.Contents))

	for i, obj := range res.Contents {
		objects[i] = types.ObjectIdentifier{
			Key: obj.Key,
		}
	}

	_, err = f.conn.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: aws.String(f.config.BucketName),
		Delete: &types.Delete{
			Objects: objects,
		},
	})

	if err != nil {
		f.logger.Errorf("Error while deleting directory: %v", err)
		return err
	}

	st = "SUCCESS"
	msg = "Directory deletion on S3 successfull."
	f.logger.Logf("Directory %s deleted.", name)
	return nil
}

func (f *fileSystem) ReadDir(name string) ([]file_interface.FileInfo, error) {
	var filePath string

	if name == "." {
		filePath = ""
	} else {
		filePath = name + string(filepath.Separator)
	}

	entries, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(filePath),
	})

	if err != nil {
		return nil, err
	}

	var fileInfo []file_interface.FileInfo

	for i := range entries.Contents {
		if i == 0 {
			continue
		}

		relativepath := strings.TrimPrefix(*entries.Contents[i].Key, filePath)
		oneLevelDeepPathIndex := strings.Index(relativepath, string(filepath.Separator))

		if oneLevelDeepPathIndex != -1 {
			relativepath = relativepath[:oneLevelDeepPathIndex+1]
		}

		if len(fileInfo) > 0 {
			temp, ok := fileInfo[len(fileInfo)-1].(*file)

			if ok && relativepath == temp.name {
				continue
			}
		}

		fileInfo = append(fileInfo, &file{
			conn:         f.conn,
			logger:       f.logger,
			metrics:      f.metrics,
			size:         *entries.Contents[i].Size,
			name:         relativepath,
			lastModified: *entries.Contents[i].LastModified,
		})
	}

	return fileInfo, nil
}

func (f *fileSystem) ChDir(_ string) error {
	return errors.New("ChDir not implemented yet")
}

// Getwd returns the absolute path of the file on S3.
func (f *fileSystem) Getwd() (string, error) {
	status := "SUCCESS"

	location := path.Join(string(filepath.Separator), f.config.BucketName)
	f.sendOperationStats(&FileLog{Operation: "ChDir", Location: location, Status: &status}, time.Now())

	return location, nil
}

func (f *fileSystem) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
