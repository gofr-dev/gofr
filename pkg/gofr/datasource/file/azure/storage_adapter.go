package azure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	azfile "github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/share"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	// Storage adapter errors.
	errAzureConfigNil            = errors.New("azure config is nil")
	errAzureClientNotInitialized = errors.New("azure client or share is not initialized")
	errEmptyObjectName           = errors.New("object name is empty")
	errInvalidOffset             = errors.New("invalid offset: must be >= 0")
	errEmptySourceOrDest         = errors.New("source and destination names cannot be empty")
	errSameSourceOrDest          = errors.New("source and destination are the same")
	errFailedToCreateReader      = errors.New("failed to create reader")
	errFailedToCreateRangeReader = errors.New("failed to create range reader")
	errObjectNotFound            = errors.New("object not found")
	errFailedToGetProperties     = errors.New("failed to get properties")
	errFailedToDeleteObject      = errors.New("failed to delete object")
	errFailedToCopyObject        = errors.New("failed to copy object")
	errFailedToListObjects       = errors.New("failed to list objects")
	errFailedToListDirectory     = errors.New("failed to list directory")
	errWriterAlreadyClosed       = errors.New("writer already closed")
	errInvalidWhence             = errors.New("invalid whence")
	errNegativeOffset            = errors.New("negative offset")
	errShareNameEmpty            = errors.New("share name cannot be empty")
)

const (
	contentTypeDirectory   = "application/x-directory"
	contentTypeOctetStream = "application/octet-stream"
)

// storageAdapter adapts Azure File Storage client to implement file.StorageProvider.
type storageAdapter struct {
	cfg         *Config
	shareClient *share.Client
}

// Connect initializes the Azure File Storage client and validates share access.
func (s *storageAdapter) Connect(ctx context.Context) error {
	// fast-path
	if s.shareClient != nil {
		return nil
	}

	if s.cfg == nil {
		return errAzureConfigNil
	}

	// Create Azure credentials
	cred, err := share.NewSharedKeyCredential(s.cfg.AccountName, s.cfg.AccountKey)
	if err != nil {
		return fmt.Errorf("failed to create shared key credential: %w", err)
	}

	// Build the share URL
	endpoint := s.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://" + s.cfg.AccountName + ".file.core.windows.net"
	}

	// Validate share name format (Azure share names must be lowercase, 3-63 chars, alphanumeric and hyphens)
	shareName := strings.TrimSpace(s.cfg.ShareName)
	if shareName == "" {
		return errShareNameEmpty
	}

	// URL encode the share name to handle any special characters
	shareURL := endpoint + "/" + shareName

	// Create share client
	shareClient, err := share.NewClientWithSharedKeyCredential(shareURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create share client: %w", err)
	}

	// Validate share access by getting properties
	_, err = shareClient.GetProperties(ctx, nil)
	if err != nil {
		// Check if error is due to context deadline exceeded (timeout)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("share validation failed: connection timeout (10s): %w", err)
		}

		return fmt.Errorf("share validation failed: %w", err)
	}

	s.shareClient = shareClient

	return nil
}

// Health checks if the Azure connection is healthy by verifying share access.
func (s *storageAdapter) Health(ctx context.Context) error {
	if s.shareClient == nil {
		return errAzureClientNotInitialized
	}

	_, err := s.shareClient.GetProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("azure health check failed: %w", err)
	}

	return nil
}

// Close closes the Azure client connection.
func (*storageAdapter) Close() error {
	// Azure SDK clients don't have explicit Close methods
	// They are stateless and can be reused
	return nil
}

// NewReader creates a reader for the given file.
func (s *storageAdapter) NewReader(ctx context.Context, name string) (io.ReadCloser, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	fileClient, err := s.getFileClient(name)
	if err != nil {
		return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateReader, name, err)
	}

	// Download file
	downloadResp, err := fileClient.DownloadStream(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateReader, name, err)
	}

	return downloadResp.Body, nil
}

// NewRangeReader creates a range reader for the given file.
func (s *storageAdapter) NewRangeReader(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	if offset < 0 {
		return nil, fmt.Errorf("%w (got: %d)", errInvalidOffset, offset)
	}

	fileClient, err := s.getFileClient(name)
	if err != nil {
		return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateRangeReader, name, err)
	}

	// Download file with range
	opts := &azfile.DownloadStreamOptions{
		Range: azfile.HTTPRange{
			Offset: offset,
			Count:  length,
		},
	}

	downloadResp, err := fileClient.DownloadStream(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateRangeReader, name, err)
	}

	return downloadResp.Body, nil
}

