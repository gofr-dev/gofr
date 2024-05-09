package local

import (
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/testutil"
	"os"
	"testing"
)

func Test_LocalFileSystemDirectoryCreation(t *testing.T) {
	dirName := "temp!@#$%^&*(123"

	logger := testutil.NewMockLogger(testutil.DEBUGLOG)

	fileStore := New(logger)

	err := fileStore.CreateDir(dirName)
	defer os.RemoveAll(dirName)

	assert.Nil(t, err)

	fInfo, err := os.Stat(dirName)

	assert.Nil(t, err)
	assert.Equal(t, true, fInfo.IsDir())
}
