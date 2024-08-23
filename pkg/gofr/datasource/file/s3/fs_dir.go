package s3

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"os"
	"path"
	"time"
)

func (*fileSystem) Mkdir(name string, perm os.FileMode) error {
	return nil

}

func (*fileSystem) MkdirAll(name string, perm os.FileMode) error {
	return nil
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
		Bucket: aws.String(f.config.BucketName + "/"),
		Key:    aws.String(name),
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

func (f *fileSystem) ReadDir(name string) error {
	//filePath := path.Join(f.config.remoteDir, name)

	_, _ = f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String("my-bucket"),
	})
	return nil

}

func (f *fileSystem) ChDir(newpath string) error {
	status := "ERROR"
	f.sendOperationStats(&FileLog{Operation: "ChDir", Location: f.remoteDir, Status: &status}, time.Now())
	f.remoteDir = path.Join(f.remoteDir, newpath)

	f.logger.Logf("Current Working Directory : %s", f.remoteDir)
	return nil
}

func (f *fileSystem) Getwd() string {
	status := "SUCCESS"
	f.sendOperationStats(&FileLog{Operation: "ChDir", Location: f.remoteDir, Status: &status}, time.Now())
	return f.remoteDir
}

func (f *fileSystem) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
