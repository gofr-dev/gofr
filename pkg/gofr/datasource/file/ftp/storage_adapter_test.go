package ftp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	filedriver "github.com/goftp/file-driver"
	ftpserver "github.com/goftp/server"
	"github.com/jlaffaye/ftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errTest550      = errors.New("550 file not found")
	errTest551      = errors.New("551 File not available")
	errTestNotFound = errors.New("requested file not found")
	errTestTimeout  = errors.New("connection timeout")
	errTestGeneric  = errors.New("test error")
)

// Test helpers

func setupTestFTPServer(t *testing.T) (server *ftpserver.Server, tmpDir string, cleanup func()) {
	t.Helper()

	tmpDir = t.TempDir()

	factory := &filedriver.FileDriverFactory{
		RootPath: tmpDir,
		Perm:     ftpserver.NewSimplePerm("test", "test"),
	}

	opts := &ftpserver.ServerOpts{
		Factory:  factory,
		Port:     0,
		Hostname: "127.0.0.1",
		Auth:     &ftpserver.SimpleAuth{Name: "test", Password: "test"},
	}

	server = ftpserver.NewServer(opts)

	go func() {
		_ = server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	cleanup = func() {
		_ = server.Shutdown()
	}

	return server, tmpDir, cleanup
}

func getTestConfig(port int) *Config {
	return &Config{
		Host:      "127.0.0.1",
		Port:      port,
		User:      "test",
		Password:  "test",
		RemoteDir: "",
	}
}

func createTestFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()

	filePath := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(filePath, content, 0600))

	return filePath
}

// Connect Tests

func TestStorageAdapter_Connect_NilConfig(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.Connect(context.Background())

	require.Error(t, err)
	require.ErrorIs(t, err, errFTPConfigNil)
}

func TestStorageAdapter_Connect_AlreadyConnected(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	err := adapter.Connect(context.Background())

	require.NoError(t, err)
}

func TestStorageAdapter_Connect_Success(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}

	err := adapter.Connect(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, adapter.conn)
}

// NewReader Tests

func TestStorageAdapter_NewReader_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	reader, err := adapter.NewReader(context.Background(), "")

	require.Error(t, err)
	assert.Nil(t, reader)
	require.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_NewReader_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{}}

	reader, err := adapter.NewReader(context.Background(), "test.txt")

	require.Error(t, err)
	assert.Nil(t, reader)
	require.ErrorIs(t, err, errFTPClientNotInitialized)
}

func TestStorageAdapter_NewReader_Success(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	testData := []byte("hello world")
	createTestFile(t, tmpDir, "test.txt", testData)

	reader, err := adapter.NewReader(context.Background(), "test.txt")

	require.NoError(t, err)
	require.NotNil(t, reader)

	defer reader.Close()

	data, readErr := io.ReadAll(reader)
	require.NoError(t, readErr)
	assert.Equal(t, testData, data)
}

func TestStorageAdapter_NewReader_NotFound(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	reader, err := adapter.NewReader(context.Background(), "missing.txt")

	require.Error(t, err)
	assert.Nil(t, reader)
	require.ErrorIs(t, err, errObjectNotFound)
}

// NewRangeReader Tests

func TestStorageAdapter_NewRangeReader_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	reader, err := adapter.NewRangeReader(context.Background(), "", 0, 10)

	require.Error(t, err)
	assert.Nil(t, reader)
	require.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_NewRangeReader_NegativeOffset(t *testing.T) {
	adapter := &storageAdapter{}

	reader, err := adapter.NewRangeReader(context.Background(), "test.txt", -1, 10)

	require.Error(t, err)
	assert.Nil(t, reader)
	require.ErrorIs(t, err, errInvalidOffset)
}

func TestStorageAdapter_NewRangeReader_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{}}

	reader, err := adapter.NewRangeReader(context.Background(), "test.txt", 0, 10)

	require.Error(t, err)
	assert.Nil(t, reader)
	require.ErrorIs(t, err, errFTPClientNotInitialized)
}

func TestStorageAdapter_NewRangeReader_Success(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	testData := []byte("hello world")
	createTestFile(t, tmpDir, "range.txt", testData)

	reader, err := adapter.NewRangeReader(context.Background(), "range.txt", 6, 5)

	require.NoError(t, err)
	require.NotNil(t, reader)

	defer reader.Close()

	data, readErr := io.ReadAll(reader)
	require.NoError(t, readErr)
	assert.Equal(t, "world", string(data))
}

