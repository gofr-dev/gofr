package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

var (
	// ErrEmptyDirectoryName is returned when attempting to create a directory with an empty name.
	ErrEmptyDirectoryName = errors.New("directory name cannot be empty")

	// ErrChDirNotSupported is returned when changing directory is attempted on cloud storage.
	ErrChDirNotSupported = errors.New("changing directory is not supported in cloud storage")
)

// CommonFileSystem provides shared implementations of FileSystem operations.
// Providers (GCS, S3, FTP, SFTP) embed this struct to inherit common directory operations,
// metadata handling, and observability patterns.
//
// Providers override only storage-specific methods like Create, Open, Remove, Rename.
//
// Note: This works with StorageProvider interface for cloud operations.
// For reading JSON/CSV files from cloud storage, providers should use:
//   - file.NewTextReader(reader) for text/CSV files
//   - file.NewJSONReader(reader) for JSON files
//
// These are separate from the local filesystem's ReadAll() implementation.
type CommonFileSystem struct {
	Provider StorageProvider   // Underlying storage implementation
	Location string            // Bucket name or connection identifier (e.g., "my-bucket", "ftp://host")
	Logger   datasource.Logger // Logger for operation tracking
	Metrics  StorageMetrics    // Metrics for observability
}

// Mkdir creates a directory in cloud storage by creating a zero-byte object with "/" suffix.
// This follows cloud storage conventions where directories are represented as special markers.
func (c *CommonFileSystem) Mkdir(name string, _ os.FileMode) error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.observe(OpMkdir, startTime, &st, &msg)

	if name == "" {
		msg = "directory name cannot be empty"
		return ErrEmptyDirectoryName
	}

	ctx := context.Background()

	// Ensure directory marker ends with "/"
	objName := name
	if !strings.HasSuffix(objName, "/") {
		objName += "/"
	}

	// Create empty object to represent directory
	writer := c.Provider.NewWriter(ctx, objName)
	defer writer.Close()

	// Write minimal content for directory marker
	_, err := writer.Write([]byte(""))
	if err != nil {
		msg = fmt.Sprintf("failed to write directory marker: %v", err)
		return err
	}

	if err := writer.Close(); err != nil {
		msg = fmt.Sprintf("failed to close writer: %v", err)
		return err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Directory %q created successfully", name)

	return nil
}

// MkdirAll creates nested directories by recursively calling Mkdir for each path component.
// Example: "a/b/c" creates "a/", "a/b/", and "a/b/c/".
func (c *CommonFileSystem) MkdirAll(dirPath string, perm os.FileMode) error {
	cleaned := strings.Trim(dirPath, "/")
	if cleaned == "" {
		return nil
	}

	dirs := strings.Split(cleaned, "/")

	var currentPath string

	for _, dir := range dirs {
		currentPath = path.Join(currentPath, dir)
		err := c.Mkdir(currentPath, perm)

		// Ignore "already exists" errors (idempotent operation)
		if err != nil && !IsAlreadyExistsError(err) {
			return err
		}
	}

	return nil
}

// RemoveAll deletes a directory and all its contents by listing all objects with the prefix
// and deleting them individually.
func (c *CommonFileSystem) RemoveAll(dirPath string) error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.observe(OpRemoveAll, startTime, &st, &msg)

	ctx := context.Background()

	// List all objects under this directory
	objects, err := c.Provider.ListObjects(ctx, dirPath)
	if err != nil {
		msg = fmt.Sprintf("failed to list objects: %v", err)
		return err
	}

	// Delete each object
	for _, obj := range objects {
		if err := c.Provider.DeleteObject(ctx, obj); err != nil {
			msg = fmt.Sprintf("failed to delete %q: %v", obj, err)
			return err
		}
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Directory %q and all contents deleted successfully", dirPath)

	return nil
}

