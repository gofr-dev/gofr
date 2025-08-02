package gcs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	file "gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/googleapi"
)

var (
	errEmptyDirectoryName = errors.New("directory name cannot be empty")
	errCHNDIRNotSupported = errors.New("changing directory is not supported in GCS")
)

func getBucketName(filePath string) string {
	return strings.Split(filePath, string(filepath.Separator))[0]
}

func getLocation(bucket string) string {
	return path.Join(string(filepath.Separator), bucket)
}

func (f *FileSystem) Mkdir(name string, _ os.FileMode) error {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "MKDIR",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	if name == "" {
		msg = "directory name cannot be empty"
		f.logger.Errorf(msg)

		return errEmptyDirectoryName
	}

	ctx := context.TODO()
	objName := name + "/"

	writer := f.conn.NewWriter(ctx, objName)
	defer writer.Close()

	_, err := writer.Write([]byte("dir"))
	if err != nil {
		if err != nil {
			msg = fmt.Sprintf("failed to create directory %q on GCS: %v", objName, err)
			f.logger.Errorf(msg)

			return err
		}
	}

	st = statusSuccess

	msg = fmt.Sprintf("Directories on path %q created successfully", name)

	f.logger.Logf("Created directories on path %q", name)

	return err
}

func (f *FileSystem) MkdirAll(dirPath string, perm os.FileMode) error {
	cleaned := strings.Trim(dirPath, "/")
	if cleaned == "" {
		return nil
	}

	dirs := strings.Split(cleaned, "/")

	var currentPath string

	for _, dir := range dirs {
		currentPath = pathJoin(currentPath, dir)
		err := f.Mkdir(currentPath, perm)

		if err != nil && !isAlreadyExistsError(err) {
			return err
		}
	}

	return nil
}
func pathJoin(parts ...string) string {
	return path.Join(parts...)
}

func isAlreadyExistsError(err error) bool {
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code == 409 || gErr.Code == 412
	}

	// Fallback check
	return strings.Contains(err.Error(), "already exists")
}

func (f *FileSystem) RemoveAll(dirPath string) error {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "REMOVEALL",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	ctx := context.TODO()

	objects, err := f.conn.ListObjects(ctx, dirPath)
	if err != nil {
		msg = fmt.Sprintf("Error retrieving objects: %v", err)
		return err
	}

	for _, obj := range objects {
		if err := f.conn.DeleteObject(ctx, obj); err != nil {
			f.logger.Errorf("Error while deleting directory: %v", err)
			return err
		}
	}

	st = statusSuccess

	msg = fmt.Sprintf("Directory with path %q, deleted successfully", dirPath)

	f.logger.Logf("Directory %s deleted.", dirPath)

	return nil
}

func (f *FileSystem) ReadDir(dir string) ([]file.FileInfo, error) {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "READDIR",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	ctx := context.TODO()

	objects, prefixes, err := f.conn.ListDir(ctx, dir)
	if err != nil {
		msg = fmt.Sprintf("Error retrieving objects: %v", err)
		f.logger.Logf(msg)

		return nil, err
	}

	fileinfo := make([]file.FileInfo, 0, len(prefixes)+len(objects))

	for _, p := range prefixes {
		trimmedName := strings.TrimSuffix(p, "/")
		dirName := path.Base(trimmedName)
		fileinfo = append(fileinfo, &File{
			name:    dirName,
			isDir:   true,
			logger:  f.logger,
			metrics: f.metrics,
		})
	}

	for _, o := range objects {
		fileinfo = append(fileinfo, &File{
			name:         path.Base(o.Name),
			size:         o.Size,
			lastModified: o.Updated,
			isDir:        false,
			logger:       f.logger,
			metrics:      f.metrics,
		})
	}

	st = statusSuccess
	msg = fmt.Sprintf("Directory/Files in directory with path %q retrieved successfully", dir)

	f.logger.Logf("Reading directory/files from GCS at path %q successful.", dir)

	return fileinfo, nil
}

func (f *FileSystem) ChDir(_ string) error {
	const op = "CHDIR"

	st := statusErr

	var msg = "Changing directory not supported"

	defer f.sendOperationStats(&FileLog{
		Operation: op,
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	f.logger.Errorf("%s: not supported in GCS", op)

	return errCHNDIRNotSupported
}
func (f *FileSystem) Getwd() (string, error) {
	const op = "GETWD"

	st := statusSuccess

	start := time.Now()

	var msg = "Returning simulated root directory"

	defer f.sendOperationStats(&FileLog{
		Operation: op,
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, start)

	return getLocation(f.config.BucketName), nil
}
func (f *FileSystem) Stat(name string) (file.FileInfo, error) {
	var msg string
	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "STAT",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	ctx := context.TODO()

	// Try to stat the object (file)
	attr, err := f.conn.StatObject(ctx, name)
	if err == nil {
		st = statusSuccess
		msg = fmt.Sprintf("File with path %q info retrieved successfully", name)

		return &File{
			name:         name,
			logger:       f.logger,
			metrics:      f.metrics,
			size:         attr.Size,
			contentType:  attr.ContentType,
			lastModified: attr.Updated,
		}, nil
	}

	// If not found, check if it's a "directory" by listing with prefix
	if errors.Is(err, storage.ErrObjectNotExist) {
		// Ensure the name ends with slash for directories
		prefix := name
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}

		objs, _, listErr := f.conn.ListDir(ctx, prefix)
		if listErr != nil {
			f.logger.Errorf("Error checking directory prefix: %v", listErr)
			return nil, listErr
		}

		if len(objs) > 0 {
			st = statusSuccess
			msg = fmt.Sprintf("Directory with path %q info retrieved successfully", name)

			return &File{
				name:         name,
				logger:       f.logger,
				metrics:      f.metrics,
				size:         0,
				contentType:  "application/x-directory",
				lastModified: objs[0].Updated,
			}, nil
		}
	}

	f.logger.Errorf("Error returning file or directory info: %v", err)

	return nil, err
}

func (f *FileSystem) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