func TestStorageAdapter_NewRangeReader_NotFound(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	reader, err := adapter.NewRangeReader(context.Background(), "missing.txt", 0, 10)

	require.Error(t, err)
	assert.Nil(t, reader)
	require.ErrorIs(t, err, errObjectNotFound)
}

// NewWriter Tests

func TestStorageAdapter_NewWriter_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	writer := adapter.NewWriter(context.Background(), "")

	n, err := writer.Write([]byte("test"))

	assert.Equal(t, 0, n)
	require.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_NewWriter_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{}}

	writer := adapter.NewWriter(context.Background(), "test.txt")

	n, err := writer.Write([]byte("test"))

	assert.Equal(t, 0, n)
	require.ErrorIs(t, err, errFTPClientNotInitialized)
}

func TestStorageAdapter_NewWriter_Success(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	testData := []byte("hello world")
	writer := adapter.NewWriter(context.Background(), "test.txt")

	n, err := writer.Write(testData)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)
	require.NoError(t, writer.Close())

	filePath := filepath.Join(tmpDir, "test.txt")
	assert.FileExists(t, filePath)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	assert.Equal(t, testData, data)
}

// FTPWriter Tests

func TestFTPWriter_Write_Success(t *testing.T) {
	writer := &ftpWriter{buffer: &bytes.Buffer{}}

	data := []byte("test data")
	n, err := writer.Write(data)

	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, data, writer.buffer.Bytes())
}

func TestFTPWriter_Write_AfterClose(t *testing.T) {
	writer := &ftpWriter{buffer: &bytes.Buffer{}, closed: true}

	n, err := writer.Write([]byte("test"))

	assert.Equal(t, 0, n)
	require.ErrorIs(t, err, errWriterAlreadyClosed)
}

func TestFTPWriter_Close_AlreadyClosed(t *testing.T) {
	writer := &ftpWriter{buffer: &bytes.Buffer{}, closed: true}

	err := writer.Close()

	require.NoError(t, err)
}

// FailWriter Tests

func TestFailWriter_Write(t *testing.T) {
	fw := &failWriter{err: errTestGeneric}

	n, err := fw.Write([]byte("test"))

	assert.Equal(t, 0, n)
	require.ErrorIs(t, err, errTestGeneric)
}

func TestFailWriter_Close(t *testing.T) {
	fw := &failWriter{err: errTestGeneric}

	err := fw.Close()

	require.ErrorIs(t, err, errTestGeneric)
}

// DeleteObject Tests

func TestStorageAdapter_DeleteObject_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.DeleteObject(context.Background(), "")

	require.Error(t, err)
	require.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_DeleteObject_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{}}

	err := adapter.DeleteObject(context.Background(), "test.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, errFTPClientNotInitialized)
}

func TestStorageAdapter_DeleteObject_Success(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	testFile := createTestFile(t, tmpDir, "delete-me.txt", []byte("test"))

	err := adapter.DeleteObject(context.Background(), "delete-me.txt")

	require.NoError(t, err)
	assert.NoFileExists(t, testFile)
}

