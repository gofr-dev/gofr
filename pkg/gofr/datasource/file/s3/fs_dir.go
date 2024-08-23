package s3

import (
	"context"
	"errors"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

func (f *fileSystem) Mkdir(name string, perm os.FileMode) error {
	filePath := path.Join(f.remoteDir, name)

	_, err := f.conn.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(f.config.BucketName),
		Key:    aws.String(filePath + "/"),
	})

	if err != nil {
		return err
	}

	return nil

}

func (f *fileSystem) MkdirAll(name string, perm os.FileMode) error {
	return f.Mkdir(name, perm)
}

func (f *fileSystem) RemoveAll(name string) error {
	//var msg string
	//st := "ERROR"

	//defer f.sendOperationStats(&FileLog{Operation: "RemoveAll", Location: f.remoteDir, Status: &st, Message: &msg}, time.Now())
	if path.Ext(name) != "" {
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

	//st = "SUCCESS"
	//msg = "Directory deletion on S3 successfull."
	//f.logger.Logf("Directory %s deleted.", name)
	return nil
}

func (f *fileSystem) ReadDir(name string) ([]file_interface.FileInfo, error) {
	if path.Ext(name) != "" {
		f.logger.Errorf("ReadDir supports reading directories and its contents only. Use Read instead.")
		return nil, errors.New("invalid argument type. Enter a valid directory name")
	}

	filePath := path.Join(f.remoteDir, name)

	res, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(filePath + "/"),
	})

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (f *fileSystem) ChDir(newpath string) error {
	//status := "ERROR"
	//f.sendOperationStats(&FileLog{Operation: "ChDir", Location: f.remoteDir, Status: &status}, time.Now())
	f.remoteDir = path.Join(f.remoteDir, newpath)

	//f.logger.Logf("Current Working Directory : %s", f.remoteDir)
	// status= "SUCCESS"
	return nil
}

// Getwd returns the absolute path of the file on S3.
func (f *fileSystem) Getwd() string {
	//status := "SUCCESS"
	//f.sendOperationStats(&FileLog{Operation: "ChDir", Location: f.remoteDir, Status: &status}, time.Now())
	return "/" + path.Join(f.config.BucketName, f.remoteDir)
}

func (f *fileSystem) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
