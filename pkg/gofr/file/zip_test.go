package file

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errTest = errors.New("mock read error")
)

func TestNewZip(t *testing.T) {
	// Create some mock content for the ZIP file
	content := []byte("file1.txt content")
	zipContent := bytes.NewBuffer(nil)
	// Create a new ZIP file using the mock content
	zipWriter := zip.NewWriter(zipContent)
	defer zipWriter.Close()

	// Add a file to the ZIP archive
	fileWriter, err := zipWriter.Create("file1.txt")
	if err != nil {
		t.Fatalf("Error creating file in ZIP: %v", err)
	}

	_, err = fileWriter.Write(content)
	if err != nil {
		t.Fatalf("Error writing to file in ZIP: %v", err)
	}

	// Close the ZIP writer
	err = zipWriter.Close()
	if err != nil {
		t.Fatalf("Error closing ZIP writer: %v", err)
	}

	// Create a new Zip instance from the ZIP content
	z, err := NewZip(zipContent.Bytes())
	require.NoError(t, err, "Error creating Zip instance")

	// Check if the Zip struct contains the expected files
	expectedFiles := map[string]file{
		"file1.txt": {name: "file1.txt", content: content, isDir: false, size: int64(len(content))},
	}

	assert.Equal(t, expectedFiles, z.Files, "Unexpected files in Zip struct")
}

func TestNewZipError(t *testing.T) {
	input := []byte(``)

	z, err := NewZip(input)

	assert.Nil(t, z)
	require.Error(t, err)
	assert.Equal(t, zip.ErrFormat, err)
}

func TestCreateLocalCopies_Success(t *testing.T) {
	mockZip := &Zip{
		// Create a Zip instance with some mock data
		Files: map[string]file{
			"file1.txt":      {name: "file1.txt", content: []byte("File 1 content"), isDir: false, size: 13},
			"dir1/file2.txt": {name: "dir1/file2.txt", content: []byte("File 2 content"), isDir: false, size: 13},
		},
	}

	destDir := "test"
	defer os.RemoveAll(destDir)

	if err := mockZip.CreateLocalCopies(destDir); err != nil {
		t.Fatalf("Error creating local copies: %v", err)
	}

	// Verify that the files were created
	expectedFiles := []string{"file1.txt", "dir1/file2.txt"}
	for _, filename := range expectedFiles {
		destPath := filepath.Join(destDir, filename)
		_, err := os.Stat(destPath)

		if os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", destPath)
		} else if err != nil {
			t.Errorf("Error checking file %s: %v", destPath, err)
		}
	}
}

func TestCopyToBuffer(t *testing.T) {
	// Test when size is within limits
	t.Run("WithinSizeLimit", func(t *testing.T) {
		testData := "This is a test data"
		buffer := bytes.NewBufferString(testData)
		mock := &mockReadCloser{Buffer: buffer}

		buf, err := copyToBuffer(mock, uint64(len(testData)))
		require.NoError(t, err)
		assert.Equal(t, testData, buf.String())
	})

	// Test when size exceeds the maximum allowed size
	t.Run("ExceedsMaxSize", func(t *testing.T) {
		testData := "This is a test data"
		buffer := bytes.NewBufferString(testData)
		mock := &mockReadCloser{Buffer: buffer}

		_, err := copyToBuffer(mock, maxFileSize+1)
		require.Error(t, err)
		assert.Equal(t, errMaxFileSize, err)
	})

	// Test when an error occurs during copying
	t.Run("CopyError", func(t *testing.T) {
		// Create a mock reader that always returns an error
		mock := &mockReadCloser{err: errTest}

		_, err := copyToBuffer(mock, 10)
		require.Error(t, err)
		assert.Equal(t, errTest, err)
	})
}

// mockReadCloser is a mock implementation of io.Reader for testing error conditions.
type mockReadCloser struct {
	*bytes.Buffer
	err error
}

func (m *mockReadCloser) Read(p []byte) (int, error) {
	if m.err != nil {
		return 0, m.err
	}

	return m.Buffer.Read(p)
}

func (*mockReadCloser) Close() error {
	return nil
}