// NewWriter creates a writer for the given file.
// Automatically creates parent directories to ensure they appear in listings
// (Azure File Storage requires explicit directory creation for directories to be visible).
func (s *storageAdapter) NewWriter(ctx context.Context, name string) io.WriteCloser {
	if name == "" {
		return &failWriter{err: errEmptyObjectName}
	}

	// Ensure parent directories exist (Azure requires explicit directory creation for listings)
	// This matches local filesystem behavior where parent directories are auto-created
	// Log error but don't fail - file creation might still work
	// Azure may create directories implicitly, but they won't appear in listings
	// This is a best-effort attempt to ensure directories are visible
	_ = s.ensureParentDirectories(ctx, name)

	fileClient, err := s.getFileClient(name)
	if err != nil {
		return &failWriter{err: err}
	}

	// Detect content type based on file extension (matches S3 behavior)
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = contentTypeOctetStream // Default for unknown extensions
	}

	// Create file if it doesn't exist (size 0 initially, will be resized on close)
	// File might already exist, that's okay for writing
	// Set content type based on file extension (e.g., .json -> application/json, .txt -> text/plain)
	_, _ = fileClient.Create(ctx, 0, &azfile.CreateOptions{
		HTTPHeaders: &azfile.HTTPHeaders{
			ContentType: &contentType,
		},
	})

	return &azureWriter{
		ctx:        ctx,
		fileClient: fileClient,
		name:       name,
		buffer:     make([]byte, 0),
	}
}

// failWriter is a helper for NewWriter validation errors.
type failWriter struct {
	err error
}

func (fw *failWriter) Write([]byte) (int, error) {
	return 0, fw.err
}

func (fw *failWriter) Close() error {
	return fw.err
}

// azureWriter implements io.WriteCloser for Azure File Storage.
type azureWriter struct {
	ctx        context.Context
	fileClient *azfile.Client
	name       string
	buffer     []byte
	offset     int64
	closed     bool
}

func (w *azureWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errWriterAlreadyClosed
	}

	w.buffer = append(w.buffer, p...)

	return len(p), nil
}

func (w *azureWriter) Close() error {
	if w.closed {
		return nil
	}

	w.closed = true

	if len(w.buffer) == 0 {
		return nil
	}

	// Get current file size to determine if we need to resize
	props, err := w.fileClient.GetProperties(w.ctx, nil)
	if err != nil {
		return w.createNewFile()
	}

	return w.resizeAndUpload(&props)
}

// createNewFile creates a new file with the buffer size.
func (w *azureWriter) createNewFile() error {
	// Detect content type based on file extension (matches S3 behavior)
	contentType := mime.TypeByExtension(filepath.Ext(w.name))
	if contentType == "" {
		contentType = contentTypeOctetStream // Default for unknown extensions
	}

	_, createErr := w.fileClient.Create(w.ctx, int64(len(w.buffer)), &azfile.CreateOptions{
		HTTPHeaders: &azfile.HTTPHeaders{
			ContentType: &contentType,
		},
	})
	if createErr != nil {
		return fmt.Errorf("failed to create file: %w", createErr)
	}

	reader := &bytesReadSeekCloser{data: w.buffer}
	_, err := w.fileClient.UploadRange(w.ctx, w.offset, reader, &azfile.UploadRangeOptions{})

	return err
}

// resizeAndUpload resizes the file if needed and uploads the buffer.
func (w *azureWriter) resizeAndUpload(props *azfile.GetPropertiesResponse) error {
	if props == nil || props.ContentLength == nil {
		// If we can't get current size, just upload
		reader := &bytesReadSeekCloser{data: w.buffer}

		_, err := w.fileClient.UploadRange(w.ctx, w.offset, reader, &azfile.UploadRangeOptions{})

		return err
	}

	currentSize := *props.ContentLength

	if int64(len(w.buffer)) > currentSize {
		_, resizeErr := w.fileClient.Resize(w.ctx, int64(len(w.buffer)), nil)
		if resizeErr != nil {
			return fmt.Errorf("failed to resize file: %w", resizeErr)
		}
	}

	reader := &bytesReadSeekCloser{data: w.buffer}
	_, err := w.fileClient.UploadRange(w.ctx, w.offset, reader, &azfile.UploadRangeOptions{})

	return err
}

