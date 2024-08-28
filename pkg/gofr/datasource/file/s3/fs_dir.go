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

// MkDir At root level creating directory.then creating files.
func (f *fileSystem) Mkdir(name string, perm os.FileMode) error {
	var msg string
	st := "ERROR"
	location := path.Join(f.config.BucketName, f.remoteDir)

	defer f.sendOperationStats(&FileLog{
		Operation: "MkDir",
		Location:  location,
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	// Currently we are handling the case of general-purpose S3 buckets only.
	bucketName := strings.Split(name, string(filepath.Separator))[0]
	pathLength := len(strings.Split(name, string(filepath.Separator)))
	// checks if the usecase was operated on root directory or not, to reset the
	// bucketName back after creating the required file on specified path
	var rootDir bool

	if f.config.BucketName == "" {
		_, err := f.conn.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				Location: &types.LocationInfo{
					Name: aws.String(f.config.Region),
				},
			},
		})

		if err != nil {
			var bne *types.BucketAlreadyExists
			var boe *types.BucketAlreadyOwnedByYou
			if errors.As(err, &bne) && pathLength == 1 {
				f.logger.Logf("Bucket %s already exists", name)
			} else if errors.As(err, &boe) && pathLength == 1 {
				f.logger.Logf("Bucket %s already owned by you", name)
			} else {
				return err
			}
		}

		// subdirectories & file need to be created
		if pathLength != 1 {
			f.config.BucketName = bucketName
			rootDir = true
		}
	}

	filePath := path.Join(f.remoteDir, name)

	_, err := f.conn.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(filePath + "/"),
	})

	if err != nil {
		return err
	}
	if rootDir {
		f.config.BucketName = ""
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

	defer f.sendOperationStats(&FileLog{Operation: "RemoveAll", Location: f.remoteDir, Status: &st, Message: &msg}, time.Now())
	if path.Ext(name) != "" || name[0] == 'b' {
		f.logger.Errorf("RemoveAll supports deleting directories and its contents only. Use Remove instead.")
		return errors.New("invalid argument type. Enter a valid directory name")
	}

	_, err := f.conn.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(name + "/"),
	})

	if err != nil {
		//f.logger.Errorf("Error while deleting directory: %v", err)
		return err
	}

	st = "SUCCESS"
	msg = "Directory deletion on S3 successfull."
	f.logger.Logf("Directory %s deleted.", name)
	return nil
}

func (f *fileSystem) ReadDir(name string) ([]file_interface.FileInfo, error) {
	if path.Ext(name) != "" || name[0] == '1' {
		f.logger.Errorf("ReadDir supports reading directories contents only. Use Read instead.")
		return nil, errors.New("invalid argument type. Enter a valid directory name")
	}

	filePath := path.Join(f.remoteDir, name)

	entries, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(filePath + "/"),
	})

	if err != nil {
		return nil, err
	}

	var fileInfo []file_interface.FileInfo

	for i := range entries.Contents {
		if i == 0 {
			continue
		}
		fileInfo = append(fileInfo, &file{
			conn:         f.conn,
			logger:       f.logger,
			metrics:      f.metrics,
			size:         *entries.Contents[i].Size,
			name:         *entries.Contents[i].Key,
			lastModified: *entries.Contents[i].LastModified,
		})
	}

	return fileInfo, nil
}

func (f *fileSystem) ChDir(newpath string) error {
	status := "ERROR"
	defer f.sendOperationStats(&FileLog{Operation: "ChDir", Location: f.remoteDir, Status: &status}, time.Now())

	previous := f.remoteDir

	if f.config.BucketName == "" {
		f.config.BucketName = strings.Split(newpath, string(filepath.Separator))[0]
		index := strings.Index(newpath, string(filepath.Separator))
		if index != -1 {
			f.remoteDir = newpath[index+1:]
		}
	} else {
		f.remoteDir = path.Join(f.remoteDir, newpath)
	}

	res, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(f.remoteDir + "/"),
	})

	if err != nil {
		return err
	}

	if len(res.Contents) == 0 {
		status = "ERROR"
		f.remoteDir = previous
		return errors.New("Path does not exist")
	}

	f.logger.Logf("Current Working Directory : %s", f.remoteDir)
	status = "SUCCESS"
	return nil
}

// Getwd returns the absolute path of the file on S3.
func (f *fileSystem) Getwd() (string, error) {
	status := "SUCCESS"
	f.sendOperationStats(&FileLog{Operation: "ChDir", Location: f.remoteDir, Status: &status}, time.Now())

	return "/" + path.Join(f.config.BucketName, f.remoteDir), nil
}

func (f *fileSystem) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
