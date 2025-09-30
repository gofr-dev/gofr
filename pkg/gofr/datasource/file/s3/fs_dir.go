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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	file "gofr.dev/pkg/gofr/datasource/file"
)

var (
	ErrOperationNotPermitted = errors.New("operation not permitted")
)

// getBucketName returns the currentS3Bucket.
func getBucketName(filePath string) string {
	return strings.Split(filePath, string(filepath.Separator))[0]
}

// getLocation returns the absolute path of the S3 bucket.
func getLocation(bucket string) string {
	return path.Join(string(filepath.Separator), bucket)
}

// Mkdir creates a directory and any necessary parent directories in the S3 bucket.
//
// This method creates a pseudo-directory in the S3 bucket by putting objects with the specified path prefixes.
// Since S3 uses a flat storage structure, directories are represented by object keys with trailing slashes.
// The method processes the path segments and ensures that each segment (directory) exists in S3.
func (f *FileSystem) Mkdir(name string, _ os.FileMode) error {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "MKDIR",
		Location:  getLocation(f.config.BucketName),
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

	st = statusSuccess
	msg = fmt.Sprintf("Directories on path %q created successfully", name)

	f.logger.Logf("Created directories on path %q", name)

	return nil
}

// MkdirAll creates directories in the S3 bucket.
//
// This method calls `MkDir` because AWS S3 buckets do not support traditional directory or file structures.
// Instead, they use a flat structure.
// S3 treats paths as part of object keys, so creating a directory is functionally equivalent to creating an
// object with a specific prefix.
func (f *FileSystem) MkdirAll(name string, perm os.FileMode) error {
	return f.Mkdir(name, perm)
}

// RemoveAll deletes a directory and all its contents from the S3 bucket.
//
// This method removes a directory and all objects within it from the S3 bucket. It only supports deleting directories
// and will return an error if a file path (as indicated by a file extension) is provided. The method lists all objects
// under the specified directory prefix and deletes them in a single batch operation.
func (f *FileSystem) RemoveAll(name string) error {
	if path.Ext(name) != "" {
		return f.Remove(name)
	}

	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "REMOVEALL",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

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

	st = statusSuccess
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
func (f *FileSystem) ReadDir(name string) ([]file.FileInfo, error) {
	var filePath, msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "READDIR",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	filePath = name + string(filepath.Separator)

	if name == "." {
		filePath = ""
	}

	// TODO: Enhance the implementation to fetch only data that is one level deep.
	// Currently, the system retrieves metadata of all files matching the prefix,
	// which may include files in nested directories. This takes more memory.
	entries, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(filePath),
	})
	if err != nil {
		msg = fmt.Sprintf("Error retrieving objects: %v", err)
		return nil, err
	}

	fileInfo := make([]file.FileInfo, 0)

	for i := range entries.Contents {
		if i == 0 && filePath != "" {
			continue
		}

		relativepath := getRelativepath(*entries.Contents[i].Key, filePath)

		if len(fileInfo) > 0 {
			temp, ok := fileInfo[len(fileInfo)-1].(*S3File)

			if ok && relativepath == path.Base(temp.name)+string(filepath.Separator) {
				continue
			}
		}

		fileInfo = append(fileInfo, &S3File{
			conn:         f.conn,
			logger:       f.logger,
			metrics:      f.metrics,
			size:         *entries.Contents[i].Size,
			name:         f.config.BucketName + string(filepath.Separator) + *entries.Contents[i].Key,
			lastModified: *entries.Contents[i].LastModified,
		})
	}

	st = statusSuccess
	msg = fmt.Sprintf("Directory/Files in directory with path %q retrieved successfully", name)

	f.logger.Logf("Reading directory/files from S3 at path %q successful.", name)

	return fileInfo, nil
}

// ChDir is not supported in S3 as the bucket is constant and the filesystem requires a full path relative to the selected bucket.
//
// This method attempts to change the current directory, but S3 does not support directory changes due to its flat file structure.
// The bucket is constant and fixed, so directory operations are not applicable.
func (f *FileSystem) ChDir(string) error {
	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "CHDIR",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   aws.String("Changing directory not supported"),
	}, time.Now())

	return fmt.Errorf("%w: s3 has a flat file structure", ErrOperationNotPermitted)
}

// Getwd returns the currently set bucket on S3.
//
// This method retrieves the name of the bucket that is currently set for S3 operations.
func (f *FileSystem) Getwd() (string, error) {
	status := statusSuccess

	f.sendOperationStats(&FileLog{Operation: "GETWD", Location: getLocation(f.config.BucketName), Status: &status}, time.Now())

	return getLocation(f.config.BucketName), nil
}

// renameDirectory renames a directory by copying all its contents to a new path and then deleting the old path.
//
// This method handles the process of renaming a directory in an S3 bucket. It first lists all objects under the old
// directory path, copies each object to the new directory path, and then deletes the old directory and its contents.
func (f *FileSystem) renameDirectory(st, msg *string, oldPath, newPath string) error {
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

		_, err = f.conn.CopyObject(context.TODO(), &s3.CopyObjectInput{
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

	*st = statusSuccess
	*msg = fmt.Sprintf("Directory with path %q successfully renamed to %q", oldPath, newPath)

	return nil
}

// Stat retrieves the FileInfo for the specified file or directory in the S3 bucket.
//
// If the provided name has no file extension, it is treated as a directory by default. If the name starts with "0",
// it is interpreted as a binary file rather than a directory, with the "0" prefix removed.
//
// For directories, the method aggregates the sizes of all objects within the directory and returns the latest modified
// time among them. For files, it returns the file's size and last modified time.
func (f *FileSystem) Stat(name string) (file.FileInfo, error) {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "STAT",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	filetype := typeFile

	// Here we assume the user passes "0filePath" in case it wants to get fileinfo about a binary file instead of a directory
	if path.Ext(name) == "" {
		filetype = typeDirectory

		var isBinary bool

		name, isBinary = strings.CutPrefix(name, "0")

		if isBinary {
			filetype = typeFile
		}
	}

	res, err := f.conn.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(f.config.BucketName),
		Prefix: aws.String(name),
	})
	if err != nil {
		f.logger.Errorf("Error returning file info: %v", err)
		return nil, err
	}

	if len(res.Contents) == 0 {
		return nil, nil
	}

	if filetype == typeDirectory {
		var size int64

		var lastModified time.Time

		for i := range res.Contents {
			size += *res.Contents[i].Size

			if res.Contents[i].LastModified.After(lastModified) {
				lastModified = *res.Contents[i].LastModified
			}
		}

		// directory exist and first value gives information about the directory
		st = statusSuccess
		msg = fmt.Sprintf("Directory with path %q info retrieved successfully", name)

		return &S3File{
			conn:         f.conn,
			logger:       f.logger,
			metrics:      f.metrics,
			size:         size,
			contentType:  filetype,
			name:         f.config.BucketName + string(filepath.Separator) + *res.Contents[0].Key,
			lastModified: lastModified,
		}, nil
	}

	return &S3File{
		conn:         f.conn,
		logger:       f.logger,
		metrics:      f.metrics,
		size:         *res.Contents[0].Size,
		name:         f.config.BucketName + string(filepath.Separator) + *res.Contents[0].Key,
		contentType:  filetype,
		lastModified: *res.Contents[0].LastModified,
	}, nil
}

// sendOperationStats logs the FileLog of any file operations performed in S3.
func (f *FileSystem) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