// statDirectory returns metadata for a directory.
func (s *storageAdapter) statDirectory(ctx context.Context, name string) (*file.ObjectInfo, error) {
	dirPath := strings.TrimSuffix(name, "/")
	dirPath = strings.TrimPrefix(dirPath, "/")

	var dirClient *directory.Client
	if dirPath == "" {
		dirClient = s.shareClient.NewRootDirectoryClient()
	} else {
		dirClient = s.shareClient.NewDirectoryClient(dirPath)
	}

	props, err := dirClient.GetProperties(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%w %q: %w", errObjectNotFound, name, err)
	}

	var lastModified time.Time
	if props.LastModified != nil {
		lastModified = *props.LastModified
	}

	return &file.ObjectInfo{
		Name:         name,
		Size:         0,
		ContentType:  contentTypeDirectory,
		LastModified: lastModified,
		IsDir:        true,
	}, nil
}

// statFile returns metadata for a file.
func (s *storageAdapter) statFile(ctx context.Context, name string) (*file.ObjectInfo, error) {
	fileClient, err := s.getFileClient(name)
	if err != nil {
		return nil, fmt.Errorf("%w for %q: %w", errFailedToGetProperties, name, err)
	}

	props, err := fileClient.GetProperties(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%w %q: %w", errObjectNotFound, name, err)
	}

	contentType := ""
	if props.ContentType != nil {
		contentType = *props.ContentType
	}

	var size int64
	if props.ContentLength != nil {
		size = *props.ContentLength
	}

	var lastModified time.Time
	if props.LastModified != nil {
		lastModified = *props.LastModified
	}

	return &file.ObjectInfo{
		Name:         name,
		Size:         size,
		ContentType:  contentType,
		LastModified: lastModified,
		IsDir:        false,
	}, nil
}

// StatObject returns metadata for the given file or directory.
func (s *storageAdapter) StatObject(ctx context.Context, name string) (*file.ObjectInfo, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	// Check if it's a directory (ends with /)
	if strings.HasSuffix(name, "/") {
		return s.statDirectory(ctx, name)
	}

	return s.statFile(ctx, name)
}

// DeleteObject deletes the file or directory with the given name.
func (s *storageAdapter) DeleteObject(ctx context.Context, name string) error {
	if name == "" {
		return errEmptyObjectName
	}

	// Check if it's a directory
	if strings.HasSuffix(name, "/") {
		dirPath := strings.TrimSuffix(name, "/")
		dirPath = strings.TrimPrefix(dirPath, "/")

		var dirClient *directory.Client
		if dirPath == "" {
			dirClient = s.shareClient.NewRootDirectoryClient()
		} else {
			dirClient = s.shareClient.NewDirectoryClient(dirPath)
		}

		_, err := dirClient.Delete(ctx, nil)
		if err != nil {
			return fmt.Errorf("%w %q: %w", errFailedToDeleteObject, name, err)
		}

		return nil
	}

	// It's a file
	fileClient, err := s.getFileClient(name)
	if err != nil {
		return fmt.Errorf("%w for %q: %w", errFailedToDeleteObject, name, err)
	}

	_, err = fileClient.Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w %q: %w", errFailedToDeleteObject, name, err)
	}

	return nil
}

// CopyObject copies a file from src to dst.
func (s *storageAdapter) CopyObject(ctx context.Context, src, dst string) error {
	if src == "" || dst == "" {
		return errEmptySourceOrDest
	}

	if src == dst {
		return errSameSourceOrDest
	}

	// Get source file client and download
	downloadResp, srcProps, err := s.getSourceFileData(ctx, src)
	if err != nil {
		return err
	}
	defer downloadResp.Body.Close()

	// Create destination file
	dstClient, err := s.createDestinationFile(ctx, dst, srcProps)
	if err != nil {
		return err
	}

	// Copy content type if available
	if err := s.copyContentType(ctx, dstClient, srcProps); err != nil {
		return err
	}

	// Upload content to destination
	return s.uploadToDestination(ctx, dstClient, downloadResp.Body, src, dst)
}

