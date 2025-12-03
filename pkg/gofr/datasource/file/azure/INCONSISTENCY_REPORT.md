# Azure File Storage vs Local Filesystem Inconsistency Report

## Executive Summary

This report documents inconsistencies between Azure File Storage implementation and local filesystem behavior, as well as differences with other cloud storage implementations (S3, GCS). The goal is to identify areas where Azure File Storage behavior deviates from expected filesystem semantics and recommend fixes to ensure consistent behavior across all storage backends.

---

## 1. Directory Creation on File Write

### Issue
**Azure File Storage does not automatically create parent directories when creating files in subdirectories.**

### Current Behavior

#### Local Filesystem
- ✅ **Automatically creates parent directories** when creating files
- Implementation: `local_fs.go:79` - `os.MkdirAll(filepath.Dir(name), dirPerm)` is called in `NewWriter`

#### Azure File Storage
- ❌ **Does NOT create parent directories** automatically
- Implementation: `storage_adapter.go:172-184` - `NewWriter` directly creates the file without ensuring parent directories exist
- **Impact**: Files can be created, but parent directories don't appear in directory listings until explicitly created

#### S3
- ⚠️ **Requires parent directories to exist** before creating files
- Implementation: `s3/fs.go:135-150` - Checks if parent path exists, returns error if not
- **Impact**: More restrictive - fails if parent doesn't exist

#### GCS
- ✅ **Automatically creates parent directories** (via CommonFileSystem)
- Uses `CommonFileSystem.MkdirAll` pattern

### Recommended Fix

**Priority: HIGH**

Modify `NewWriter` in `storage_adapter.go` to automatically create parent directories before creating files:

```go
func (s *storageAdapter) NewWriter(ctx context.Context, name string) io.WriteCloser {
	if name == "" {
		return &failWriter{err: errEmptyObjectName}
	}

	// Ensure parent directories exist (Azure requires explicit directory creation for listings)
	if dirPath := getParentDir(name); dirPath != "" {
		// Create parent directories using directory client
		dirClient := s.shareClient.NewDirectoryClient(dirPath)
		_, err := dirClient.Create(ctx, nil)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			// Log but don't fail - file creation might still work
			// Parent dirs might be created implicitly by Azure
		}
	}

	fileClient, err := s.getFileClient(name)
	if err != nil {
		return &failWriter{err: err}
	}

	// Create file if it doesn't exist
	_, _ = fileClient.Create(ctx, 0, nil)

	return &azureWriter{
		ctx:        ctx,
		fileClient: fileClient,
		name:       name,
		// ... rest of implementation
	}
}
```

**Alternative Approach**: Handle this at the `CommonFileSystem` level in `Create()` method, but this would require changes to the common implementation.

---

## 2. Directory Visibility in Listings

### Issue
**Directories created implicitly (when files are created in subdirectories) do not appear in directory listings until explicitly created.**

### Current Behavior

#### Local Filesystem
- ✅ Directories appear in listings immediately after files are created in them
- Directories are real filesystem entities

#### Azure File Storage
- ❌ Directories only appear in listings if explicitly created via `Mkdir` or `MkdirAll`
- Azure File Storage requires explicit directory creation for directory entries to appear in `ListFilesAndDirectoriesPager` results
- **Impact**: When you create `dir1/file.txt`, the file exists but `dir1` doesn't show up in root directory listings

#### S3
- ⚠️ Similar behavior - directories are pseudo-entities (prefix markers)
- Directories appear if directory markers are created

#### GCS
- ⚠️ Similar behavior - directories are prefix-based

### Recommended Fix

**Priority: HIGH**

This is directly related to Issue #1. By automatically creating parent directories in `NewWriter`, this issue will be resolved.

**Additional Consideration**: Consider creating directory markers for all parent directories in the path when creating a file, not just the immediate parent.

---

## 3. Error Handling for Non-Existent Directories

### Issue
**Azure File Storage behavior differs when attempting to create files in non-existent directories.**

### Current Behavior

#### Local Filesystem
- ✅ Automatically creates parent directories (no error)