func TestStorageAdapter_DeleteObject_NotFound(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	err := adapter.DeleteObject(context.Background(), "missing.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, errObjectNotFound)
}

// CopyObject Tests

func TestStorageAdapter_CopyObject_EmptySource(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.CopyObject(context.Background(), "", "dest.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, errEmptySourceOrDest)
}

func TestStorageAdapter_CopyObject_EmptyDestination(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.CopyObject(context.Background(), "source.txt", "")

	require.Error(t, err)
	require.ErrorIs(t, err, errEmptySourceOrDest)
}

func TestStorageAdapter_CopyObject_SameSourceAndDest(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.CopyObject(context.Background(), "file.txt", "file.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, errSameSourceAndDest)
}

func TestStorageAdapter_CopyObject_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{}}

	err := adapter.CopyObject(context.Background(), "source.txt", "dest.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, errFTPClientNotInitialized)
}

func TestStorageAdapter_CopyObject_Success(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	sourceData := []byte("copy me")
	createTestFile(t, tmpDir, "source.txt", sourceData)

	err := adapter.CopyObject(context.Background(), "source.txt", "dest.txt")

	require.NoError(t, err)

	destFile := filepath.Join(tmpDir, "dest.txt")
	assert.FileExists(t, destFile)

	data, readErr := os.ReadFile(destFile)
	require.NoError(t, readErr)
	assert.Equal(t, sourceData, data)
}

func TestStorageAdapter_CopyObject_SourceNotFound(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	err := adapter.CopyObject(context.Background(), "missing.txt", "dest.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, errObjectNotFound)
}

// StatObject Tests

func TestStorageAdapter_StatObject_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	info, err := adapter.StatObject(context.Background(), "")

	require.Error(t, err)
	assert.Nil(t, info)
	require.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_StatObject_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{}}

	info, err := adapter.StatObject(context.Background(), "test.txt")

	require.Error(t, err)
	assert.Nil(t, info)
	require.ErrorIs(t, err, errFTPClientNotInitialized)
}

func TestStorageAdapter_StatObject_Success(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	testData := []byte("test content")
	createTestFile(t, tmpDir, "stat-test.txt", testData)

	info, err := adapter.StatObject(context.Background(), "stat-test.txt")

	require.NoError(t, err)
	assert.Equal(t, "stat-test.txt", info.Name)
	assert.Equal(t, int64(len(testData)), info.Size)
	assert.False(t, info.IsDir)
}

func TestStorageAdapter_StatObject_NotFound(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	info, err := adapter.StatObject(context.Background(), "missing.txt")

	require.Error(t, err)
	assert.Nil(t, info)
	require.ErrorIs(t, err, errObjectNotFound)
}

// ListObjects Tests

func TestStorageAdapter_ListObjects_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{}}

	objects, err := adapter.ListObjects(context.Background(), "prefix/")

	require.Error(t, err)
	assert.Nil(t, objects)
	require.ErrorIs(t, err, errFTPClientNotInitialized)
}

func TestStorageAdapter_ListObjects_Success(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	createTestFile(t, tmpDir, "file1.txt", []byte("1"))
	createTestFile(t, tmpDir, "file2.txt", []byte("2"))

	objects, err := adapter.ListObjects(context.Background(), "")

	require.NoError(t, err)
	assert.Len(t, objects, 2)
	assert.Contains(t, objects, "file1.txt")
	assert.Contains(t, objects, "file2.txt")
}

func TestStorageAdapter_ListObjects_EmptyDirectory(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	objects, err := adapter.ListObjects(context.Background(), "")

	require.NoError(t, err)
	assert.Empty(t, objects)
}

// ListDir Tests

func TestStorageAdapter_ListDir_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{}}

	files, dirs, err := adapter.ListDir(context.Background(), "prefix/")

	require.Error(t, err)
	assert.Nil(t, files)
	assert.Nil(t, dirs)
	require.ErrorIs(t, err, errFTPClientNotInitialized)
}

func TestStorageAdapter_ListDir_Success(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(context.Background()))

	createTestFile(t, tmpDir, "file.txt", []byte("test"))

	files, dirs, err := adapter.ListDir(context.Background(), "")

	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "file.txt", files[0].Name)
	assert.Empty(t, dirs)
}