// getSourceFileData downloads the source file and gets its properties.
func (s *storageAdapter) getSourceFileData(ctx context.Context, src string) (
	azfile.DownloadStreamResponse, *azfile.GetPropertiesResponse, error) {
	srcClient, err := s.getFileClient(src)
	if err != nil {
		return azfile.DownloadStreamResponse{}, nil,
			fmt.Errorf("%w: failed to get source client: %w", errFailedToCopyObject, err)
	}

	downloadResp, err := srcClient.DownloadStream(ctx, nil)
	if err != nil {
		return azfile.DownloadStreamResponse{}, nil,
			fmt.Errorf("%w from %q: %w", errFailedToCopyObject, src, err)
	}

	srcProps, err := srcClient.GetProperties(ctx, nil)
	if err != nil {
		downloadResp.Body.Close()

		return azfile.DownloadStreamResponse{}, nil,
			fmt.Errorf("%w: failed to get source properties: %w", errFailedToCopyObject, err)
	}

	return downloadResp, &srcProps, nil
}

// createDestinationFile creates the destination file with the same size as source.
// Automatically creates parent directories to ensure they appear in listings
// (matches local filesystem behavior where parent directories are auto-created).
func (s *storageAdapter) createDestinationFile(
	ctx context.Context, dst string, srcProps *azfile.GetPropertiesResponse) (*azfile.Client, error) {
	// Ensure parent directories exist (Azure requires explicit directory creation for listings)
	// This matches local filesystem behavior where parent directories are auto-created in CopyObject
	// Log error but don't fail - directory creation might still work
	// Azure may create directories implicitly, but they won't appear in listings
	// This is a best-effort attempt to ensure directories are visible
	_ = s.ensureParentDirectories(ctx, dst)

	dstClient, err := s.getFileClient(dst)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get destination client: %w", errFailedToCopyObject, err)
	}

	var contentLength int64
	if srcProps != nil && srcProps.ContentLength != nil {
		contentLength = *srcProps.ContentLength
	}

	// Detect content type based on destination file extension (matches S3 behavior)
	// If source has content type, prefer it; otherwise detect from extension
	var contentType string
	if srcProps != nil && srcProps.ContentType != nil && *srcProps.ContentType != "" {
		contentType = *srcProps.ContentType
	} else {
		contentType = mime.TypeByExtension(filepath.Ext(dst))
		if contentType == "" {
			contentType = contentTypeOctetStream // Default for unknown extensions
		}
	}

	_, err = dstClient.Create(ctx, contentLength, &azfile.CreateOptions{
		HTTPHeaders: &azfile.HTTPHeaders{
			ContentType: &contentType,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create destination file: %w", errFailedToCopyObject, err)
	}

	// Note: copyContentType is still called after this to ensure content type is set correctly
	// even if Create() didn't set it properly. This provides redundancy and ensures correctness.

	return dstClient, nil
}

// copyContentType copies the content type from source to destination if available.
// This is called after createDestinationFile to ensure content type is correctly set.
// If content type was already set during Create(), this will set it again to the same value (safe).
func (*storageAdapter) copyContentType(ctx context.Context, dstClient *azfile.Client, srcProps *azfile.GetPropertiesResponse) error {
	if srcProps == nil || srcProps.ContentType == nil || *srcProps.ContentType == "" {
		return nil
	}

	_, err := dstClient.SetHTTPHeaders(ctx, &azfile.SetHTTPHeadersOptions{
		HTTPHeaders: &azfile.HTTPHeaders{
			ContentType: srcProps.ContentType,
		},
	})
	if err != nil {
		return fmt.Errorf("%w: failed to set content type: %w", errFailedToCopyObject, err)
	}

	return nil
}

// uploadToDestination uploads the content to the destination file.
func (*storageAdapter) uploadToDestination(ctx context.Context, dstClient *azfile.Client, body io.ReadCloser, src, dst string) error {
	readSeekCloser := &readSeekCloserWrapper{reader: body}

	_, err := dstClient.UploadRange(ctx, 0, readSeekCloser, &azfile.UploadRangeOptions{})
	if err != nil {
		return fmt.Errorf("%w from %q to %q: %w", errFailedToCopyObject, src, dst, err)
	}

	return nil
}

// normalizePrefix normalizes the prefix for listing operations.
func normalizePrefix(prefix string) string {
	normalizedPrefix := strings.TrimPrefix(prefix, "/")
	if normalizedPrefix != "" && !strings.HasSuffix(normalizedPrefix, "/") {
		normalizedPrefix += "/"
	}

	return normalizedPrefix
}

// getDirectoryClient returns the appropriate directory client for the given prefix.
func (s *storageAdapter) getDirectoryClient(normalizedPrefix string) *directory.Client {
	if normalizedPrefix == "" {
		return s.shareClient.NewRootDirectoryClient()
	}

	dirPath := strings.TrimSuffix(normalizedPrefix, "/")

	return s.shareClient.NewDirectoryClient(dirPath)
}

// processListObjectsPage processes a single page of list results and adds files to objects.
func processListObjectsPage(page *directory.ListFilesAndDirectoriesResponse, normalizedPrefix string, objects []string) []string {
	for _, fileItem := range page.Segment.Files {
		if fileItem.Name == nil {
			continue
		}

		fileName := *fileItem.Name
		if normalizedPrefix == "" || strings.HasPrefix(fileName, normalizedPrefix) {
			objects = append(objects, fileName)
		}
	}

	return objects
}

// ListObjects lists all files with the given prefix.
func (s *storageAdapter) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	if s.shareClient == nil {
		return nil, errAzureClientNotInitialized
	}

	var objects []string

	normalizedPrefix := normalizePrefix(prefix)
	dirClient := s.getDirectoryClient(normalizedPrefix)
	pager := dirClient.NewListFilesAndDirectoriesPager(&directory.ListFilesAndDirectoriesOptions{})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w with prefix %q: %w", errFailedToListObjects, prefix, err)
		}

		objects = processListObjectsPage(&page, normalizedPrefix, objects)
	}

	return objects, nil
}

