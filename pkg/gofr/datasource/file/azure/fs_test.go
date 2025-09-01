package azure

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// MockLogger implements the Logger interface for testing.
type MockLogger struct {
	debugLogs  []string
	debugfLogs []string
	logfLogs   []string
	errorfLogs []string
}

func (m *MockLogger) Debug(args ...any) {
	m.debugLogs = append(m.debugLogs, args[0].(string))
}

func (m *MockLogger) Debugf(pattern string, _ ...any) {
	m.debugfLogs = append(m.debugfLogs, pattern)
}

func (m *MockLogger) Logf(pattern string, _ ...any) {
	m.logfLogs = append(m.logfLogs, pattern)
}

func (m *MockLogger) Errorf(pattern string, _ ...any) {
	m.errorfLogs = append(m.errorfLogs, pattern)
}

// MockMetrics implements the Metrics interface for testing.
type MockMetrics struct {
	histograms []string
	records    []string
}

func (m *MockMetrics) NewHistogram(name, _ string, _ ...float64) {
	m.histograms = append(m.histograms, name)
}

func (m *MockMetrics) RecordHistogram(_ context.Context, name string, _ float64, _ ...string) {
	m.records = append(m.records, name)
}

func TestNew(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs := New(config)
	require.NotNil(t, fs)

	// Type require to access config field
	azureFS, ok := fs.(*FileSystem)
	require.True(t, ok)
	require.Equal(t, config, azureFS.config)
}

func TestUseLogger(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs := New(config)
	logger := &MockLogger{}

	fs.UseLogger(logger)

	// Type require to access logger field
	azureFS, ok := fs.(*FileSystem)
	require.True(t, ok)
	require.Equal(t, logger, azureFS.logger)
}

func TestUseMetrics(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs := New(config)
	metrics := &MockMetrics{}

	fs.UseMetrics(metrics)

	// Type require to access metrics field
	azureFS, ok := fs.(*FileSystem)
	require.True(t, ok)
	require.Equal(t, metrics, azureFS.metrics)
}

func TestConnect(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs := New(config)
	logger := &MockLogger{}
	fs.UseLogger(logger)

	// Test Connect without proper Azure credentials (should not panic)
	fs.Connect()

	// Verify that logger was used (it should log debug messages even on failure)
	require.NotEmpty(t, logger.debugfLogs)
}

func TestConnectWithEndpoint(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
		Endpoint:    "https://custom.endpoint.com",
	}

	fs := New(config)
	logger := &MockLogger{}
	fs.UseLogger(logger)

	fs.Connect()

	// Verify that logger was used (it should log debug messages even on failure)
	require.NotEmpty(t, logger.debugfLogs)
}

