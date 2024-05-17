package file

import (
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"io/fs"
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

func Test_NewFileStoreWithoutLogger(t *testing.T) {
	fs := New(sql.NewMockMetrics(gomock.NewController(t)))

	assert.NotNil(t, fs)
}

func Test_CreateFileInvalidPath(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	err := fileStore.Create("", []byte("some content"))

	assert.IsType(t, &fs.PathError{}, err)
}

func Test_CreateFileDuplicateFile(t *testing.T) {
	fileName := "test"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	_ = fileStore.Create("test", []byte("some content"))
	defer fileStore.Delete(fileName)

	err := fileStore.Create("test", []byte("some content"))

	assert.IsType(t, &fs.PathError{}, err)
}

func Test_ReadFileNotFoundError(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	_, err := fileStore.Read("test")

	assert.IsType(t, &fs.PathError{}, err)
}

func Test_UpdateFileNotFoundError(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	err := fileStore.Update("test", []byte(""))

	assert.IsType(t, &fs.PathError{}, err)
}
