package azure

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockLogger implements the Logger interface for testing.
type MockLogger struct{}

func (*MockLogger) Debug(_ ...any)            {}
func (*MockLogger) Debugf(_ string, _ ...any) {}
func (*MockLogger) Logf(_ string, _ ...any)   {}
func (*MockLogger) Errorf(_ string, _ ...any) {}

// MockMetrics implements the Metrics interface for testing.
type MockMetrics struct{}

func (*MockMetrics) NewHistogram(_, _ string, _ ...float64) {}
func (*MockMetrics) RecordHistogram(_ context.Context, _ string, _ float64, _ ...string) {
}

func TestNew(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs := New(config)
	assert.NotNil(t, fs)
}

func TestUseLogger(_ *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs := New(config)
	logger := &MockLogger{}

	fs.UseLogger(logger)
	// This test just ensures the method doesn't panic
}

func TestUseMetrics(_ *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	fs := New(config)
	metrics := &MockMetrics{}

	fs.UseMetrics(metrics)
	// This test just ensures the method doesn't panic
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
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLocation(t *testing.T) {
	result := getLocation("testshare")
	assert.Equal(t, "azure://testshare", result)
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
			assert.Equal(t, tt.expected, result)
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

	assert.Equal(t, "test.txt", fileInfo.Name())
	assert.Equal(t, int64(1024), fileInfo.Size())
	assert.Equal(t, now, fileInfo.ModTime())
	assert.False(t, fileInfo.IsDir())
	assert.Equal(t, os.FileMode(0644), fileInfo.Mode())

	// Test directory
	dirInfo := &azureFileInfo{
		name:    "testdir",
		size:    0,
		isDir:   true,
		modTime: now,
	}

	assert.True(t, dirInfo.IsDir())
	assert.Equal(t, os.ModeDir|0755, dirInfo.Mode())
}
