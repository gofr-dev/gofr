package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

var (
	// ErrEmptyDirectoryName is returned when attempting to create a directory with an empty name.
	errEmptyDirectoryName = errors.New("directory name cannot be empty")

	// ErrChDirNotSupported is returned when changing directory is attempted on cloud storage.
	errChDirNotSupported = errors.New("changing directory is not supported in cloud storage")

	errUnsupportedFlags = errors.New("unsupported flag combination for OpenFile")

	errProviderNil = errors.New("storage provider is not configured")
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

	registerHistogram sync.Once
	connected         bool
	disableRetry      bool
}

// Connect calls the provider's Connect and performs common bookkeeping (metrics / logs / observe).
func (c *CommonFileSystem) Connect(ctx context.Context) error {
	start := time.Now()
	st := StatusError
	msg := ""

	defer c.Observe(OpConnect, start, &st, &msg)

	c.registerHistogram.Do(func() {
		if c.Metrics != nil {
			c.Metrics.NewHistogram(AppFileStats, "App File Stats - duration of file operations",
				DefaultHistogramBuckets()...)
		}
	})

	if c.Provider == nil {
		return errProviderNil
	}

	// already connected fast-path
	if c.connected {
		st = StatusSuccess
		msg = "already connected"

		return nil
	}

	// delegate provider-specific connect
	if err := c.Provider.Connect(ctx); err != nil {
		return err
	}

	// success bookkeeping
	c.connected = true
	st = StatusSuccess
	msg = "connected"

	if c.Logger != nil {
		c.Logger.Infof("connected to %s", c.Location)
	}

	return nil
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

// Mkdir creates a directory in cloud storage by creating a zero-byte object with "/" suffix.
// This follows cloud storage conventions where directories are represented as special markers.
func (c *CommonFileSystem) Mkdir(name string, _ os.FileMode) error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.Observe(OpMkdir, startTime, &st, &msg)

	if name == "" {
		msg = "directory name cannot be empty"

		return errEmptyDirectoryName
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
		return errWriterNil
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
// MkdirAll creates nested directories by recursively calling Mkdir for each path component.
func (c *CommonFileSystem) MkdirAll(dirPath string, perm os.FileMode) error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.Observe(OpMkdirAll, startTime, &st, &msg)

	if dirPath == "" {
		msg = "directory path cannot be empty"

		return errEmptyDirectoryName
	}

	// Split and filter path components
	components := c.getPathComponents(dirPath)

	// Create each directory in the path
	currentPath := ""
	for _, component := range components {
		currentPath = path.Join(currentPath, component)

		if err := c.Mkdir(currentPath, perm); err != nil && !os.IsExist(err) {
			msg = fmt.Sprintf("failed to create %q: %v", currentPath, err)

			return err
		}
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Created directory %q successfully", dirPath)

	return nil
}

// getPathComponents splits a path and filters out empty, ".", and ".." components.
func (*CommonFileSystem) getPathComponents(dirPath string) []string {
	parts := strings.Split(strings.Trim(dirPath, "/"), "/")
	components := make([]string, 0, len(parts))

	for _, part := range parts {
		if part != "" && part != "." && part != ".." {
			components = append(components, part)
		}
	}

	return components
}

// RemoveAll deletes a directory and all its contents by listing all objects with the prefix
// and deleting them individually.
func (c *CommonFileSystem) RemoveAll(dirPath string) error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.Observe(OpRemoveAll, startTime, &st, &msg)

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

	defer c.Observe(OpReadDir, startTime, &st, &msg)

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

	defer c.Observe(OpStat, startTime, &st, &msg)

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

	defer c.Observe(OpChDir, startTime, &st, &msg)

	return errChDirNotSupported
}

// Getwd returns the configured location (bucket name or connection identifier).
func (c *CommonFileSystem) Getwd() (string, error) {
	st := StatusSuccess
	msg := "Returning location"
	startTime := time.Now()

	defer c.Observe(OpGetwd, startTime, &st, &msg)

	return c.Location, nil
}

// ============= File Operations (NEW) =============

// Create creates a new file for writing.
func (c *CommonFileSystem) Create(name string) (File, error) {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.Observe(OpCreate, startTime, &st, &msg)

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

	defer c.Observe(OpOpen, startTime, &st, &msg)

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
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.Observe(OpOpenFile, startTime, &st, &msg)

	// Try different strategies based on flags
	file, err := c.tryOpenStrategies(name, flag, perm)
	if err != nil {
		msg = fmt.Sprintf("failed to open %q: %v", name, err)
		return nil, err
	}

	st = StatusSuccess
	msg = fmt.Sprintf("Opened %q with flags %d", name, flag)

	return file, nil
}

// tryOpenStrategies attempts different strategies to open a file based on flags.
func (c *CommonFileSystem) tryOpenStrategies(name string, flag int, perm os.FileMode) (File, error) {
	// Strategy 1: Try local filesystem for O_RDWR
	if flag&os.O_RDWR != 0 {
		if localFile, err := c.tryLocalFileOpen(name, flag, perm); err == nil {
			return localFile, nil
		}
	}

	// Strategy 2: Handle read-only flags
	if flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		return c.Open(name)
	}

	// Strategy 3: Handle write flags
	return c.handleWriteFlags(name, flag)
}

