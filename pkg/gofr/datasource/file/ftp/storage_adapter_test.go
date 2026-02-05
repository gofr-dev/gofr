package ftp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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
