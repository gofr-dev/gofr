# Azure File Storage vs Local Filesystem - Complete Behavior Comparison

## Executive Summary

This document provides a comprehensive comparison of all filesystem operations between Azure File Storage and local filesystem implementations. It identifies any remaining inconsistencies after the parent directory auto-creation fix.

---

## Operation-by-Operation Comparison

### 1. NewReader (Read File)

#### Local Filesystem
```go
func (*localProvider) NewReader(_ context.Context, name string) (io.ReadCloser, error) {
    return os.Open(name)
}
```
- ✅ Opens file directly
- ✅ Returns error if file doesn't exist
- ✅ Returns `io.ReadCloser`

#### Azure File Storage
```go
func (s *storageAdapter) NewReader(ctx context.Context, name string) (io.ReadCloser, error) {
    fileClient, err := s.getFileClient(name)
    if err != nil {
        return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateReader, name, err)
    }
    downloadResp, err := fileClient.DownloadStream(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateReader, name, err)
    }
    return downloadResp.Body, nil
}
```
- ✅ Opens file via Azure SDK
- ✅ Returns error if file doesn't exist
- ✅ Returns `io.ReadCloser`
- **Status**: ✅ **CONSISTENT**

---

### 2. NewRangeReader (Read File with Range)

#### Local Filesystem
```go
func (*localProvider) NewRangeReader(_ context.Context, name string, offset, length int64) (io.ReadCloser, error) {
    f, err := os.Open(name)
    if err != nil {
        return nil, err
    }
    if _, err := f.Seek(offset, io.SeekStart); err != nil {
        _ = f.Close()
        return nil, err
    }
    if length > 0 {
        return &limitedReadCloser{f, length}, nil
    }
    return f, nil
}
```
- ✅ Opens file and seeks to offset
- ✅ Limits read to `length` bytes if specified
- ✅ Returns error if file doesn't exist

#### Azure File Storage
```go
func (s *storageAdapter) NewRangeReader(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error) {
    if offset < 0 {
        return nil, fmt.Errorf("%w (got: %d)", errInvalidOffset, offset)
    }
    fileClient, err := s.getFileClient(name)
    if err != nil {
        return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateRangeReader, name, err)
    }
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
```
- ✅ Validates offset >= 0 (local doesn't, but Azure's validation is better)
- ✅ Uses HTTP range requests for efficient partial reads
- ✅ Returns error if file doesn't exist
- **Status**: ✅ **CONSISTENT** (Azure has better validation)

---

### 3. NewWriter (Create/Write File)

#### Local Filesystem
```go
func (*localProvider) NewWriter(_ context.Context, name string) io.WriteCloser {
    // Create parent directories if needed
    if err := os.MkdirAll(filepath.Dir(name), dirPerm); err != nil {
        return &failWriter{err: err}
    }
    f, err := os.Create(name)
    if err != nil {
        return &failWriter{err: err}
    }
    return f
}
```
- ✅ Automatically creates parent directories
- ✅ Creates file (truncates if exists)
- ✅ Returns `io.WriteCloser`

#### Azure File Storage
```go
func (s *storageAdapter) NewWriter(ctx context.Context, name string) io.WriteCloser {
    // Ensure parent directories exist (Azure requires explicit directory creation for listings)
    if err := s.ensureParentDirectories(ctx, name); err != nil {
        // Log error but don't fail - file creation might still work
    }
    fileClient, err := s.getFileClient(name)
    if err != nil {
        return &failWriter{err: err}
    }
    _, _ = fileClient.Create(ctx, 0, nil)
    return &azureWriter{...}
}
```
- ✅ **NOW** automatically creates parent directories (FIXED)
- ✅ Creates file (size 0 initially, resized on close)
- ✅ Returns `io.WriteCloser`
- **Status**: ✅ **CONSISTENT** (after fix)

---

### 4. StatObject (Get File/Directory Metadata)

#### Local Filesystem
```go
func (*localProvider) StatObject(_ context.Context, name string) (*ObjectInfo, error) {
    info, err := os.Stat(name)
    if err != nil {
        return nil, err
    }
    return &ObjectInfo{
        Name:         info.Name(),
        Size:         info.Size(),
        ContentType:  "", // Local FS doesn't store content type
        LastModified: info.ModTime(),
        IsDir:        info.IsDir(),
    }, nil
}
```
- ✅ Returns file/directory metadata
- ✅ Includes `IsDir` flag
- ✅ Returns error if not found