// tryLocalFileOpen attempts to open a file using os.OpenFile (for local filesystem).
func (c *CommonFileSystem) tryLocalFileOpen(name string, flag int, perm os.FileMode) (File, error) {
	osFile, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}

	info, err := osFile.Stat()
	if err != nil {
		osFile.Close()

		return nil, err
	}

	return &CommonFile{
		name:     name,
		body:     osFile,
		writer:   osFile,
		size:     info.Size(),
		logger:   c.Logger,
		metrics:  c.Metrics,
		location: c.Location,
	}, nil
}

// handleWriteFlags handles O_WRONLY, O_APPEND, O_CREATE, O_TRUNC flags.
func (c *CommonFileSystem) handleWriteFlags(name string, flag int) (File, error) {
	// O_CREATE or O_TRUNC: create new file
	if flag&(os.O_CREATE|os.O_TRUNC) != 0 {
		return c.Create(name)
	}

	// O_APPEND: open existing for append
	if flag&os.O_APPEND != 0 {
		ctx := context.Background()

		writer := c.Provider.NewWriter(ctx, name)
		if writer == nil {
			return nil, errWriterNil
		}

		return &CommonFile{
			name:     name,
			writer:   writer,
			logger:   c.Logger,
			metrics:  c.Metrics,
			location: c.Location,
		}, nil
	}

	return nil, errUnsupportedFlags
}

// Remove deletes a file.
func (c *CommonFileSystem) Remove(name string) error {
	var msg string

	st := StatusError
	startTime := time.Now()

	defer c.Observe(OpRemove, startTime, &st, &msg)

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

	defer c.Observe(OpRename, startTime, &st, &msg)

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

// Observe is a helper method to centralize observability for all operations.
func (c *CommonFileSystem) Observe(operation string, startTime time.Time, status, message *string) {
	ObserveOperation(&OperationObservability{
		Context:   context.Background(),
		Logger:    c.Logger,
		Metrics:   c.Metrics,
		Operation: operation,
		Location:  c.Location,
		Provider:  "Common", // Providers can override this in their own Observe methods
		StartTime: startTime,
		Status:    status,
		Message:   message,
	})
}

func (c *CommonFileSystem) IsConnected() bool {
	return c.connected
}

// SetDisableRetry enables or disables background retry behavior.
func (c *CommonFileSystem) SetDisableRetry(disable bool) {
	c.disableRetry = disable
}

// IsRetryDisabled returns true if background retry is disabled.
func (c *CommonFileSystem) IsRetryDisabled() bool {
	return c.disableRetry
}

// SetConnected manually sets the connected state (for testing).
func (c *CommonFileSystem) SetConnected(connected bool) {
	c.connected = connected
}