#### Azure File Storage
- ⚠️ File creation may succeed, but directory won't appear in listings
- No explicit error is returned, but behavior is inconsistent

#### S3
- ❌ Returns explicit error if parent directory doesn't exist
- `s3/fs.go:148` - Returns `ErrOperationNotPermitted`

### Recommended Fix

**Priority: MEDIUM**

Ensure consistent error handling:
- Option A: Auto-create directories (matches local filesystem) - **RECOMMENDED**
- Option B: Return explicit error if parent doesn't exist (matches S3 behavior)

Given that Azure File Storage supports real directory structures (unlike S3's flat structure), Option A is more appropriate.

---

## 4. Directory Creation Semantics

### Issue
**Azure File Storage directory creation behavior differs from local filesystem.**

### Current Behavior

#### Local Filesystem
- ✅ `Mkdir` creates single directory
- ✅ `MkdirAll` creates all parent directories recursively
- Uses `os.Mkdir` and `os.MkdirAll`

#### Azure File Storage
- ✅ Uses `CommonFileSystem.Mkdir` and `MkdirAll` (correct)
- ✅ Creates directory markers via `NewWriter` with trailing slash
- ⚠️ But directories created implicitly (via file creation) don't appear in listings

### Recommended Fix

**Priority: LOW** (Already implemented correctly)

The `Mkdir` and `MkdirAll` implementations are correct. The issue is that they're not being called automatically when files are created.

---

## 5. File Creation in Root vs Subdirectories

### Issue
**No difference in behavior, but worth documenting for consistency.**

### Current Behavior

#### All Implementations
- ✅ Creating files in root directory works the same as in subdirectories
- ✅ No special handling needed

### Status
**No action needed** - Behavior is consistent.

---

## 6. Path Normalization

### Issue
**Path handling differences between implementations.**

### Current Behavior

#### Local Filesystem
- Uses `filepath` package (OS-specific path separators)
- Handles `..` and `.` components

#### Azure File Storage
- Uses forward slashes (`/`) consistently
- Handles path normalization in `getFileClient`
- ✅ Correctly handles root directory case

#### S3/GCS
- Use forward slashes
- Handle prefixes correctly

### Recommended Fix

**Priority: LOW**

Current implementation is correct. Azure File Storage correctly normalizes paths and handles edge cases.

---

## Implementation Priority Summary

| Issue | Priority | Impact | Effort | Status |
|-------|----------|--------|--------|--------|
| 1. Auto-create parent directories | HIGH | High - Breaks expected behavior | Medium | 🔴 Needs Fix |
| 2. Directory visibility | HIGH | High - Related to #1 | Low | 🔴 Needs Fix |
| 3. Error handling | MEDIUM | Medium - Consistency | Low | 🟡 Consider |
| 4. Directory creation | LOW | Low - Already correct | N/A | ✅ OK |
| 5. Root vs subdir | N/A | None | N/A | ✅ OK |
| 6. Path normalization | LOW | Low - Already correct | N/A | ✅ OK |

---

## Recommended Implementation Plan

### Phase 1: Critical Fixes (HIGH Priority)

1. **Modify `NewWriter` in `storage_adapter.go`**
   - Add automatic parent directory creation
   - Ensure directories are explicitly created before file creation
   - This fixes both Issue #1 and Issue #2

2. **Add helper function `ensureParentDirectories`**
   - Extracts parent directory path
   - Creates all parent directories recursively
   - Handles errors gracefully (log but don't fail file creation)

### Phase 2: Consistency Improvements (MEDIUM Priority)

3. **Update error handling**
   - Ensure consistent error messages
   - Document behavior differences in code comments

### Phase 3: Documentation (LOW Priority)

4. **Update documentation**
   - Document Azure File Storage-specific behaviors
   - Add examples showing directory creation patterns
   - Note differences from local filesystem where applicable

---

## Code Changes Required

### File: `gofr/pkg/gofr/datasource/file/azure/storage_adapter.go`

#### Change 1: Add helper function
```go
// ensureParentDirectories creates all parent directories for the given file path.
// Azure File Storage requires explicit directory creation for directories to appear in listings.
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

		// Create directory at this level
		dirClient := s.shareClient.NewDirectoryClient(currentPath)
		_, err := dirClient.Create(ctx, nil)
		if err != nil {
			// Check if directory already exists (Azure returns specific error)
			// If it exists, continue; otherwise return error
			if !strings.Contains(err.Error(), "already exists") &&
			   !strings.Contains(err.Error(), "ShareAlreadyExists") {
				return fmt.Errorf("failed to create directory %q: %w", currentPath, err)
			}
		}
	}

	return nil
}

// getParentDir extracts the parent directory path from a file path.
func getParentDir(filePath string) string {
	filePath = strings.TrimPrefix(filePath, "/")
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash == -1 {
		return "" // File is in root
	}
	return filePath[:lastSlash]
}
```

#### Change 2: Modify NewWriter
```go
func (s *storageAdapter) NewWriter(ctx context.Context, name string) io.WriteCloser {
	if name == "" {
		return &failWriter{err: errEmptyObjectName}
	}

	// Ensure parent directories exist (Azure requires explicit directory creation for listings)
	if err := s.ensureParentDirectories(ctx, name); err != nil {
		// Log error but don't fail - file creation might still work
		// Azure may create directories implicitly, but they won't appear in listings
		// This is a best-effort attempt to ensure directories are visible
	}

	fileClient, err := s.getFileClient(name)
	if err != nil {
		return &failWriter{err: err}
	}

	// Create file if it doesn't exist (size 0 initially, will be resized on close)
	_, _ = fileClient.Create(ctx, 0, nil)

	return &azureWriter{
		ctx:        ctx,
		fileClient: fileClient,
		name:       name,
		// ... rest of implementation
	}
}
```

---

## Testing Requirements

### Test Cases to Add

1. **Test automatic parent directory creation**
   - Create file at `dir1/subdir/file.txt`
   - Verify `dir1` appears in root listing
   - Verify `subdir` appears in `dir1` listing
   - Verify `file.txt` exists

2. **Test nested directory creation**
   - Create file at `a/b/c/d/file.txt`
   - Verify all parent directories are created and visible

3. **Test root directory files**
   - Create file at `file.txt` (root)
   - Verify no errors and file is created

4. **Test existing directories**
   - Create directory `dir1` explicitly
   - Create file at `dir1/file.txt`
   - Verify no duplicate directory creation errors

5. **Test directory listing after implicit creation**
   - Create file at `dir1/file.txt` (without explicit directory creation)
   - List root directory
   - Verify `dir1` appears in listing

---

## Comparison Matrix

| Feature | Local FS | Azure | S3 | GCS | Status |
|---------|----------|-------|----|----|--------|
| Auto-create parent dirs on file write | ✅ Yes | ❌ No | ❌ No (fails) | ✅ Yes | 🔴 Inconsistent |
| Directories visible after implicit creation | ✅ Yes | ❌ No | ⚠️ Partial | ⚠️ Partial | 🔴 Inconsistent |
| Explicit directory creation required | ❌ No | ⚠️ For listings | ⚠️ For file creation | ❌ No | 🟡 Varies |
| Real directory structure | ✅ Yes | ✅ Yes | ❌ No (prefixes) | ❌ No (prefixes) | ✅ Correct |
| Path normalization | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Consistent |

---

## Conclusion

The primary inconsistency is that **Azure File Storage does not automatically create parent directories when creating files**, which causes directories to not appear in listings. This differs from local filesystem behavior where parent directories are automatically created.

**Recommended Action**: Implement automatic parent directory creation in `NewWriter` to match local filesystem behavior and ensure directories appear in listings.

This fix will:
- ✅ Make Azure File Storage behavior consistent with local filesystem
- ✅ Ensure directories appear in listings after file creation
- ✅ Improve user experience (no need to manually create directories)
- ✅ Match GCS behavior (which also auto-creates)

**Estimated Effort**: Medium (2-3 hours)
- Add helper functions: ~30 minutes
- Modify `NewWriter`: ~30 minutes
- Add tests: ~1-2 hours
- Documentation: ~30 minutes

