package azure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/share"
	fileSystem "gofr.dev/pkg/gofr/datasource/file"
)

const (
	// File permissions.
	filePermission = 0644
	dirPermission  = 0755
)

// client struct embeds the *share.Client and *file.Client.
type client struct {
	shareClient *share.Client
	fileClient  *file.Client
}

// CreateDirectory creates a directory in Azure File Storage.
func (c *client) CreateDirectory(ctx context.Context, path string, _ any) (any, error) {
	if c.shareClient == nil {
		return nil, ErrShareClientNotInitialized
	}

	// Create directory using Azure File Share client
	_, err := c.shareClient.NewDirectoryClient(path).Create(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return &struct{}{}, nil
}

// DeleteDirectory deletes a directory from Azure File Storage.
func (c *client) DeleteDirectory(ctx context.Context, path string, _ any) (any, error) {
	if c.shareClient == nil {
		return nil, ErrShareClientNotInitialized
	}

	// Delete directory using Azure File Share client
	_, err := c.shareClient.NewDirectoryClient(path).Delete(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete directory %s: %w", path, err)
	}

	return &struct{}{}, nil
}

// ListFilesAndDirectoriesSegment lists files and directories in Azure File Storage.
func (c *client) ListFilesAndDirectoriesSegment(ctx context.Context, marker *string, _ any) (any, error) {
	if c.shareClient == nil {
		return nil, ErrShareClientNotInitialized
	}

	// List files and directories using Azure File Share client
	listOptions := &directory.ListFilesAndDirectoriesOptions{}
	if marker != nil {
		listOptions.Marker = marker
	}

	pager := c.shareClient.NewRootDirectoryClient().NewListFilesAndDirectoriesPager(listOptions)

	var results []any

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get next page: %w", err)
		}

		results = append(results, page)
	}

	return results, nil
}

// CreateFile creates a file in Azure File Storage.
func (c *client) CreateFile(ctx context.Context, path string, size int64, _ any) (any, error) {
	if c.shareClient == nil {
		return nil, ErrShareClientNotInitialized
	}

	// Create file using Azure File Share client
	// First get the directory client for the path, then create the file
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)

	var fileClient *file.Client
	if dirPath == "." || dirPath == "/" {
		// File is in root directory
		fileClient = c.shareClient.NewRootDirectoryClient().NewFileClient(fileName)
	} else {
		// File is in a subdirectory
		fileClient = c.shareClient.NewDirectoryClient(dirPath).NewFileClient(fileName)
	}

	_, err := fileClient.Create(ctx, size, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", path, err)
	}

	return &struct{}{}, nil
}

// DeleteFile deletes a file from Azure File Storage.
func (c *client) DeleteFile(ctx context.Context, path string, _ any) (any, error) {
	if c.shareClient == nil {
		return nil, ErrShareClientNotInitialized
	}

	// Delete file using Azure File Share client
	// First get the directory client for the path, then delete the file
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)

	var fileClient *file.Client
	if dirPath == "." || dirPath == "/" {
		// File is in root directory
		fileClient = c.shareClient.NewRootDirectoryClient().NewFileClient(fileName)
	} else {
		// File is in a subdirectory
		fileClient = c.shareClient.NewDirectoryClient(dirPath).NewFileClient(fileName)
	}

	_, err := fileClient.Delete(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete file %s: %w", path, err)
	}

	return &struct{}{}, nil
}

// DownloadFile downloads a file from Azure File Storage.
func (c *client) DownloadFile(_ context.Context, _ any) (any, error) {
	if c.shareClient == nil {
		return nil, ErrShareClientNotInitialized
	}

	// This would need more context about which file to download
	// For now, return an error indicating this needs file path
	return nil, ErrDownloadFileRequiresPath
}

// UploadRange uploads a range of data to Azure File Storage.
func (c *client) UploadRange(_ context.Context, _ int64, _ io.ReadSeekCloser, _ any) (any, error) {
	if c.shareClient == nil {
		return nil, ErrShareClientNotInitialized
	}

	// This would need more context about which file to upload to
	// For now, return an error indicating this needs file path
	return nil, ErrUploadRangeRequiresPath
}