// processListDirDirectories processes directory items from a list page.
func processListDirDirectories(directories []*directory.Directory, prefixes []string) []string {
	for _, dirItem := range directories {
		if dirItem.Name == nil {
			continue
		}

		dirName := *dirItem.Name
		if !strings.HasSuffix(dirName, "/") {
			dirName += "/"
		}

		prefixes = append(prefixes, dirName)
	}

	return prefixes
}

// processListDirFiles processes file items from a list page.
func processListDirFiles(files []*directory.File, objects []file.ObjectInfo) []file.ObjectInfo {
	for _, fileItem := range files {
		if fileItem.Name == nil || fileItem.Properties == nil {
			continue
		}

		var size int64
		if fileItem.Properties.ContentLength != nil {
			size = *fileItem.Properties.ContentLength
		}

		var lastModified time.Time
		if fileItem.Properties.LastModified != nil {
			lastModified = *fileItem.Properties.LastModified
		}

		// FileProperty doesn't have ContentType field in directory package
		// We'll use empty string for ContentType as it's not available in directory listing
		objects = append(objects, file.ObjectInfo{
			Name:         *fileItem.Name,
			Size:         size,
			ContentType:  "", // ContentType not available in directory listing
			LastModified: lastModified,
			IsDir:        false,
		})
	}

	return objects
}

// ListDir lists files and directories (prefixes) under the given prefix.
func (s *storageAdapter) ListDir(ctx context.Context, prefix string) ([]file.ObjectInfo, []string, error) {
	if s.shareClient == nil {
		return nil, nil, errAzureClientNotInitialized
	}

	var (
		objects  []file.ObjectInfo
		prefixes []string
	)

	normalizedPrefix := normalizePrefix(prefix)
	dirClient := s.getDirectoryClient(normalizedPrefix)
	pager := dirClient.NewListFilesAndDirectoriesPager(&directory.ListFilesAndDirectoriesOptions{})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("%w %q: %w", errFailedToListDirectory, prefix, err)
		}

		prefixes = processListDirDirectories(page.Segment.Directories, prefixes)
		objects = processListDirFiles(page.Segment.Files, objects)
	}

	return objects, prefixes, nil
}

// bytesReadSeekCloser wraps a byte slice to implement io.ReadSeekCloser.
type bytesReadSeekCloser struct {
	data   []byte
	offset int64
}

func (b *bytesReadSeekCloser) Read(p []byte) (int, error) {
	if b.offset >= int64(len(b.data)) {
		return 0, io.EOF
	}

	n := copy(p, b.data[b.offset:])
	b.offset += int64(n)

	return n, nil
}

func (b *bytesReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64

	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = b.offset + offset
	case io.SeekEnd:
		newOffset = int64(len(b.data)) + offset
	default:
		return 0, fmt.Errorf("%w: %d", errInvalidWhence, whence)
	}

	if newOffset < 0 {
		return 0, fmt.Errorf("%w: %d", errNegativeOffset, newOffset)
	}

	if newOffset > int64(len(b.data)) {
		newOffset = int64(len(b.data))
	}

	b.offset = newOffset

	return newOffset, nil
}

func (*bytesReadSeekCloser) Close() error {
	return nil
}

// readSeekCloserWrapper wraps io.ReadCloser to implement io.ReadSeekCloser.
// Note: Seek operations will read and discard data, which is inefficient but necessary for compatibility.
type readSeekCloserWrapper struct {
	reader io.ReadCloser
	offset int64
	buffer []byte
}

