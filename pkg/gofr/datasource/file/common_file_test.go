package file

import (
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCommonFile_Name(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
	}{
		{
			name:     "simple file name",
			fileName: "test.txt",
		},
		{
			name:     "file with path",
			fileName: "path/to/file.txt",
		},
		{
			name:     "empty name",
			fileName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{name: tt.fileName}

			assert.Equal(t, tt.fileName, f.Name())
		})
	}
}

func TestCommonFile_Size(t *testing.T) {
	tests := []struct {
		name         string
		size         int64
		expectedSize int64
	}{
		{
			name:         "zero size",
			size:         0,
			expectedSize: 0,
		},
		{
			name:         "small file",
			size:         1024,
			expectedSize: 1024,
		},
		{
			name:         "large file",
			size:         1073741824, // 1GB
			expectedSize: 1073741824,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{size: tt.size}

			assert.Equal(t, tt.expectedSize, f.Size())
		})
	}
}

func TestCommonFile_ModTime(t *testing.T) {
	now := time.Now()
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name         string
		lastModified time.Time
	}{
		{
			name:         "current time",
			lastModified: now,
		},
		{
			name:         "past time",
			lastModified: past,
		},
		{
			name:         "zero time",
			lastModified: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{lastModified: tt.lastModified}

			assert.Equal(t, tt.lastModified, f.ModTime())
		})
	}
}

func TestCommonFile_IsDir(t *testing.T) {
	tests := []struct {
		name        string
		isDir       bool
		contentType string
		expected    bool
	}{
		{
			name:     "explicit directory flag",
			isDir:    true,
			expected: true,
		},
		{
			name:     "explicit file flag",
			isDir:    false,
			expected: false,
		},
		{
			name:        "directory content type",
			isDir:       false,
			contentType: "application/x-directory",
			expected:    true,
		},
		{
			name:        "file content type",
			isDir:       false,
			contentType: "text/plain",
			expected:    false,
		},
		{
			name:        "both directory indicators",
			isDir:       true,
			contentType: "application/x-directory",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{
				isDir:       tt.isDir,
				contentType: tt.contentType,
			}

			assert.Equal(t, tt.expected, f.IsDir())
		})
	}
}

func TestCommonFile_Mode(t *testing.T) {
	tests := []struct {
		name         string
		isDir        bool
		contentType  string
		expectedMode fs.FileMode
	}{
		{
			name:         "directory mode",
			isDir:        true,
			expectedMode: DefaultDirMode,
		},
		{
			name:         "file mode",
			isDir:        false,
			expectedMode: DefaultFileMode,
		},
		{
			name:         "directory by content type",
			contentType:  "application/x-directory",
			expectedMode: DefaultFileMode,
		},
		{
			name:         "regular file content type",
			contentType:  "text/plain",
			expectedMode: DefaultFileMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{
				isDir:       tt.isDir,
				contentType: tt.contentType,
			}

			assert.Equal(t, tt.expectedMode, f.Mode())
		})
	}
}

func TestCommonFile_Sys(t *testing.T) {
	f := &CommonFile{
		name:  "test.txt",
		size:  100,
		isDir: false,
	}

	result := f.Sys()

	assert.Nil(t, result, "Sys() should always return nil for cloud storage")
}

func TestCommonFile_CompleteFileInfo(t *testing.T) {
	now := time.Now()

	f := &CommonFile{
		name:         "document.pdf",
		size:         2048,
		contentType:  "application/pdf",
		lastModified: now,
		isDir:        false,
	}

	assert.Equal(t, "document.pdf", f.Name())
	assert.Equal(t, int64(2048), f.Size())
	assert.Equal(t, now, f.ModTime())
	assert.False(t, f.IsDir())
	assert.Equal(t, DefaultFileMode, f.Mode())
	assert.Nil(t, f.Sys())
}

func TestCommonFile_CompleteDirectoryInfo(t *testing.T) {
	now := time.Now()

	f := &CommonFile{
		name:         "mydir",
		size:         0,
		contentType:  "application/x-directory",
		lastModified: now,
		isDir:        true,
	}

	assert.Equal(t, "mydir", f.Name())
	assert.Equal(t, int64(0), f.Size())
	assert.Equal(t, now, f.ModTime())
	assert.True(t, f.IsDir())
	assert.Equal(t, DefaultDirMode, f.Mode())
	assert.Nil(t, f.Sys())
}