// GetProperties gets properties of a file in Azure File Storage.
func (c *client) GetProperties(_ context.Context, _ any) (any, error) {
	if c.shareClient == nil {
		return nil, ErrShareClientNotInitialized
	}

	// This would need more context about which file to get properties for
	// For now, return an error indicating this needs file path
	return nil, ErrGetPropertiesRequiresPath
}

type FileSystem struct {
	conn    azureClient
	config  *Config
	logger  Logger
	metrics Metrics
}

// Config represents the azure file storage configuration.
type Config struct {
	AccountName string // Azure Storage Account name
	AccountKey  string // Azure Storage Account key
	ShareName   string // Azure File Share name
	Endpoint    string // Azure Storage endpoint (optional, defaults to core.windows.net)
}

// New initializes a new instance of Azure File Storage with provided configuration.
func New(config *Config) fileSystem.FileSystemProvider {
	return &FileSystem{config: config}
}

// UseLogger sets the Logger interface for the Azure file system.
func (f *FileSystem) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the Metrics interface.
func (f *FileSystem) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}

// Connect initializes and validates the connection to the Azure File Storage service.
func (f *FileSystem) Connect() {
	defer f.sendOperationStats(&FileLog{
		Operation: "CONNECT",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	f.logger.Debugf("connecting to Azure File Share: %s", f.config.ShareName)

	// Create Azure credentials and clients
	cred, err := share.NewSharedKeyCredential(f.config.AccountName, f.config.AccountKey)
	if err != nil {
		f.logger.Errorf("failed to create shared key credential: %v", err)
		return
	}

	// Build the share URL
	endpoint := f.config.Endpoint
	if endpoint == "" {
		endpoint = "https://" + f.config.AccountName + ".file.core.windows.net"
	}

	shareURL := endpoint + "/" + f.config.ShareName

	// Create share client
	shareClient, err := share.NewClientWithSharedKeyCredential(shareURL, cred, nil)
	if err != nil {
		f.logger.Errorf("failed to create share client: %v", err)
		return
	}

	// Create file client (for root operations)
	fileClient, err := file.NewClientWithSharedKeyCredential(endpoint+"/"+f.config.ShareName+"/", cred, nil)
	if err != nil {
		f.logger.Errorf("failed to create file client: %v", err)
		return
	}

	f.conn = &client{shareClient: shareClient, fileClient: fileClient}
	f.logger.Logf("successfully connected to Azure File Share: %s", f.config.ShareName)
}

// Create creates a file in the Azure File Storage.
func (f *FileSystem) Create(name string) (fileSystem.File, error) {
	defer f.sendOperationStats(&FileLog{
		Operation: "CREATE",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// Create file using Azure SDK
	_, err := f.conn.CreateFile(context.Background(), name, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", name, err)
	}

	return &File{
		conn:    f.conn,
		name:    name,
		logger:  f.logger,
		metrics: f.metrics,
		ctx:     context.Background(),
	}, nil
}

// Open opens a file in the Azure File Storage.
func (f *FileSystem) Open(name string) (fileSystem.File, error) {
	defer f.sendOperationStats(&FileLog{
		Operation: "OPEN",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// Check if file exists and get its properties
	// For now, we'll assume the file exists and return a file object
	// In a full implementation, you'd check file existence and get properties
	return &File{
		conn:         f.conn,
		name:         name,
		logger:       f.logger,
		metrics:      f.metrics,
		size:         0,          // TODO: Get actual file size from Azure
		lastModified: time.Now(), // TODO: Get actual modification time from Azure
		ctx:          context.Background(),
	}, nil
}

// OpenFile opens a file using the given flags and the given mode.
func (f *FileSystem) OpenFile(name string, _ int, _ os.FileMode) (fileSystem.File, error) {
	// For Azure File Storage, we'll use Open for now
	// In a full implementation, you'd handle different flags
	return f.Open(name)
}

// Remove removes a file from the Azure File Storage.
func (f *FileSystem) Remove(name string) error {
	defer f.sendOperationStats(&FileLog{
		Operation: "REMOVE",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// Remove file using Azure SDK
	_, err := f.conn.DeleteFile(context.Background(), name, nil)
	if err != nil {
		return fmt.Errorf("failed to remove file %s: %w", name, err)
	}

	return nil
}

// RemoveAll removes a directory and all its contents.
func (f *FileSystem) RemoveAll(path string) error {
	// Implementation would list all files in the directory and remove them
	// For now, just remove the directory itself
	return f.Remove(path)
}

// Rename renames a file in the Azure File Storage.
func (f *FileSystem) Rename(oldname, newname string) error {
	defer f.sendOperationStats(&FileLog{
		Operation: "RENAME",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// Azure File Storage doesn't have a direct rename operation
	// We need to copy the file and then delete the original
	// This is a simplified implementation
	f.logger.Debugf("Rename called from %s to %s (not implemented for Azure)", oldname, newname)

	return ErrRenameNotImplemented
}

// Mkdir creates a directory in the Azure File Storage.
func (f *FileSystem) Mkdir(name string, _ os.FileMode) error {
	defer f.sendOperationStats(&FileLog{
		Operation: "MKDIR",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// Create directory using Azure SDK
	_, err := f.conn.CreateDirectory(context.Background(), name, nil)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", name, err)
	}

	return nil
}

// MkdirAll creates a directory path and all parents that do not exist yet.
func (f *FileSystem) MkdirAll(path string, perm os.FileMode) error {
	// Split the path and create each directory level
	parts := strings.Split(path, string(filepath.Separator))
	currentPath := ""

	for _, part := range parts {
		if part == "" {
			continue
		}

		currentPath = filepath.Join(currentPath, part)

		err := f.Mkdir(currentPath, perm)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	}

	return nil
}

// ReadDir returns a list of files/directories present in the directory.
func (f *FileSystem) ReadDir(dir string) ([]fileSystem.FileInfo, error) {
	defer f.sendOperationStats(&FileLog{
		Operation: "READDIR",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// List files and directories using Azure SDK
	_, err := f.conn.ListFilesAndDirectoriesSegment(context.Background(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory %s: %w", dir, err)
	}

	// Convert Azure response to FileInfo slice
	// This is a simplified implementation - in a full version you'd parse the actual response
	var fileInfos []fileSystem.FileInfo

	// For now, return empty list as we need to parse the Azure response properly
	// TODO: Parse the actual Azure response and convert to FileInfo objects

	return fileInfos, nil
}

// Stat returns the file/directory information.
func (f *FileSystem) Stat(name string) (fileSystem.FileInfo, error) {
	defer f.sendOperationStats(&FileLog{
		Operation: "STAT",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// TODO: Implement proper file properties
	return &azureFileInfo{
		name:    name,
		size:    0,
		isDir:   false,
		modTime: time.Now(),
	}, nil
}

// ChDir changes the current directory.
func (f *FileSystem) ChDir(dir string) error {
	defer f.sendOperationStats(&FileLog{
		Operation: "CHDIR",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// Azure File Storage doesn't have a concept of current directory
	// This would need to be implemented at the application level
	f.logger.Debugf("ChDir called with directory: %s (not implemented for Azure)", dir)

	return ErrChDirNotImplemented
}

// Getwd returns the path of the current directory.
func (f *FileSystem) Getwd() (string, error) {
	defer f.sendOperationStats(&FileLog{
		Operation: "GETWD",
		Location:  getLocation(f.config.ShareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	// Azure File Storage doesn't have a concept of current directory
	f.logger.Debugf("Getwd called (not implemented for Azure)")

	return "/", nil
}

// Helper functions.
func getFilePath(name string) string {
	// Remove leading slash if present
	return strings.TrimPrefix(name, "/")
}

// azureFileInfo implements fileSystem.FileInfo.
type azureFileInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
}

func (fi *azureFileInfo) Name() string {
	return fi.name
}

func (fi *azureFileInfo) Size() int64 {
	return fi.size
}

func (fi *azureFileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *azureFileInfo) Mode() os.FileMode {
	if fi.isDir {
		return os.ModeDir | dirPermission
	}

	return filePermission
}

func (fi *azureFileInfo) IsDir() bool {
	return fi.isDir
}
