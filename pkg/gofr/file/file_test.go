package file

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestFile(t *testing.T) {
	// Create a sample file
	content := []byte("This is a test file.")
	f := file{
		name:    "test.txt",
		content: content,
		size:    int64(len(content)),
		isDir:   false,
	}

	// Test GetName method
	assert.Equal(t, "test.txt", f.GetName(), "File name should be 'test.txt'")

	// Test GetSize method
	assert.Equal(t, int64(20), f.GetSize(), "File size should be 20 bytes")

	// Test Bytes method
	assert.Equal(t, content, f.Bytes(), "File content should match")

	// Test IsDir method
	assert.False(t, f.IsDir(), "File should not be a directory")
}