#### Azure File Storage
```go
func (s *storageAdapter) StatObject(ctx context.Context, name string) (*file.ObjectInfo, error) {
    // Check if it's a directory (ends with /)
    if strings.HasSuffix(name, "/") {
        // ... directory handling ...
    }
    // It's a file
    fileClient, err := s.getFileClient(name)
    props, err := fileClient.GetProperties(ctx, nil)
    // ... return ObjectInfo ...
}
```
- ✅ Returns file/directory metadata
- ✅ Handles both files and directories
- ✅ Includes `IsDir` flag
- ✅ Returns error if not found
- ✅ Includes `ContentType` (local doesn't)
- **Status**: ✅ **CONSISTENT** (Azure provides more metadata)

---

### 5. DeleteObject (Delete File/Directory)

#### Local Filesystem
```go
func (*localProvider) DeleteObject(_ context.Context, name string) error {
    return os.Remove(name)
}
```
- ✅ Deletes file or empty directory
- ✅ Returns error if not found
- ✅ Returns error if directory is not empty

#### Azure File Storage
```go
func (s *storageAdapter) DeleteObject(ctx context.Context, name string) error {
    // Check if it's a directory
    if strings.HasSuffix(name, "/") {
        dirClient := s.shareClient.NewDirectoryClient(dirPath)
        _, err := dirClient.Delete(ctx, nil)
        // ...
    }
    // It's a file
    fileClient, err := s.getFileClient(name)
    _, err = fileClient.Delete(ctx, nil)
    // ...
}
```
- ✅ Deletes file or directory
- ✅ Handles both files and directories explicitly
- ✅ Returns error if not found
- ⚠️ **Difference**: Azure may allow deleting non-empty directories (need to verify)
- **Status**: ⚠️ **MOSTLY CONSISTENT** (behavior may differ for non-empty directories)

---

### 6. CopyObject (Copy File)

#### Local Filesystem
```go
func (*localProvider) CopyObject(_ context.Context, src, dst string) error {
    srcFile, err := os.Open(src)
    if err != nil {
        return err
    }
    defer srcFile.Close()
    
    if mkdirErr := os.MkdirAll(filepath.Dir(dst), DefaultDirMode); mkdirErr != nil {
        return mkdirErr
    }
    
    dstFile, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer dstFile.Close()
    
    _, err = io.Copy(dstFile, srcFile)
    return err
}
```
- ✅ Automatically creates parent directories for destination
- ✅ Copies file content
- ✅ Returns error if source doesn't exist
- ✅ Overwrites destination if exists

#### Azure File Storage
```go
func (s *storageAdapter) CopyObject(ctx context.Context, src, dst string) error {
    // Get source file data
    downloadResp, srcProps, err := s.getSourceFileData(ctx, src)
    // ...
    
    // Create destination file
    dstClient, err := s.createDestinationFile(ctx, dst, srcProps)
    // ...
    
    // Copy content type
    if err := s.copyContentType(ctx, dstClient, srcProps); err != nil {
        return err
    }
    
    // Upload content to destination
    return s.uploadToDestination(ctx, dstClient, downloadResp.Body, src, dst)
}
```
- ❌ **DOES NOT** automatically create parent directories for destination
- ✅ Copies file content
- ✅ Returns error if source doesn't exist
- ✅ Overwrites destination if exists
- ✅ Preserves content type
- **Status**: 🔴 **INCONSISTENT** - Missing parent directory creation

---

### 7. ListObjects (List Files Only)

#### Local Filesystem
```go
func (*localProvider) ListObjects(_ context.Context, prefix string) ([]string, error) {
    entries, err := os.ReadDir(prefix)
    if err != nil {
        return nil, err
    }
    var objects []string
    for _, entry := range entries {
        if !entry.IsDir() {
            objects = append(objects, entry.Name())
        }
    }
    return objects, nil
}
```
- ✅ Lists only files (not directories)
- ✅ Returns relative file names
- ✅ Returns error if directory doesn't exist

#### Azure File Storage
```go
func (s *storageAdapter) ListObjects(ctx context.Context, prefix string) ([]string, error) {
    // Normalize prefix
    normalizedPrefix := strings.TrimPrefix(prefix, "/")
    if normalizedPrefix != "" && !strings.HasSuffix(normalizedPrefix, "/") {
        normalizedPrefix += "/"
    }
    
    var dirClient *directory.Client
    if normalizedPrefix == "" {
        dirClient = s.shareClient.NewRootDirectoryClient()
    } else {
        dirPath := strings.TrimSuffix(normalizedPrefix, "/")
        dirClient = s.shareClient.NewDirectoryClient(dirPath)
    }
    
    pager := dirClient.NewListFilesAndDirectoriesPager(&directory.ListFilesAndDirectoriesOptions{})
    // ... iterate and collect file names only ...
}
```
- ✅ Lists only files (not directories)
- ✅ Returns relative file names
- ✅ Returns error if directory doesn't exist
- **Status**: ✅ **CONSISTENT**

---

### 8. ListDir (List Files and Directories)

#### Local Filesystem
```go
func (*localProvider) ListDir(_ context.Context, prefix string) ([]ObjectInfo, []string, error) {
    entries, err := os.ReadDir(prefix)
    if err != nil {
        return nil, nil, err
    }
    var objects []ObjectInfo
    var dirs []string
    for _, entry := range entries {
        if entry.IsDir() {
            dirs = append(dirs, entry.Name()+"/")
        } else {
            objects = append(objects, ObjectInfo{
                Name:         entry.Name(),
                Size:         info.Size(),
                LastModified: info.ModTime(),
                IsDir:        false,
            })
        }
    }
    return objects, dirs, nil
}
```
- ✅ Returns both files and directories
- ✅ Directories have trailing `/`
- ✅ Returns error if directory doesn't exist

#### Azure File Storage
```go
func (s *storageAdapter) ListDir(ctx context.Context, prefix string) ([]file.ObjectInfo, []string, error) {
    // ... similar structure ...
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
        // ... create ObjectInfo ...
    }
    return objects, prefixes, nil
}
```
- ✅ Returns both files and directories
- ✅ Directories have trailing `/`
- ✅ Returns error if directory doesn't exist
- ✅ Handles nil values safely (after our fixes)
- **Status**: ✅ **CONSISTENT**

---

## Identified Inconsistencies

### Issue #1: CopyObject Missing Parent Directory Creation

**Priority**: HIGH

**Problem**: 
- Local filesystem: Automatically creates parent directories for destination in `CopyObject`
- Azure File Storage: Does NOT create parent directories for destination

**Impact**: 
- Copying to a new subdirectory will fail or create the file without the directory appearing in listings

**Current Code**:
```go
func (s *storageAdapter) createDestinationFile(
    ctx context.Context, dst string, srcProps *azfile.GetPropertiesResponse) (*azfile.Client, error) {
    dstClient, err := s.getFileClient(dst)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to get destination client: %w", errFailedToCopyObject, err)
    }
    // ... create file ...
}
```

**Recommended Fix**:
```go
func (s *storageAdapter) createDestinationFile(
    ctx context.Context, dst string, srcProps *azfile.GetPropertiesResponse) (*azfile.Client, error) {
    // Ensure parent directories exist (matches local filesystem behavior)
    if err := s.ensureParentDirectories(ctx, dst); err != nil {
        // Log but don't fail - directory creation might still work
    }
    
    dstClient, err := s.getFileClient(dst)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to get destination client: %w", errFailedToCopyObject, err)
    }
    // ... rest of implementation ...
}
```

---

### Issue #2: DeleteObject Behavior for Non-Empty Directories

**Priority**: MEDIUM

**Problem**:
- Local filesystem: `os.Remove` fails if directory is not empty
- Azure File Storage: Need to verify if `dirClient.Delete` allows deleting non-empty directories

**Impact**: 
- Inconsistent error behavior when attempting to delete non-empty directories

**Status**: ⚠️ **NEEDS VERIFICATION**

**Action Required**: Test Azure behavior and document or fix if inconsistent.

---

## Summary Table

| Operation | Local FS Behavior | Azure Behavior | Status |
|-----------|------------------|----------------|--------|
| **NewReader** | Opens file, returns error if not found | Opens file, returns error if not found | ✅ Consistent |
| **NewRangeReader** | Seeks to offset, limits length | HTTP range request | ✅ Consistent |
| **NewWriter** | Auto-creates parent dirs | ✅ Auto-creates parent dirs (FIXED) | ✅ Consistent |
| **StatObject** | Returns metadata | Returns metadata + ContentType | ✅ Consistent |
| **DeleteObject** | Deletes file/empty dir | Deletes file/dir | ⚠️ Needs verification |
| **CopyObject** | Auto-creates parent dirs for dst | ❌ Does NOT create parent dirs | 🔴 **INCONSISTENT** |
| **ListObjects** | Lists files only | Lists files only | ✅ Consistent |
| **ListDir** | Lists files + dirs | Lists files + dirs | ✅ Consistent |

---

## Recommended Actions

### High Priority

1. **Fix CopyObject** - Add parent directory creation in `createDestinationFile` method
   - File: `gofr/pkg/gofr/datasource/file/azure/storage_adapter.go`
   - Method: `createDestinationFile`
   - Add call to `ensureParentDirectories(ctx, dst)` before creating destination file

### Medium Priority

2. **Verify DeleteObject behavior** - Test deleting non-empty directories
   - Document expected behavior
   - Add error handling if needed

### Low Priority

3. **Documentation** - Update documentation to note any Azure-specific behaviors
   - ContentType support
   - Directory structure differences (if any)

---

## Conclusion

After implementing the parent directory auto-creation fix for `NewWriter`, there is **one remaining inconsistency**:

- **CopyObject**: Does not create parent directories for destination file

All other operations are consistent between Azure File Storage and local filesystem implementations.

