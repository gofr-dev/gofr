package file

import (
	"gofr.dev/pkg/gofr/logging"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LocalFileSystemDirectoryCreation(t *testing.T) {
	dirName := "temp!@#$%^&*(123"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	err := fileStore.CreateDir(dirName)
	defer os.RemoveAll(dirName)

	assert.Nil(t, err)

	fInfo, err := fileStore.Stat(dirName)

	assert.Nil(t, err)
	assert.Equal(t, true, fInfo.IsDir())
}

func Test_CreateReadDeleteFile(t *testing.T) {
	fileName := "temp.txt"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	err := fileStore.Create(fileName, []byte("some content"))
	defer fileStore.Delete(fileName)

	assert.Nil(t, err)

	data, err := fileStore.Read("temp.txt")

	assert.Nil(t, err)
	assert.Equal(t, "some content", string(data))
}

func Test_CreateMoveDeleteFile(t *testing.T) {
	fileName := "temp.txt"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	err := fileStore.Create(fileName, []byte("some content"))
	defer fileStore.Delete("temp.text")

	assert.Nil(t, err)

	err = fileStore.Move("temp.txt", "temp.text")

	assert.Nil(t, err)
}

func Test_CreateUpdateReadFile(t *testing.T) {
	fileName := "temp.txt"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	err := fileStore.Create(fileName, []byte("some content"))
	defer fileStore.Delete(fileName)

	assert.Nil(t, err)

	err = fileStore.Update(fileName, []byte("some new content"))

	data, err := fileStore.Read("temp.txt")

	assert.Nil(t, err)
	assert.Equal(t, "some new content", string(data))
}