// ReadDir lists the contents of a directory, returning both subdirectories (prefixes) and files (objects).
func (c *CommonFileSystem) ReadDir(dir string) ([]FileInfo, error) {
	var msg string

	st := StatusError

	startTime := time.Now()

	defer c.observe(OpReadDir, startTime, &st, &msg)

	ctx := context.Background()

	// List with delimiter to get immediate children only
	objects, prefixes, err := c.Provider.ListDir(ctx, dir)
	if err != nil {
		msg = fmt.Sprintf("failed to list directory: %v", err)
		return nil, err
	}

	fileInfos := make([]FileInfo, 0, len(prefixes)+len(objects))

	// Add subdirectories (prefixes represent nested directories)
	for _, p := range prefixes {
		trimmedName := strings.TrimSuffix(p, "/")
		dirName := path.Base(trimmedName)

		fileInfos = append(fileInfos, &CommonFile{
			name:  dirName,
			isDir: true,
		})
	}

	// Add files (objects)
	for _, o := range objects {
		// Skip directory markers themselves
		if strings.HasSuffix(o.Name, "/") {
			continue
		}

		fileInfos = append(fileInfos, &CommonFile{
			name:         path.Base(o.Name),
			size:         o.Size,
			contentType:  o.ContentType,
			lastModified: o.LastModified,
			isDir:        o.IsDir,
		})
	}

	st = StatusSuccess
	msg = fmt.Sprintf("ReadDir %q successful (%d items)", dir, len(fileInfos))

	return fileInfos, nil
}

// Stat returns file/directory metadata by querying the storage provider.
func (c *CommonFileSystem) Stat(name string) (FileInfo, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.observe(OpStat, startTime, &st, &msg)

	ctx := context.Background()

	// Try to get object metadata
	info, err := c.Provider.StatObject(ctx, name)
	if err == nil {
		st = StatusSuccess
		msg = fmt.Sprintf("Stat %q successful", name)

		return &CommonFile{
			name:         name,
			size:         info.Size,
			contentType:  info.ContentType,
			lastModified: info.LastModified,
			isDir:        info.IsDir,
		}, nil
	}

	// If not found as file, check if it's a directory by listing with prefix
	prefix := name
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	objects, _, listErr := c.Provider.ListDir(ctx, prefix)
	if listErr != nil {
		msg = fmt.Sprintf("failed to stat %q: %v", name, listErr)
		return nil, listErr
	}

	if len(objects) > 0 {
		st = StatusSuccess
		msg = fmt.Sprintf("Stat %q successful (directory)", name)

		return &CommonFile{
			name:         name,
			size:         0,
			contentType:  "application/x-directory",
			lastModified: objects[0].LastModified,
			isDir:        true,
		}, nil
	}

	msg = fmt.Sprintf("file or directory %q not found", name)

	return nil, ErrFileNotFound
}

// ChDir is not supported for cloud storage (no concept of "current directory").
func (c *CommonFileSystem) ChDir(_ string) error {
	st := StatusError

	msg := "ChDir not supported in cloud storage"
	startTime := time.Now()

	defer c.observe(OpChDir, startTime, &st, &msg)

	return ErrChDirNotSupported
}

// Getwd returns the configured location (bucket name or connection identifier).
func (c *CommonFileSystem) Getwd() (string, error) {
	st := StatusSuccess
	msg := "Returning location"
	startTime := time.Now()

	defer c.observe(OpGetwd, startTime, &st, &msg)

	return c.Location, nil
}

// observe is a helper method to centralize observability for all operations.
func (c *CommonFileSystem) observe(operation string, startTime time.Time, status, message *string) {
	ObserveOperation(&OperationObservability{
		Context:   context.Background(),
		Logger:    c.Logger,
		Metrics:   c.Metrics,
		Operation: operation,
		Location:  c.Location,
		Provider:  "Common", // Providers can override this in their own observe methods
		StartTime: startTime,
		Status:    status,
		Message:   message,
	})
}
