# Azure File Storage Example

This example demonstrates how to use Azure File Storage with GoFr to perform various file and directory operations.

## Prerequisites

- Azure Storage Account with File Storage enabled
- Azure Storage Account Name and Key
- Azure File Share created in your storage account

## Important Note

**Azurite (Azure Storage Emulator) does NOT support Azure File Storage.** Azurite only supports:
- Blob Storage
- Queue Storage  
- Table Storage

## Testing Options

### Option 1: Use Actual Azure Storage Account (Recommended for Integration Tests)

For testing Azure File Storage, you need to use actual Azure Storage Account credentials. 

**Free Tier Benefits:**
- 5GB of storage
- 20,000 transactions per month
- Perfect for development and testing

**Setup Steps:**
1. Create a free Azure account: https://azure.microsoft.com/free/
2. Create a Storage Account in Azure Portal
3. Create a File Share in your Storage Account
4. Get your Storage Account name and access key
5. Configure the credentials in `configs/.env`

### Option 2: Use Mocks for Unit Testing

The implementation includes comprehensive unit tests that use mocks and don't require Azure credentials. These tests are located in:
- `pkg/gofr/datasource/file/azure/storage_adapter_test.go`
- `pkg/gofr/datasource/file/azure/fs_test.go`

### Option 3: Use Local Filesystem for Development

For basic development without Azure-specific features, you can use GoFr's local filesystem:
```go
import "gofr.dev/pkg/gofr/datasource/file"

fs := file.NewLocalFileSystem(logger)
app.AddFileStore(fs)
```

**Note:** This won't test Azure File Storage specific features like share management, but is useful for basic file operations during development.

## Configuration

Create a `configs/.env` file with the following variables:

```env
AZURE_STORAGE_ACCOUNT=your-storage-account-name
AZURE_STORAGE_KEY=your-storage-account-key
AZURE_FILE_SHARE=your-file-share-name
AZURE_STORAGE_ENDPOINT=  # Optional, defaults to https://{AccountName}.file.core.windows.net
```

### Getting Azure Storage Credentials

1. Go to [Azure Portal](https://portal.azure.com)
2. Navigate to your Storage Account
3. Go to "Access keys" under "Security + networking"
4. Copy the "Storage account name" and one of the "Key" values
5. Create a File Share:
   - Go to "File shares" under "Data storage"
   - Click "+ File share"
   - Enter a name and click "Create"

## Running the Example

```bash
go run main.go
```

The server will start on `http://localhost:8000` (default port).

## API Endpoints

### 1. List Files in Current Directory
```bash
GET /files
```

### 2. List Files in Specific Directory
```bash
GET /files?path=/directory
```

### 3. Read a File
```bash
GET /files/{filename}
```

Example:
```bash
curl http://localhost:8000/files/test.txt
```

### 4. Create a File
```bash
POST /files/{filename}
Content-Type: application/json

{
  "content": "Hello, Azure File Storage!"
}
```

Example:
```bash
curl -X POST http://localhost:8000/files/test.txt \
  -H "Content-Type: application/json" \
  -d '{"content": "Hello, Azure File Storage!"}'
```

Or with raw body:
```bash
curl -X POST http://localhost:8000/files/test.txt \
  -H "Content-Type: text/plain" \
  -d "Hello, Azure File Storage!"
```

### 5. Update a File
```bash
PUT /files/{filename}
Content-Type: application/json

{
  "content": "Updated content"
}
```

Example:
```bash
curl -X PUT http://localhost:8000/files/test.txt \
  -H "Content-Type: text/plain" \
  -d "Updated content"
```

### 6. Delete a File
```bash
DELETE /files/{filename}
```

Example:
```bash
curl -X DELETE http://localhost:8000/files/test.txt
```

### 7. Create a Directory
```bash
POST /directories/{directory-name}
```

Example:
```bash
curl -X POST http://localhost:8000/directories/mydir
```

### 8. List Directory Contents
```bash
GET /directories/{directory-name}
```

Example:
```bash
curl http://localhost:8000/directories/mydir
```

### 9. Delete a Directory
```bash
DELETE /directories/{directory-name}
```

Example:
```bash
curl -X DELETE http://localhost:8000/directories/mydir
```

### 10. Copy a File
```bash
POST /copy
Content-Type: application/json

{
  "source": "source-file.txt",
  "destination": "destination-file.txt"
}
```

Example:
```bash
curl -X POST http://localhost:8000/copy \
  -H "Content-Type: application/json" \
  -d '{"source": "test.txt", "destination": "test-copy.txt"}'
```

### 11. Get File/Directory Information
```bash
GET /stat/{name}
```

Example:
```bash
curl http://localhost:8000/stat/test.txt
```

## Testing Workflow

1. **Create a directory:**
   ```bash
   curl -X POST http://localhost:8000/directories/testdir
   ```

2. **Create a file:**
   ```bash
   curl -X POST http://localhost:8000/files/testdir/hello.txt \
     -H "Content-Type: application/json" \
     -d '{"content": "Hello from Azure File Storage!"}'
   ```

3. **Read the file:**
   ```bash
   curl http://localhost:8000/files/testdir/hello.txt
   ```

4. **List directory contents:**
   ```bash
   curl http://localhost:8000/directories/testdir
   ```

5. **Get file information:**
   ```bash
   curl http://localhost:8000/stat/testdir/hello.txt
   ```

6. **Copy the file:**
   ```bash
   curl -X POST http://localhost:8000/copy \
     -H "Content-Type: application/json" \
     -d '{"source": "testdir/hello.txt", "destination": "testdir/hello-copy.txt"}'
   ```

7. **Update the file:**
   ```bash
   curl -X PUT http://localhost:8000/files/testdir/hello.txt \
     -H "Content-Type: application/json" \
     -d '{"content": "Updated content!"}'
   ```

8. **Delete files:**
   ```bash
   curl -X DELETE http://localhost:8000/files/testdir/hello.txt
   curl -X DELETE http://localhost:8000/files/testdir/hello-copy.txt
   ```

9. **Delete directory:**
   ```bash
   curl -X DELETE http://localhost:8000/directories/testdir
   ```

## Features Demonstrated

- ✅ File creation and writing
- ✅ File reading
- ✅ File updating
- ✅ File deletion
- ✅ Directory creation
- ✅ Directory listing
- ✅ Directory deletion
- ✅ File copying
- ✅ File/directory metadata (stat)
- ✅ Nested directory support
- ✅ Error handling

## Notes

- Azure File Storage supports native directory structures (unlike S3/GCS which use flat structures)
- All operations are performed on the configured Azure File Share
- The implementation automatically retries connection if the initial connection fails
- File paths are relative to the root of the file share

## Troubleshooting

1. **Connection errors**: Ensure your Azure Storage Account credentials are correct and the File Share exists
2. **Permission errors**: Verify that your storage account key has the necessary permissions
3. **File not found**: Check that the file path is correct and the file exists in the share
4. **Endpoint issues**: If using a custom endpoint, ensure it's correctly formatted

## References

- [Azure File Storage Documentation](https://docs.microsoft.com/azure/storage/files/)
- [GoFr File Storage Documentation](https://gofr.dev/docs/advanced-guide/handling-file)
- [Azure Storage Account Setup](https://docs.microsoft.com/azure/storage/common/storage-account-create)