func TestCreateLocalCopies_WithDirectory(t *testing.T) {
	mockZip := &Zip{
		Files: map[string]file{
			"dir1/": {name: "dir1/", isDir: true},
		},
	}

	destDir := "test-dir"
	defer os.RemoveAll(destDir)

	err := mockZip.CreateLocalCopies(destDir)
	require.NoError(t, err)

	// Check if the directory exists
	expectedDir := filepath.Join(destDir, "dir1")
	info, err := os.Stat(expectedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "Expected dir1 to be a directory")
}
func TestCreateLocalCopies_Failure(t *testing.T) {
	mockZip := &Zip{
		Files: map[string]file{
			string([]byte{0x00}): {name: string([]byte{0x00}), content: []byte("invalid"), isDir: false, size: 7}, // Invalid path
		},
	}

	err := mockZip.CreateLocalCopies("test-bad")
	require.Error(t, err, "Expected error when creating file with invalid name")
}

func TestNewZip_PathTraversal_Success(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "valid filename",
			filename: "file.txt",
		},
		{
			name:     "valid nested path",
			filename: "dir/subdir/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipContent := bytes.NewBuffer(nil)
			zipWriter := zip.NewWriter(zipContent)

			fileWriter, err := zipWriter.Create(tt.filename)
			require.NoError(t, err)

			_, err = fileWriter.Write([]byte("test content"))
			require.NoError(t, err)

			err = zipWriter.Close()
			require.NoError(t, err)

			z, err := NewZip(zipContent.Bytes())

			require.NoError(t, err)
			assert.NotNil(t, z)
		})
	}
}

func TestNewZip_PathTraversal_Error(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "absolute path unix",
			filename: "/etc/passwd",
		},
		{
			name:     "path traversal with parent directory",
			filename: "../etc/passwd",
		},
		{
			name:     "path traversal with multiple parent dirs",
			filename: "../../../../../../etc/passwd",
		},
		{
			name:     "path traversal hidden in path",
			filename: "foo/../../../etc/passwd",
		},
		{
			name:     "double dot only",
			filename: "..",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipContent := bytes.NewBuffer(nil)
			zipWriter := zip.NewWriter(zipContent)

			fileWriter, err := zipWriter.Create(tt.filename)
			require.NoError(t, err)

			_, err = fileWriter.Write([]byte("test content"))
			require.NoError(t, err)

			err = zipWriter.Close()
			require.NoError(t, err)

			z, err := NewZip(zipContent.Bytes())

			require.Error(t, err)
			require.ErrorIs(t, err, errPathTraversal)
			assert.Contains(t, err.Error(), tt.filename)
			assert.Nil(t, z)
		})
	}
}

func TestCreateLocalCopies_PathTraversal_Success(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "valid filename",
			filename: "file.txt",
		},
		{
			name:     "valid nested path",
			filename: "dir/subdir/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destDir := t.TempDir()

			mockZip := &Zip{
				Files: map[string]file{
					tt.filename: {name: tt.filename, content: []byte("test content"), isDir: false, size: 12},
				},
			}

			err := mockZip.CreateLocalCopies(destDir)

			require.NoError(t, err)

			expectedPath := filepath.Join(destDir, tt.filename)
			_, statErr := os.Stat(expectedPath)
			assert.NoError(t, statErr, "Expected file to exist at %s", expectedPath)
		})
	}
}

func TestCreateLocalCopies_PathTraversal_Error(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "path traversal with parent directory",
			filename: "../etc/passwd",
		},
		{
			name:     "path traversal with multiple parent dirs",
			filename: "../../../../../../etc/passwd",
		},
		{
			name:     "path traversal hidden in path",
			filename: "foo/../../../etc/passwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destDir := t.TempDir()

			mockZip := &Zip{
				Files: map[string]file{
					tt.filename: {name: tt.filename, content: []byte("test content"), isDir: false, size: 12},
				},
			}

			err := mockZip.CreateLocalCopies(destDir)

			require.Error(t, err)
			require.ErrorIs(t, err, errPathTraversal)
			assert.Contains(t, err.Error(), tt.filename)
		})
	}
}
