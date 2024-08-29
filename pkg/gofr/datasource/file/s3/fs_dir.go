package s3

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

// Mkdir creates a directory and any necessary parent directories in the S3 bucket.
//
// This method creates a pseudo-directory in the S3 bucket by putting objects with the specified path prefixes.
// Since S3 uses a flat storage structure, directories are represented by object keys with trailing slashes.
// The method processes the path segments and ensures that each segment (directory) exists in S3.
func (f *fileSystem) Mkdir(name string, perm os.FileMode) error {
	var msg string
	st := "ERROR"
	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "MKDIR",
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
			msg = fmt.Sprintf("failed to create directory %q on s3: %v", currentdir, err)
			return err
		}
	}

	st = "SUCCESS"
	msg = fmt.Sprintf("Directories on path %q created successfully", name)

	f.logger.Logf("Created directories on path %q", name)
	return nil
}

// MkDirAll creates directories in the S3 bucket.
//
// This method calls `MkDir` because AWS S3 buckets do not support traditional directory or file structures; instead, they use a flat structure.
// S3 treats paths as part of object keys, so creating a directory is functionally equivalent to creating an object with a specific prefix.
func (f *fileSystem) MkdirAll(name string, perm os.FileMode) error {
	return f.Mkdir(name, perm)
}

// RemoveAll deletes a directory and all its contents from the S3 bucket.
//
// This method removes a directory and all objects within it from the S3 bucket. It only supports deleting directories
// and will return an error if a file path (as indicated by a file extension) is provided. The method lists all objects
// under the specified directory prefix and deletes them in a single batch operation.
func (f *fileSystem) RemoveAll(name string) error {
	var msg string
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "REMOVEALL",
		Location:  location,
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	if path.Ext(name) != "" {
		f.logger.Errorf("RemoveAll supports deleting directories only. Use Remove instead.")
		return errors.New("invalid argument type. Enter a valid directory name")
	}

	res, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(name + "/"),
	})

	if err != nil {
		msg = fmt.Sprintf("Error retrieving objects: %v", err)
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
	msg = fmt.Sprintf("Directory with path %q, deleted successfully", name)

	f.logger.Logf("Directory %s deleted.", name)
	return nil
}

func getRelativepath(key, filePath string) string {
	relativepath := strings.TrimPrefix(key, filePath)
	oneLevelDeepPathIndex := strings.Index(relativepath, string(filepath.Separator))

	if oneLevelDeepPathIndex != -1 {
		relativepath = relativepath[:oneLevelDeepPathIndex+1]
	}

	return relativepath
}

// ReadDir lists the files and directories within the specified directory in the S3 bucket.
//
// This method retrieves and returns information about the files and directories located under the specified path
// within the S3 bucket. It uses the provided directory name to construct the S3 prefix for listing objects.
// It returns a slice of `file_interface.FileInfo` representing the files and directories found. If the directory name is
// ".", it lists the contents at the root of the bucket.
// Note:
//   - Directories are represented by the prefixes of the file keys in S3, and this method retrieves file entries
//     only one level deep from the specified directory.
func (f *fileSystem) ReadDir(name string) ([]file_interface.FileInfo, error) {
	var filePath, msg string
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "READDIR",
		Location:  location,
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	filePath = name + string(filepath.Separator)

	if name == "." {
		filePath = ""
	}

	entries, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(filePath),
	})

	if err != nil {
		msg = fmt.Sprintf("Error retrieving objects: %v", err)
		return nil, err
	}

	var fileInfo []file_interface.FileInfo

	for i := range entries.Contents {
		if i == 0 {
			continue
		}

		relativepath := getRelativepath(*entries.Contents[i].Key, filePath)

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
			name:         path.Join(f.config.BucketName, *entries.Contents[i].Key),
			lastModified: *entries.Contents[i].LastModified,
		})
	}

	st = "SUCCESS"
	msg = fmt.Sprintf("Directory/Files in directory with path %q retrived successfully", name)

	f.logger.Logf("Reading directory/files from S3 at path %q successful.", name)
	return fileInfo, nil
}

// ChDir is not supported in S3 as the bucket is constant and the filesystem requires a full path relative to the selected bucket.
//
// This method attempts to change the current directory, but S3 does not support directory changes due to its flat file structure.
// The bucket is constant and fixed, so directory operations are not applicable.
func (f *fileSystem) ChDir(_ string) error {
	st := "ERROR"

	location := path.Join(string(filepath.Separator), f.config.BucketName)

	defer f.sendOperationStats(&FileLog{
		Operation: "CHDIR",
		Location:  location,
		Status:    &st,
		Message:   aws.String("Changing directory not supported"),
	}, time.Now())

	return errors.New("s3 does not support changing directories due to flat file structure")
}

// Getwd returns the currently set bucket on S3.
//
// This method retrieves the name of the bucket that is currently set for S3 operations.
func (f *fileSystem) Getwd() (string, error) {
	status := "SUCCESS"

	location := path.Join(string(filepath.Separator), f.config.BucketName)
	f.sendOperationStats(&FileLog{Operation: "GETWD", Location: location, Status: &status}, time.Now())

	return location, nil
}

// renameDirectory renames a directory by copying all its contents to a new path and then deleting the old path.
//
// This method handles the process of renaming a directory in an S3 bucket. It first lists all objects under the old
// directory path, copies each object to the new directory path, and then deletes the old directory and its contents.
func (f *fileSystem) renameDirectory(st *string, msg *string, oldPath, newPath string) error {
	entries, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(oldPath + "/"),
	})

	if err != nil {
		f.logger.Errorf("Error while listing objects: %v", err)
		return err
	}

	// copying objects to new path
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

// sendOperationStats logs the FileLog of any file operations performed in S3.
func (f *fileSystem) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
