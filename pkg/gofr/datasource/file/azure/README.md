# Azure File Storage Implementation for GoFr

This package provides Azure File Storage integration for the GoFr framework, following the same patterns as the GCS and S3 implementations using the `CommonFileSystem` pattern.

## Features

- File operations (Create, Open, Read, Write, Delete)
- Directory operations (Mkdir, MkdirAll, ReadDir, Remove, RemoveAll)
- File metadata and properties (Stat)
- Metrics and logging support
- Automatic retry on connection failure
- Comprehensive unit tests

## Configuration

```go
config := &azure.Config{
    AccountName: "your-storage-account",
    AccountKey:  "your-account-key",
    ShareName:   "your-file-share",
    Endpoint:    "", // Optional, defaults to https://{AccountName}.file.core.windows.net
}
```

## Usage

```go
import (
    "gofr.dev/pkg/gofr/datasource/file/azure"
    "gofr.dev/pkg/gofr/datasource"
    "gofr.dev/pkg/gofr/datasource/file"
)

// Create a new Azure File Storage filesystem
config := &azure.Config{
    AccountName: "mystorageaccount",
    AccountKey:  "myaccountkey",
    ShareName:   "myshare",
}

// Pass logger and metrics to New()
fs, err := azure.New(config, logger, metrics)
if err != nil {
    // Handle error
}

// Connect to Azure File Storage (automatic retry on failure)
fs.Connect()

// Use the filesystem - all methods from CommonFileSystem are available
file, err := fs.Create("test.txt")
if err != nil {
    // Handle error
}
defer file.Close()

// Write data
_, err = file.Write([]byte("Hello, Azure File Storage!"))
if err != nil {
    // Handle error
}

// Open and read data
readFile, err := fs.Open("test.txt")
if err != nil {
    // Handle error
}
defer readFile.Close()

data := make([]byte, 1024)
n, err := readFile.Read(data)
```

## Implementation Details

### File Operations

The implementation supports all standard file operations:
- `Create(name string)` - Creates a new file
- `Open(name string)` - Opens an existing file
- `Read(p []byte)` - Reads data from file
- `Write(p []byte)` - Writes data to file
- `Seek(offset int64, whence int)` - Seeks to position in file
- `Close()` - Closes the file

### Directory Operations

- `Mkdir(name string, perm os.FileMode)` - Creates a directory
- `MkdirAll(path string, perm os.FileMode)` - Creates directory hierarchy
- `ReadDir(dir string)` - Lists directory contents
- `Remove(name string)` - Removes a file or directory
- `RemoveAll(path string)` - Removes directory and contents

### Azure File Storage Specifics

- Uses Azure File Storage REST API via the Azure SDK
- Supports file shares (equivalent to S3 buckets)
- Handles file paths relative to the share
- Supports file properties and metadata

## Testing

The package includes comprehensive unit tests covering:
- Connection and health checks
- File operations (read, write, stat, delete, copy)
- Directory operations (list, create, delete)
- Error handling and edge cases
- Helper functions and utilities

Tests use mocks from `gofr.dev/pkg/gofr/datasource/file` package for logger and metrics.

## Dependencies

- `github.com/Azure/azure-sdk-for-go/sdk/azcore`
- `github.com/Azure/azure-sdk-for-go/sdk/storage/azfile`
- `github.com/stretchr/testify` (for testing)
- `go.uber.org/mock` (for mocking)

## Implementation Details

### Architecture

The implementation follows the `CommonFileSystem` pattern:
- `storageAdapter` implements `file.StorageProvider` interface
- `azureFileSystem` embeds `file.CommonFileSystem` for shared functionality
- All file operations are handled through `CommonFileSystem` methods
- Automatic retry logic for connection failures

### Available Methods

All methods from `file.FileSystemProvider` are available:
- `Create(name string) (File, error)` - Create a new file
- `Open(name string) (File, error)` - Open an existing file
- `OpenFile(name string, flag int, perm os.FileMode) (File, error)` - Open with flags
- `Stat(name string) (FileInfo, error)` - Get file/directory metadata
- `Mkdir(name string, perm os.FileMode) error` - Create directory
- `MkdirAll(path string, perm os.FileMode) error` - Create directory hierarchy
- `ReadDir(dir string) ([]FileInfo, error)` - List directory contents
- `Remove(name string) error` - Remove file or directory
- `RemoveAll(path string) error` - Remove directory and contents
- `Connect()` - Connect to Azure File Storage

## Notes

- Azure File Storage doesn't support direct rename operations (use CopyObject + DeleteObject)
- Directory operations are implemented using Azure File Storage's directory concept
- The implementation follows the same interface as other GoFr file storage providers (GCS, S3)
- Error handling and logging follow GoFr conventions
- Connection retry happens automatically in the background if initial connection fails

