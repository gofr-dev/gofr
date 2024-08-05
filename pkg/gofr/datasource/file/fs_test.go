package file

import (
	"encoding/json"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
)

func Test_LocalFileSystemDirectoryCreation(t *testing.T) {
	dirName := "temp!@#$%^&*(123"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	err := fileStore.Mkdir(dirName, os.ModePerm)
	defer os.RemoveAll(dirName)

	require.NoError(t, err)

	fInfo, err := os.Stat(dirName)

	require.NoError(t, err)
	assert.True(t, fInfo.IsDir())
}

func Test_LocalFileOpenError(t *testing.T) {
	dirName := "temp!@#$%^&*(123"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	_, err := fileStore.Open(dirName)

	assert.IsType(t, &fs.PathError{}, err)
}

func Test_LocalFileOpenFileError(t *testing.T) {
	dirName := "temp!@#$%^&*(123"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	_, err := fileStore.OpenFile(dirName, 0, os.ModePerm)

	assert.IsType(t, &fs.PathError{}, err)
}

func Test_CreateReadDeleteFile(t *testing.T) {
	fileName := "temp.txt"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newFile, err := fileStore.Create(fileName)

	_, _ = newFile.Write([]byte("some content"))

	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.Remove(name)
	}(fileStore, fileName)

	require.NoError(t, err)

	tempFile, _ := fileStore.Open("temp.txt")

	reader := make([]byte, 30)

	_, err = tempFile.Read(reader)

	require.NoError(t, err)
	assert.Contains(t, string(reader), "some content")
}

func Test_CreateMoveDeleteFile(t *testing.T) {
	fileName := "temp.txt"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	_, err := fileStore.Create(fileName)

	require.NoError(t, err)

	err = fileStore.Rename("temp.txt", "temp.text")
	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.Remove(name)
	}(fileStore, "temp.text")

	require.NoError(t, err)
}

func Test_CreateUpdateReadFile(t *testing.T) {
	fileName := "temp.txt"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newFile, err := fileStore.Create(fileName)

	_, _ = newFile.Write([]byte("some content"))

	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.Remove(name)
	}(fileStore, fileName)

	require.NoError(t, err)

	openedFile, _ := fileStore.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	_, _ = openedFile.WriteAt([]byte("some new content"), 0)
	openedFile.Close()

	openedFile, _ = fileStore.Open(fileName)
	reader := make([]byte, 30)
	_, err = openedFile.Read(reader)
	openedFile.Close()

	require.NoError(t, err)
	assert.Contains(t, string(reader), "some new content")
}

func Test_CreateFileInvalidPath(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	_, err := fileStore.Create("")

	assert.IsType(t, &fs.PathError{}, err)
}

func Test_CreateAndDeleteMultipleDirectories(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	_ = fileStore.MkdirAll("temp/text/", os.ModePerm)

	err := fileStore.RemoveAll("temp")

	require.NoError(t, err)
}

func Test_ReadFromCSV(t *testing.T) {
	var csvContent = `Name,Age,Email
John Doe,30,johndoe@example.com
Jane Smith,25,janesmith@example.com
Emily Johnson,35,emilyj@example.com
Michael Brown,40,michaelb@example.com`

	var csvValue = []string{"Name,Age,Email",
		"John Doe,30,johndoe@example.com",
		"Jane Smith,25,janesmith@example.com",
		"Emily Johnson,35,emilyj@example.com",
		"Michael Brown,40,michaelb@example.com",
	}

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.csv")
	_, _ = newCsvFile.Write([]byte(csvContent))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.csv")
	reader, _ := newCsvFile.ReadAll()

	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.RemoveAll(name)
	}(fileStore, "temp.csv")

	var i = 0

	for reader.Next() {
		var content string

		err := reader.Scan(&content)

		assert.Equal(t, csvValue[i], content)

		require.NoError(t, err)

		i++
	}
}

func Test_ReadFromCSVScanError(t *testing.T) {
	var csvContent = `Name,Age,Email`

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.csv")
	_, _ = newCsvFile.Write([]byte(csvContent))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.csv")
	reader, _ := newCsvFile.ReadAll()

	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.RemoveAll(name)
	}(fileStore, "temp.csv")

	for reader.Next() {
		var content string

		err := reader.Scan(content)

		require.Error(t, err)
		assert.Equal(t, "", content)
	}
}

func Test_ReadFromJSONArray(t *testing.T) {
	var jsonContent = `[{"name": "Sam", "age": 123},{"name": "Jane", "age": 456},{"name": "John", "age": 789}]`

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var jsonValue = []User{{"Sam", 123}, {"Jane", 456}, {"John", 789}}

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.json")
	_, _ = newCsvFile.Write([]byte(jsonContent))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.json")
	reader, _ := newCsvFile.ReadAll()

	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.RemoveAll(name)
	}(fileStore, "temp.json")

	var i = 0

	for reader.Next() {
		var u User

		err := reader.Scan(&u)

		assert.Equal(t, jsonValue[i].Name, u.Name)
		assert.Equal(t, jsonValue[i].Age, u.Age)

		require.NoError(t, err)

		i++
	}
}

func Test_ReadFromJSONObject(t *testing.T) {
	var jsonContent = `{"name": "Sam", "age": 123}`

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.json")
	_, _ = newCsvFile.Write([]byte(jsonContent))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.json")
	reader, _ := newCsvFile.ReadAll()

	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.RemoveAll(name)
	}(fileStore, "temp.json")

	for reader.Next() {
		var u User

		err := reader.Scan(&u)

		assert.Equal(t, "Sam", u.Name)
		assert.Equal(t, 123, u.Age)

		require.NoError(t, err)
	}
}

func Test_ReadFromJSONArrayInvalidDelimiter(t *testing.T) {
	var jsonContent = `!@#$%^&*`

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.json")
	_, _ = newCsvFile.Write([]byte(jsonContent))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.json")

	_, err := newCsvFile.ReadAll()

	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.RemoveAll(name)
	}(fileStore, "temp.json")

	assert.IsType(t, &json.SyntaxError{}, err)
}
