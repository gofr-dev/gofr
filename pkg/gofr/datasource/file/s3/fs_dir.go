package s3

import (
	"context"
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

func (*fileSystem) RemoveAll(name string) error {
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
