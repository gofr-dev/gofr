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

	ErrProviderNotSet = errors.New("file provider not set")
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

// UseLogger sets the logger for the CommonFileSystem.
func (c *CommonFileSystem) UseLogger(logger any) {
	if l, ok := logger.(datasource.Logger); ok {
		c.Logger = l
	}
}

// UseMetrics sets the metrics interface for the CommonFileSystem.
func (c *CommonFileSystem) UseMetrics(metrics any) {
	if m, ok := metrics.(StorageMetrics); ok {
		c.Metrics = m
	}
}

// Connect is a no-op for CommonFileSystem since StorageProvider is stateless.
// The actual connection logic is handled by the provider wrapper (e.g., gcs.FileSystem).
func (c *CommonFileSystem) Connect() {
	// No-op: StorageProvider doesn't have a Connect method
	// Connection is handled by the wrapper (gcs.FileSystem, s3.FileSystem, etc.)
	if c.Logger != nil {
		c.Logger.Debugf("CommonFileSystem initialized for location: %s", c.Location)
	}
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

	if _, err := c.Provider.StatObject(ctx, objName); err == nil {
		st = StatusSuccess
		msg = fmt.Sprintf("Directory %q already exists", name)

		return nil
	}

	// Create empty object to represent directory
	writer := c.Provider.NewWriter(ctx, objName)
	if writer == nil {
		msg = "failed to create writer"

		return fmt.Errorf("NewWriter returned nil")
	}
	defer writer.Close()

	// Write minimal content for directory marker
	_, err := writer.Write([]byte(""))
	if err != nil {
		if strings.Contains(err.Error(), "is a directory") {
			st = StatusSuccess
			msg = fmt.Sprintf("Directory %q already exists", name)

			return nil
		}

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
	msg := ""
	st := StatusError
	startTime := time.Now()

	defer c.observe(OpMkdirAll, startTime, &st, &msg)

	// Clean and validate path
	cleaned := strings.Trim(dirPath, "/")
	if cleaned == "" || cleaned == "." {
		st = StatusSuccess
		msg = "Skipped root/current directory"

		return nil
	}

	// Check if already exists (using StatObject for cloud storage)
	ctx := context.Background()
	if _, err := c.Provider.StatObject(ctx, cleaned+"/"); err == nil {
		st = StatusSuccess
		msg = fmt.Sprintf("Directory %q already exists", dirPath)

		return nil
	}

	// Split path into components and filter out "." and ".."
	dirs := strings.Split(cleaned, "/")
	var validDirs []string
	for _, dir := range dirs {
		if dir != "" && dir != "." && dir != ".." {
			validDirs = append(validDirs, dir)
		}
	}

	// Create each directory component
	var currentPath string
	for _, dir := range validDirs {
		currentPath = path.Join(currentPath, dir)
		err := c.Mkdir(currentPath, perm)

		if err != nil && !IsAlreadyExistsError(err) {
			msg = err.Error()

			return err
		}
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Created directory %q successfully", dirPath)
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

// ============= File Operations (NEW) =============

// Create creates a new file for writing.
func (c *CommonFileSystem) Create(name string) (File, error) {
	var msg string
	st := StatusError
	startTime := time.Now()

	defer c.observe(OpCreate, startTime, &st, &msg)

	ctx := context.Background()

	// Create writer
	writer := c.Provider.NewWriter(ctx, name)

	st = StatusSuccess
	msg = fmt.Sprintf("Created %q for writing", name)

	return NewCommonFileWriter(
		c.Provider,
		name,
		writer,
		c.Logger,
		c.Metrics,
		c.Location,
	), nil
}

// Open opens a file for reading.
func (c *CommonFileSystem) Open(name string) (File, error) {
	var msg string
	st := StatusError
	startTime := time.Now()

	defer c.observe(OpOpen, startTime, &st, &msg)

	ctx := context.Background()

	// Get metadata first (efficient HEAD request)
	info, err := c.Provider.StatObject(ctx, name)
	if err != nil {
		msg = fmt.Sprintf("file %q not found: %v", name, err)
		return nil, ErrFileNotFound
	}

	// Create reader
	reader, err := c.Provider.NewReader(ctx, name)
	if err != nil {
		msg = fmt.Sprintf("failed to open %q: %v", name, err)
		return nil, err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Opened %q for reading", name)

	return NewCommonFile(
		c.Provider,
		name,
		info,
		reader,
		c.Logger,
		c.Metrics,
		c.Location,
	), nil
}

// OpenFile opens a file with the specified flags and permissions.
func (c *CommonFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	msg := ""
	st := StatusError
	startTime := time.Now()

	defer c.observe(OpOpenFile, startTime, &st, &msg)

	if c.Provider == nil {
		return nil, ErrProviderNotSet
	}

	ctx := context.Background()

	// Handle different flag combinations
	switch {
	case flag&os.O_RDWR != 0:
		info, err := c.Provider.StatObject(ctx, name)
		if err != nil && flag&os.O_CREATE == 0 {
			msg = fmt.Sprintf("file %q not found: %v", name, err)
			return nil, ErrFileNotFound
		}

		osFile, err := os.OpenFile(name, flag, perm)
		if err != nil {
			msg = fmt.Sprintf("unsupported flags for cloud storage: %d", flag)
			return nil, fmt.Errorf("%s", msg)
		}

		size := int64(0)
		if info != nil {
			size = info.Size
		}

		st = StatusSuccess
		msg = fmt.Sprintf("Opened %q with flags %d", name, flag)

		return &CommonFile{
			name:     name,
			body:     osFile,
			writer:   osFile,
			size:     size,
			logger:   c.Logger,
			metrics:  c.Metrics,
			location: c.Location,
		}, nil

	case flag&os.O_WRONLY != 0 || flag&os.O_CREATE != 0:
		// Write or create mode
		writer := c.Provider.NewWriter(ctx, name)

		info, _ := c.Provider.StatObject(ctx, name)
		size := int64(0)
		if info != nil {
			size = info.Size
		}

		st = StatusSuccess
		msg = fmt.Sprintf("Opened %q for writing", name)

		return &CommonFile{
			name:     name,
			writer:   writer,
			size:     size,
			logger:   c.Logger,
			metrics:  c.Metrics,
			location: c.Location,
		}, nil

	default:
		// Read-only mode (default)
		reader, err := c.Provider.NewReader(ctx, name)
		if err != nil {
			msg = err.Error()
			return nil, err
		}

		info, err := c.Provider.StatObject(ctx, name)
		if err != nil {
			_ = reader.Close()
			msg = err.Error()
			return nil, err
		}

		st = StatusSuccess
		msg = fmt.Sprintf("Opened %q for reading", name)

		return &CommonFile{
			name:         name,
			body:         reader,
			size:         info.Size,
			contentType:  info.ContentType,
			lastModified: info.LastModified,
			isDir:        info.IsDir,
			logger:       c.Logger,
			metrics:      c.Metrics,
			location:     c.Location,
		}, nil
	}
}

// Remove deletes a file.
func (c *CommonFileSystem) Remove(name string) error {
	var msg string
	st := StatusError
	startTime := time.Now()

	defer c.observe(OpRemove, startTime, &st, &msg)

	ctx := context.Background()

	if err := c.Provider.DeleteObject(ctx, name); err != nil {
		msg = fmt.Sprintf("failed to remove %q: %v", name, err)
		return err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Removed %q successfully", name)

	return nil
}

// Rename renames a file (copy + delete).
func (c *CommonFileSystem) Rename(oldname, newname string) error {
	var msg string
	st := StatusError
	startTime := time.Now()

	defer c.observe(OpRename, startTime, &st, &msg)

	ctx := context.Background()

	// Copy to new location
	if err := c.Provider.CopyObject(ctx, oldname, newname); err != nil {
		msg = fmt.Sprintf("failed to copy %q to %q: %v", oldname, newname, err)
		return err
	}

	// Delete old
	if err := c.Provider.DeleteObject(ctx, oldname); err != nil {
		msg = fmt.Sprintf("failed to delete old file %q: %v", oldname, err)
		return err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Renamed %q to %q successfully", oldname, newname)

	return nil
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
