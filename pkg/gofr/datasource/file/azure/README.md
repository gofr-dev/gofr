# Azure File Storage Implementation for GoFr

This package provides Azure File Storage integration for the GoFr framework, following the same patterns as the S3 implementation.

## Features

- File operations (Create, Open, Read, Write, Delete)
- Directory operations (Create, List, Delete)
- File metadata and properties
- Metrics and logging support
- Mock interface for testing

## Configuration

```go
config := &azure.Config{
    AccountName: "your-storage-account",
    AccountKey:  "your-account-key",
    ShareName:   "your-file-share",
    Endpoint:    "", // Optional, defaults to core.windows.net
}
```

## Usage

```go
import (
    "gofr.dev/pkg/gofr/datasource/file/azure"
)

// Create a new Azure File Storage filesystem
config := &azure.Config{
    AccountName: "mystorageaccount",
    AccountKey:  "myaccountkey",
    ShareName:   "myshare",
}

fs := azure.New(config)

// Set up logging and metrics
fs.UseLogger(logger)
fs.UseMetrics(metrics)

// Connect to Azure File Storage
fs.Connect()

// Use the filesystem
file, err := fs.Create("test.txt")
if err != nil {
    // Handle error
}
defer file.Close()

// Write data
file.Write([]byte("Hello, Azure File Storage!"))

// Read data
data := make([]byte, 1024)
n, err := file.Read(data)
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

The package includes mock interfaces for testing:

```go
mockClient := &azure.MockAzureClient{
    CreateFileResponse: file.CreateResponse{},
    CreateFileError:    nil,
}

// Use mock client in tests
```

## Dependencies

- `github.com/Azure/azure-sdk-for-go/sdk/azcore`
- `github.com/Azure/azure-sdk-for-go/sdk/storage/azfile`
- `github.com/stretchr/testify` (for testing)
- `go.uber.org/mock` (for mocking)

## Notes

- Azure File Storage doesn't support direct rename operations
- Directory operations are implemented using Azure File Storage's directory concept
- The implementation follows the same interface as other GoFr file storage providers
- Error handling and logging follow GoFr conventions