// Helper Function Tests - Table Driven

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name      string
		remoteDir string
		path      string
		expected  string
	}{
		{
			name:      "with_remote_dir",
			remoteDir: "/uploads",
			path:      "file.txt",
			expected:  "/uploads/file.txt",
		},
		{
			name:      "root_remote_dir",
			remoteDir: "/",
			path:      "file.txt",
			expected:  "file.txt",
		},
		{
			name:      "empty_remote_dir",
			remoteDir: "",
			path:      "file.txt",
			expected:  "file.txt",
		},
		{
			name:      "nested_path",
			remoteDir: "/base",
			path:      "subdir/file.txt",
			expected:  "/base/subdir/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &storageAdapter{cfg: &Config{RemoteDir: tt.remoteDir}}

			result := adapter.buildPath(tt.path)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveDirPath(t *testing.T) {
	tests := []struct {
		name      string
		remoteDir string
		prefix    string
		expected  string
	}{
		{
			name:      "empty_prefix",
			remoteDir: "/uploads",
			prefix:    "",
			expected:  "/uploads",
		},
		{
			name:      "dot_prefix",
			remoteDir: "/uploads",
			prefix:    ".",
			expected:  "/uploads",
		},
		{
			name:      "with_prefix",
			remoteDir: "/uploads",
			prefix:    "subdir",
			expected:  "/uploads/subdir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &storageAdapter{cfg: &Config{RemoteDir: tt.remoteDir}}

			result := adapter.resolveDirPath(tt.prefix)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildDirPrefix(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		prefix   string
		expected string
	}{
		{
			name:     "with_prefix",
			dirName:  "subdir",
			prefix:   "parent",
			expected: "parent/subdir/",
		},
		{
			name:     "empty_prefix",
			dirName:  "subdir",
			prefix:   "",
			expected: "subdir/",
		},
		{
			name:     "dot_prefix",
			dirName:  "subdir",
			prefix:   ".",
			expected: "subdir/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &storageAdapter{}

			result := adapter.buildDirPrefix(tt.dirName, tt.prefix)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessEntries(t *testing.T) {
	adapter := &storageAdapter{}

	tests := []struct {
		name          string
		entries       []*ftp.Entry
		prefix        string
		expectedFiles int
		expectedDirs  int
	}{
		{
			name: "mixed_content",
			entries: []*ftp.Entry{
				{Name: "file.txt", Type: ftp.EntryTypeFile, Size: 1024},
				{Name: "subdir", Type: ftp.EntryTypeFolder},
			},
			prefix:        "",
			expectedFiles: 1,
			expectedDirs:  1,
		},
		{
			name:          "empty",
			entries:       []*ftp.Entry{},
			prefix:        "",
			expectedFiles: 0,
			expectedDirs:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, dirs, err := adapter.processEntries(tt.entries, tt.prefix)

			require.NoError(t, err)
			assert.Len(t, files, tt.expectedFiles)
			assert.Len(t, dirs, tt.expectedDirs)
		})
	}
}

func TestHandleListError(t *testing.T) {
	adapter := &storageAdapter{}

	tests := []struct {
		name           string
		err            error
		expectNoError  bool
		expectEmptyRes bool
	}{
		{
			name:           "not_found_error",
			err:            errTest550,
			expectNoError:  true,
			expectEmptyRes: true,
		},
		{
			name:           "other_error",
			err:            errTestTimeout,
			expectNoError:  false,
			expectEmptyRes: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, dirs, err := adapter.handleListError(tt.err, "prefix")

			if tt.expectNoError {
				require.NoError(t, err)
				assert.Empty(t, files)
				assert.Empty(t, dirs)
			} else {
				require.Error(t, err)
				assert.Nil(t, files)
				assert.Nil(t, dirs)
			}
		})
	}
}

func TestIsFTPNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "550_error",
			err:      errTest550,
			expected: true,
		},
		{
			name:     "551_error",
			err:      errTest551,
			expected: true,
		},
		{
			name:     "not_found_text",
			err:      errTestNotFound,
			expected: true,
		},
		{
			name:     "other_error",
			err:      errTestTimeout,
			expected: false,
		},
		{
			name:     "nil_error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFTPNotFoundError(tt.err)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeUint64ToInt64(t *testing.T) {
	const maxInt64 = 1<<63 - 1

	tests := []struct {
		name     string
		input    uint64
		expected int64
	}{
		{
			name:     "valid_value",
			input:    1000,
			expected: 1000,
		},
		{
			name:     "max_int64",
			input:    uint64(maxInt64),
			expected: maxInt64,
		},
		{
			name:     "overflow",
			input:    uint64(maxInt64) + 1,
			expected: maxInt64,
		},
		{
			name:     "zero",
			input:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeUint64ToInt64(tt.input)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetContentType(t *testing.T) {
	tests := []struct {
		name        string
		entry       *ftp.Entry
		contentType string
	}{
		{
			name:        "folder",
			entry:       &ftp.Entry{Name: "folder", Type: ftp.EntryTypeFolder},
			contentType: "application/x-directory",
		},
		{
			name:        "json",
			entry:       &ftp.Entry{Name: "data.json", Type: ftp.EntryTypeFile},
			contentType: "application/json",
		},
		{
			name:        "xml",
			entry:       &ftp.Entry{Name: "config.xml", Type: ftp.EntryTypeFile},
			contentType: "application/xml",
		},
		{
			name:        "text",
			entry:       &ftp.Entry{Name: "readme.txt", Type: ftp.EntryTypeFile},
			contentType: "text/plain; charset=utf-8",
		},
		{
			name:        "csv",
			entry:       &ftp.Entry{Name: "data.csv", Type: ftp.EntryTypeFile},
			contentType: "text/csv",
		},
		{
			name:        "html",
			entry:       &ftp.Entry{Name: "index.html", Type: ftp.EntryTypeFile},
			contentType: "text/html",
		},
		{
			name:        "htm",
			entry:       &ftp.Entry{Name: "page.htm", Type: ftp.EntryTypeFile},
			contentType: "text/html",
		},
		{
			name:        "pdf",
			entry:       &ftp.Entry{Name: "document.pdf", Type: ftp.EntryTypeFile},
			contentType: "application/pdf",
		},
		{
			name:        "zip",
			entry:       &ftp.Entry{Name: "archive.zip", Type: ftp.EntryTypeFile},
			contentType: "application/zip",
		},
		{
			name:        "jpeg",
			entry:       &ftp.Entry{Name: "photo.jpeg", Type: ftp.EntryTypeFile},
			contentType: "image/jpeg",
		},
		{
			name:        "jpg",
			entry:       &ftp.Entry{Name: "photo.jpg", Type: ftp.EntryTypeFile},
			contentType: "image/jpeg",
		},
		{
			name:        "png",
			entry:       &ftp.Entry{Name: "image.png", Type: ftp.EntryTypeFile},
			contentType: "image/png",
		},
		{
			name:        "gif",
			entry:       &ftp.Entry{Name: "animation.gif", Type: ftp.EntryTypeFile},
			contentType: "image/gif",
		},
		{
			name:        "unknown",
			entry:       &ftp.Entry{Name: "file.unknown", Type: ftp.EntryTypeFile},
			contentType: "application/octet-stream",
		},
		{
			name:        "no_extension",
			entry:       &ftp.Entry{Name: "README", Type: ftp.EntryTypeFile},
			contentType: "application/octet-stream",
		},
		{
			name:        "case_insensitive",
			entry:       &ftp.Entry{Name: "FILE.TXT", Type: ftp.EntryTypeFile},
			contentType: "text/plain; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getContentType(tt.entry)

			assert.Equal(t, tt.contentType, result)
		})
	}
}

// LimitedReadCloser Tests

func TestLimitedReadCloser_Read(t *testing.T) {
	data := []byte("hello world")
	reader := strings.NewReader(string(data))
	limited := &limitedReadCloser{
		Reader: io.LimitReader(reader, 5),
		Closer: io.NopCloser(reader),
	}

	buf := make([]byte, 10)
	n, err := limited.Read(buf)

	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", string(buf[:n]))
}

func TestLimitedReadCloser_Close(t *testing.T) {
	reader := io.NopCloser(strings.NewReader("test"))
	limited := &limitedReadCloser{
		Reader: reader,
		Closer: reader,
	}

	err := limited.Close()

	require.NoError(t, err)
}

func TestStorageAdapter_Connect_Errors(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	wrongPassword := getTestConfig(server.Port)
	wrongPassword.Password = "wrong"

	tests := []struct {
		desc   string
		cfg    *Config
		expMsg string
	}{
		{desc: "empty host", cfg: &Config{Port: 21}, expMsg: errFTPConfigInvalid.Error()},
		{desc: "invalid port", cfg: &Config{Host: "127.0.0.1", Port: 0}, expMsg: errFTPConfigInvalid.Error()},
		{desc: "login failure", cfg: wrongPassword, expMsg: `FTP login failed for user "test"`},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			adapter := &storageAdapter{cfg: tc.cfg}

			err := adapter.Connect(t.Context())

			require.ErrorContains(t, err, tc.expMsg)
			assert.Nil(t, adapter.conn)
		})
	}
}

func TestStorageAdapter_ClosedConnection(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	createTestFile(t, tmpDir, "file.txt", []byte("data"))

	tests := []struct {
		desc   string
		call   func(s *storageAdapter) error
		expMsg string
	}{
		{
			desc: "NewReader",
			call: func(s *storageAdapter) error {
				_, err := s.NewReader(t.Context(), "file.txt")
				return err
			},
			expMsg: `failed to create reader for "file.txt"`,
		},
		{
			desc: "NewRangeReader",
			call: func(s *storageAdapter) error {
				_, err := s.NewRangeReader(t.Context(), "file.txt", 1, 2)
				return err
			},
			expMsg: `failed to create reader for "file.txt" at offset 1`,
		},
		{
			desc:   "DeleteObject",
			call:   func(s *storageAdapter) error { return s.DeleteObject(t.Context(), "file.txt") },
			expMsg: `failed to delete object "file.txt"`,
		},
		{
			desc:   "CopyObject",
			call:   func(s *storageAdapter) error { return s.CopyObject(t.Context(), "file.txt", "copy.txt") },
			expMsg: `failed to read source object "file.txt"`,
		},
		{
			desc: "StatObject",
			call: func(s *storageAdapter) error {
				_, err := s.StatObject(t.Context(), "file.txt")
				return err
			},
			expMsg: `failed to get object attrs for "file.txt"`,
		},
		{
			desc: "ListObjects",
			call: func(s *storageAdapter) error {
				_, err := s.ListObjects(t.Context(), "dir")
				return err
			},
			expMsg: `failed to list objects with prefix "dir"`,
		},
		{
			desc: "ListDir",
			call: func(s *storageAdapter) error {
				_, _, err := s.ListDir(t.Context(), "dir")
				return err
			},
			expMsg: `failed to list directory "dir"`,
		},
		{
			desc: "writer Close",
			call: func(s *storageAdapter) error {
				w := s.NewWriter(t.Context(), "new.txt")

				_, _ = w.Write([]byte("data"))

				return w.Close()
			},
			expMsg: `failed to create writer for "new.txt"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
			require.NoError(t, adapter.Connect(t.Context()))
			require.NoError(t, adapter.conn.Quit())

			err := tc.call(adapter)

			require.ErrorContains(t, err, tc.expMsg)
		})
	}
}

func TestStorageAdapter_StatObject_EmptyDirectory(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "empty"), 0o700))

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(t.Context()))

	info, err := adapter.StatObject(t.Context(), "empty")

	require.ErrorIs(t, err, errObjectNotFound)
	assert.Nil(t, info)
}

func TestStorageAdapter_ListWithPrefix(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "sub"), 0o700))
	createTestFile(t, tmpDir, "sub/a.txt", []byte("abc"))

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(t.Context()))

	objects, err := adapter.ListObjects(t.Context(), "sub")
	require.NoError(t, err)
	assert.Equal(t, []string{"sub/a.txt"}, objects)

	infos, dirs, err := adapter.ListDir(t.Context(), "sub")
	require.NoError(t, err)
	assert.Empty(t, dirs)
	require.Len(t, infos, 1)
	assert.Equal(t, "sub/a.txt", infos[0].Name)
	assert.Equal(t, int64(3), infos[0].Size)
}

func TestStorageAdapter_NewRangeReader_NoLength(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	createTestFile(t, tmpDir, "range.txt", []byte("0123456789"))

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(t.Context()))

	reader, err := adapter.NewRangeReader(t.Context(), "range.txt", 7, 0)
	require.NoError(t, err)

	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "789", string(data))
}

func TestStorageAdapter_CopyObject_DestinationWriteFails(t *testing.T) {
	server, tmpDir, cleanup := setupTestFTPServer(t)
	defer cleanup()

	createTestFile(t, tmpDir, "src.txt", []byte("data"))

	adapter := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, adapter.Connect(t.Context()))

	err := adapter.CopyObject(t.Context(), "src.txt", "missing/dir/copy.txt")

	require.ErrorContains(t, err, `failed to write destination object "missing/dir/copy.txt"`)
}

func TestStorageAdapter_ConnectConcurrentWithOperations(t *testing.T) {
	tests := []struct {
		name string
		op   func(a *storageAdapter)
	}{
		{name: "NewReader", op: func(a *storageAdapter) { _, _ = a.NewReader(t.Context(), "missing.txt") }},
		{name: "StatObject", op: func(a *storageAdapter) { _, _ = a.StatObject(t.Context(), "missing.txt") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _, cleanup := setupTestFTPServer(t)
			defer cleanup()

			a := &storageAdapter{cfg: getTestConfig(server.Port)}

			var wg sync.WaitGroup

			wg.Go(func() { assert.NoError(t, a.Connect(t.Context())) })
			wg.Go(func() {
				for range 50 {
					tt.op(a)
				}
			})
			wg.Wait()

			_, err := a.StatObject(t.Context(), "missing.txt")
			require.NotErrorIs(t, err, errFTPClientNotInitialized)
		})
	}
}

func TestStorageAdapter_PublishConn_KeepsExisting(t *testing.T) {
	server, _, cleanup := setupTestFTPServer(t)
	defer cleanup()

	a := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, a.Connect(t.Context()))

	other := &storageAdapter{cfg: getTestConfig(server.Port)}
	require.NoError(t, other.Connect(t.Context()))

	first, second := a.conn, other.conn

	a.publishConn(second)

	assert.Same(t, first, a.conn)
	require.Error(t, second.NoOp(), "redundant connection should be closed")
}