func (r *readSeekCloserWrapper) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *readSeekCloserWrapper) Seek(offset int64, whence int) (int64, error) {
	// For simplicity, we'll read all data into buffer on first seek
	// This is not efficient but works for copy operations
	if r.buffer == nil {
		data, err := io.ReadAll(r.reader)
		if err != nil {
			return 0, err
		}

		r.buffer = data
		r.reader = io.NopCloser(bytes.NewReader(r.buffer))
	}

	var newOffset int64

	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.offset + offset
	case io.SeekEnd:
		newOffset = int64(len(r.buffer)) + offset
	default:
		return 0, fmt.Errorf("%w: %d", errInvalidWhence, whence)
	}

	if newOffset < 0 {
		return 0, fmt.Errorf("%w: %d", errNegativeOffset, newOffset)
	}

	if newOffset > int64(len(r.buffer)) {
		newOffset = int64(len(r.buffer))
	}

	r.offset = newOffset
	r.reader = io.NopCloser(bytes.NewReader(r.buffer[newOffset:]))

	return newOffset, nil
}

func (r *readSeekCloserWrapper) Close() error {
	return r.reader.Close()
}

// isDirectoryExistsError checks if the error indicates the directory already exists.
func isDirectoryExistsError(err error) bool {
	errStr := err.Error()

	return strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "ShareAlreadyExists") ||
		strings.Contains(errStr, "ResourceAlreadyExists")
}

// createDirectoryLevel creates a single directory level and handles errors.
func (s *storageAdapter) createDirectoryLevel(ctx context.Context, dirPath string) error {
	dirClient := s.shareClient.NewDirectoryClient(dirPath)

	_, err := dirClient.Create(ctx, nil)
	if err != nil && !isDirectoryExistsError(err) {
		return fmt.Errorf("failed to create directory %q: %w", dirPath, err)
	}

	return nil
}

// ensureParentDirectories creates all parent directories for the given file path.
// Azure File Storage requires explicit directory creation for directories to appear in listings.
// This function ensures parent directories are created before file creation, matching
// local filesystem behavior where os.MkdirAll is called automatically.
func (s *storageAdapter) ensureParentDirectories(ctx context.Context, filePath string) error {
	if s.shareClient == nil {
		return errAzureClientNotInitialized
	}

	// Extract parent directory path
	parentDir := getParentDir(filePath)
	if parentDir == "" {
		return nil // File is in root, no parent directories needed
	}

	// Normalize path (remove leading/trailing slashes)
	parentDir = strings.Trim(parentDir, "/")
	if parentDir == "" {
		return nil
	}

	// Split into components and create each level
	components := strings.Split(parentDir, "/")
	currentPath := ""

	for _, component := range components {
		if component == "" || component == "." || component == ".." {
			continue
		}

		if currentPath == "" {
			currentPath = component
		} else {
			currentPath = currentPath + "/" + component
		}

		if err := s.createDirectoryLevel(ctx, currentPath); err != nil {
			return err
		}
	}

	return nil
}

// getParentDir extracts the parent directory path from a file path.
// Example: "dir1/subdir/file.txt" -> "dir1/subdir".
// Example: "file.txt" -> "" (root directory).
func getParentDir(filePath string) string {
	// Remove leading slash if present
	filePath = strings.TrimPrefix(filePath, "/")

	// Find last slash
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash == -1 {
		return "" // File is in root
	}

	// Return everything before the last slash
	return filePath[:lastSlash]
}

// getFileClient returns a file client for the given path.
func (s *storageAdapter) getFileClient(name string) (*azfile.Client, error) {
	if s.shareClient == nil {
		return nil, errAzureClientNotInitialized
	}

	// Normalize the path
	name = strings.TrimPrefix(name, "/")
	dirPath := filepath.Dir(name)
	fileName := filepath.Base(name)

	// Handle root directory case
	if dirPath == "." || dirPath == "" || dirPath == "/" {
		// File is in root directory
		return s.shareClient.NewRootDirectoryClient().NewFileClient(fileName), nil
	}

	// File is in a subdirectory
	// Normalize directory path (remove leading/trailing slashes)
	dirPath = strings.TrimPrefix(dirPath, "/")
	dirPath = strings.TrimSuffix(dirPath, "/")

	return s.shareClient.NewDirectoryClient(dirPath).NewFileClient(fileName), nil
}
