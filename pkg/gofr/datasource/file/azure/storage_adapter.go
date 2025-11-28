package azure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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
)

const (
	contentTypeDirectory = "application/x-directory"
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

	shareURL := endpoint + "/" + s.cfg.ShareName

	// Create share client
	shareClient, err := share.NewClientWithSharedKeyCredential(shareURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create share client: %w", err)
	}

	// Validate share access by getting properties
	_, err = shareClient.GetProperties(ctx, nil)
	if err != nil {
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
func (s *storageAdapter) NewWriter(ctx context.Context, name string) io.WriteCloser {
	if name == "" {
		return &failWriter{err: errEmptyObjectName}
	}

	fileClient, err := s.getFileClient(name)
	if err != nil {
		return &failWriter{err: err}
	}

	// Create file if it doesn't exist (size 0 initially, will be resized on close)
	// File might already exist, that's okay for writing
	_, _ = fileClient.Create(ctx, 0, nil)

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
	_, createErr := w.fileClient.Create(w.ctx, int64(len(w.buffer)), nil)
	if createErr != nil {
		return fmt.Errorf("failed to create file: %w", createErr)
	}

	reader := &bytesReadSeekCloser{data: w.buffer}
	_, err := w.fileClient.UploadRange(w.ctx, w.offset, reader, &azfile.UploadRangeOptions{})

	return err
}

// resizeAndUpload resizes the file if needed and uploads the buffer.
func (w *azureWriter) resizeAndUpload(props *azfile.GetPropertiesResponse) error {
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

// StatObject returns metadata for the given file or directory.
func (s *storageAdapter) StatObject(ctx context.Context, name string) (*file.ObjectInfo, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	// Check if it's a directory (ends with /)
	if strings.HasSuffix(name, "/") {
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

		return &file.ObjectInfo{
			Name:         name,
			Size:         0,
			ContentType:  contentTypeDirectory,
			LastModified: *props.LastModified,
			IsDir:        true,
		}, nil
	}

	// It's a file
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

	return &file.ObjectInfo{
		Name:         name,
		Size:         *props.ContentLength,
		ContentType:  contentType,
		LastModified: *props.LastModified,
		IsDir:        false,
	}, nil
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
func (s *storageAdapter) createDestinationFile(
	ctx context.Context, dst string, srcProps *azfile.GetPropertiesResponse) (*azfile.Client, error) {
	dstClient, err := s.getFileClient(dst)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get destination client: %w", errFailedToCopyObject, err)
	}

	_, err = dstClient.Create(ctx, *srcProps.ContentLength, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create destination file: %w", errFailedToCopyObject, err)
	}

	return dstClient, nil
}

// copyContentType copies the content type from source to destination if available.
func (*storageAdapter) copyContentType(ctx context.Context, dstClient *azfile.Client, srcProps *azfile.GetPropertiesResponse) error {
	if srcProps.ContentType == nil {
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

// ListObjects lists all files with the given prefix.
func (s *storageAdapter) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	if s.shareClient == nil {
		return nil, errAzureClientNotInitialized
	}

	var objects []string

	// Normalize prefix
	normalizedPrefix := strings.TrimPrefix(prefix, "/")
	if normalizedPrefix != "" && !strings.HasSuffix(normalizedPrefix, "/") {
		normalizedPrefix += "/"
	}

	var dirClient *directory.Client
	if normalizedPrefix == "" {
		dirClient = s.shareClient.NewRootDirectoryClient()
	} else {
		// Get directory client for the prefix
		dirPath := strings.TrimSuffix(normalizedPrefix, "/")
		dirClient = s.shareClient.NewDirectoryClient(dirPath)
	}

	pager := dirClient.NewListFilesAndDirectoriesPager(&directory.ListFilesAndDirectoriesOptions{})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w with prefix %q: %w", errFailedToListObjects, prefix, err)
		}

		// Add files
		for _, fileItem := range page.Segment.Files {
			fileName := *fileItem.Name
			// If prefix is specified, only include files that match
			if normalizedPrefix == "" || strings.HasPrefix(fileName, normalizedPrefix) {
				objects = append(objects, fileName)
			}
		}
	}

	return objects, nil
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

	// Normalize prefix
	normalizedPrefix := strings.TrimPrefix(prefix, "/")
	if normalizedPrefix != "" && !strings.HasSuffix(normalizedPrefix, "/") {
		normalizedPrefix += "/"
	}

	var dirClient *directory.Client
	if normalizedPrefix == "" {
		dirClient = s.shareClient.NewRootDirectoryClient()
	} else {
		// Get directory client for the prefix
		dirPath := strings.TrimSuffix(normalizedPrefix, "/")
		dirClient = s.shareClient.NewDirectoryClient(dirPath)
	}

	pager := dirClient.NewListFilesAndDirectoriesPager(&directory.ListFilesAndDirectoriesOptions{})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("%w %q: %w", errFailedToListDirectory, prefix, err)
		}

		// Add directories (prefixes)
		for _, dirItem := range page.Segment.Directories {
			dirName := *dirItem.Name

			if !strings.HasSuffix(dirName, "/") {
				dirName += "/"
			}

			prefixes = append(prefixes, dirName)
		}

		// Add files
		for _, fileItem := range page.Segment.Files {
			// FileProperty doesn't have ContentType field in directory package
			// We'll use empty string for ContentType as it's not available in directory listing
			objects = append(objects, file.ObjectInfo{
				Name:         *fileItem.Name,
				Size:         *fileItem.Properties.ContentLength,
				ContentType:  "", // ContentType not available in directory listing
				LastModified: *fileItem.Properties.LastModified,
				IsDir:        false,
			})
		}
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