func TestGetShareName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple path", "share/file.txt", "share"},
		{"nested path", "share/dir/file.txt", "share"},
		{"root path", "/share/file.txt", ""},
		{"empty path", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getShareName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLocation(t *testing.T) {
	result := getLocation("testshare")
	require.Equal(t, "azure://testshare", result)
}

func TestGetFilePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with leading slash", "/file.txt", "file.txt"},
		{"without leading slash", "file.txt", "file.txt"},
		{"nested path", "/dir/file.txt", "dir/file.txt"},
		{"empty path", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFilePath(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestAzureFileInfo(t *testing.T) {
	now := time.Now()
	fileInfo := &azureFileInfo{
		name:    "test.txt",
		size:    1024,
		isDir:   false,
		modTime: now,
	}

	require.Equal(t, "test.txt", fileInfo.Name())
	require.Equal(t, int64(1024), fileInfo.Size())
	require.Equal(t, now, fileInfo.ModTime())
	require.False(t, fileInfo.IsDir())
	require.Equal(t, os.FileMode(0644), fileInfo.Mode())

	// Test directory
	dirInfo := &azureFileInfo{
		name:    "testdir",
		size:    0,
		isDir:   true,
		modTime: now,
	}

	require.True(t, dirInfo.IsDir())
	require.Equal(t, os.ModeDir|0755, dirInfo.Mode())
}

// Test File methods.
func TestFileClose(t *testing.T) {
	f := &File{
		name:    "test.txt",
		body:    nil,
		logger:  &MockLogger{},
		metrics: &MockMetrics{},
	}

	err := f.Close()
	require.NoError(t, err)
}

func TestFileRead(t *testing.T) {
	f := &File{
		name:    "test.txt",
		logger:  &MockLogger{},
		metrics: &MockMetrics{},
	}

	n, err := f.Read([]byte("test"))
	require.Error(t, err)
	require.Equal(t, ErrReadNotImplemented, err)
	require.Equal(t, 0, n)
}

func TestFileReadAt(t *testing.T) {
	f := &File{
		name:    "test.txt",
		offset:  0,
		logger:  &MockLogger{},
		metrics: &MockMetrics{},
	}

	n, err := f.ReadAt([]byte("test"), 10)
	require.Error(t, err)
	require.Equal(t, ErrReadNotImplemented, err)
	require.Equal(t, 0, n)
	require.Equal(t, int64(10), f.offset)
}

func TestFileSeek(t *testing.T) {
	f := &File{
		name:    "test.txt",
		offset:  0,
		size:    100,
		logger:  &MockLogger{},
		metrics: &MockMetrics{},
	}

	// Test SeekStart
	offset, err := f.Seek(10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(10), offset)
	require.Equal(t, int64(10), f.offset)

	// Test SeekCurrent
	offset, err = f.Seek(5, 1)
	require.NoError(t, err)
	require.Equal(t, int64(15), offset)
	require.Equal(t, int64(15), f.offset)

	// Test SeekEnd
	offset, err = f.Seek(-10, 2)
	require.NoError(t, err)
	require.Equal(t, int64(90), offset)
	require.Equal(t, int64(90), f.offset)

	// Test invalid whence
	_, err = f.Seek(0, 99)

	require.Error(t, err)
	require.Equal(t, ErrInvalidWhence, err)
	require.Equal(t, int64(90), f.offset) // Should remain unchanged
}

func TestFileWrite(t *testing.T) {
	f := &File{
		name:    "test.txt",
		logger:  &MockLogger{},
		metrics: &MockMetrics{},
	}

	n, err := f.Write([]byte("test"))
	require.Error(t, err)
	require.Equal(t, ErrWriteNotImplemented, err)
	require.Equal(t, 0, n)
}

func TestFileWriteAt(t *testing.T) {
	f := &File{
		name:    "test.txt",
		offset:  0,
		logger:  &MockLogger{},
		metrics: &MockMetrics{},
	}

	n, err := f.WriteAt([]byte("test"), 10)
	require.Error(t, err)
	require.Equal(t, ErrWriteNotImplemented, err)
	require.Equal(t, 0, n)
	require.Equal(t, int64(10), f.offset)
}

func TestFileName(t *testing.T) {
	f := &File{
		name: "test.txt",
	}

	require.Equal(t, "test.txt", f.name)
}

func TestFileSize(t *testing.T) {
	f := &File{
		size: 1024,
	}

	require.Equal(t, int64(1024), f.size)
}

func TestFileModTime(t *testing.T) {
	now := time.Now()
	f := &File{
		lastModified: now,
	}

	require.Equal(t, now, f.lastModified)
}

func TestFileReadAll(t *testing.T) {
	f := &File{
		name:    "test.txt",
		logger:  &MockLogger{},
		metrics: &MockMetrics{},
	}

	reader, err := f.ReadAll()
	require.NoError(t, err)
	require.NotNil(t, reader)
}

func TestAzureRowReader(t *testing.T) {
	reader := &azureRowReader{
		file: &File{name: "test.txt"},
		read: false,
	}

	// Test Next() first time
	require.True(t, reader.Next())
	require.True(t, reader.read)

	// Test Next() second time
	require.False(t, reader.Next())
}

func TestAzureRowReaderScan(t *testing.T) {
	reader := &azureRowReader{
		file: &File{name: "test.txt"},
		read: false,
	}

	err := reader.Scan(nil)
	require.NoError(t, err)
}

// Test error constants.
func TestErrorConstants(t *testing.T) {
	require.Error(t, ErrCreateDirectoryNotImplemented)
	require.Error(t, ErrDeleteDirectoryNotImplemented)
	require.Error(t, ErrListFilesAndDirectoriesSegmentNotImplemented)
	require.Error(t, ErrCreateFileNotImplemented)
	require.Error(t, ErrDeleteFileNotImplemented)
	require.Error(t, ErrDownloadFileNotImplemented)
	require.Error(t, ErrUploadRangeNotImplemented)
	require.Error(t, ErrGetPropertiesNotImplemented)
	require.Error(t, ErrRemoveNotImplemented)
	require.Error(t, ErrRenameNotImplemented)
	require.Error(t, ErrMkdirNotImplemented)
	require.Error(t, ErrReadDirNotImplemented)
	require.Error(t, ErrChDirNotImplemented)
	require.Error(t, ErrReadNotImplemented)
	require.Error(t, ErrWriteNotImplemented)
	require.Error(t, ErrInvalidWhence)
	require.Error(t, ErrShareClientNotInitialized)
	require.Error(t, ErrDownloadFileRequiresPath)
	require.Error(t, ErrUploadRangeRequiresPath)
	require.Error(t, ErrGetPropertiesRequiresPath)
}

// Test metrics recording.
func TestMetricsRecording(t *testing.T) {
	metrics := &MockMetrics{}
	fs := &FileSystem{
		metrics: metrics,
		config:  &Config{ShareName: "testshare"},
	}

	// Test that metrics are recorded
	fs.sendOperationStats(&FileLog{
		Operation: "TEST",
		Location:  "test",
	}, time.Now())

	require.NotEmpty(t, metrics.records)
}

// Test logger usage.
func TestLoggerUsage(t *testing.T) {
	logger := &MockLogger{}
	fs := &FileSystem{
		logger: logger,
		config: &Config{ShareName: "testshare"},
	}

	// Test that logger is used in Connect method
	fs.Connect()

	// The logger should be used in Connect method
	require.NotEmpty(t, logger.debugfLogs)
}
