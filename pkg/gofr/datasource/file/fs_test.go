package file

import (
	"encoding/json"
	"gofr.dev/pkg/gofr/datasource"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/logging"
)

func Test_LocalFileSystemDirectoryCreation(t *testing.T) {
	dirName := "temp!@#$%^&*(123"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	err := fileStore.Mkdir(dirName, os.ModePerm)
	defer os.RemoveAll(dirName)

	assert.Nil(t, err)

	fInfo, err := os.Stat(dirName)

	assert.Nil(t, err)
	assert.Equal(t, true, fInfo.IsDir())
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
	newFile.Write([]byte("some content"))

	defer func(fileStore datasource.FileSystem, name string, options ...interface{}) {
		_ = fileStore.Remove(name)
	}(fileStore, fileName)

	assert.Nil(t, err)

	tempFile, _ := fileStore.Open("temp.txt")

	reader := make([]byte, 30)

	_, err = tempFile.Read(reader)

	assert.Nil(t, err)
	assert.Contains(t, string(reader), "some content")
}

func Test_CreateMoveDeleteFile(t *testing.T) {
	fileName := "temp.txt"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	_, err := fileStore.Create(fileName)

	assert.Nil(t, err)

	err = fileStore.Rename("temp.txt", "temp.text")
	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.Remove(name)
	}(fileStore, "temp.text")

	assert.Nil(t, err)
}

func Test_CreateUpdateReadFile(t *testing.T) {
	fileName := "temp.txt"

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newFile, err := fileStore.Create(fileName)
	newFile.Write([]byte("some content"))

	defer func(fileStore datasource.FileSystem, name string) {
		_ = fileStore.Remove(name)
	}(fileStore, fileName)

	assert.Nil(t, err)

	openedFile, err := fileStore.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	openedFile.WriteAt([]byte("some new content"), 0)
	openedFile.Close()

	openedFile, err = fileStore.Open(fileName)
	reader := make([]byte, 30)
	_, err = openedFile.Read(reader)
	openedFile.Close()

	assert.Nil(t, err)
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

	err := fileStore.MkdirAll("temp/text/", os.ModePerm)

	err = fileStore.RemoveAll("temp")

	assert.Nil(t, err)
}

func Test_ReadFromCSV(t *testing.T) {
	var csv_content = `Name,Age,Email
John Doe,30,johndoe@example.com
Jane Smith,25,janesmith@example.com
Emily Johnson,35,emilyj@example.com
Michael Brown,40,michaelb@example.com`

	var csv_value = []string{"Name,Age,Email",
		"John Doe,30,johndoe@example.com",
		"Jane Smith,25,janesmith@example.com",
		"Emily Johnson,35,emilyj@example.com",
		"Michael Brown,40,michaelb@example.com",
	}

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.csv")
	newCsvFile.Write([]byte(csv_content))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.csv")
	reader, _ := newCsvFile.ReadAll()

	defer fileStore.RemoveAll("temp.csv")

	var i = 0

	for reader.Next() {
		var content string

		reader.Scan(&content)

		assert.Equal(t, csv_value[i], content)
		i++
	}
}

func Test_ReadFromCSVScanError(t *testing.T) {
	var csv_content = `Name,Age,Email`

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.csv")
	newCsvFile.Write([]byte(csv_content))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.csv")
	reader, _ := newCsvFile.ReadAll()

	defer fileStore.RemoveAll("temp.csv")

	for reader.Next() {
		var content string

		err := reader.Scan(content)

		assert.NotNil(t, err)
		assert.Equal(t, "", content)
	}
}

func Test_ReadFromJSONArray(t *testing.T) {
	var json_content = `[{"name": "Sam", "age": 123},{"name": "Jane", "age": 456},{"name": "John", "age": 789}]`

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var json_value = []User{{"Sam", 123}, {"Jane", 456}, {"John", 789}}

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.json")
	newCsvFile.Write([]byte(json_content))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.json")
	reader, _ := newCsvFile.ReadAll()

	defer fileStore.RemoveAll("temp.json")

	var i = 0

	for reader.Next() {
		var u User

		reader.Scan(&u)

		assert.Equal(t, json_value[i].Name, u.Name)
		assert.Equal(t, json_value[i].Age, u.Age)

		i++
	}
}

func Test_ReadFromJSONObject(t *testing.T) {
	var json_content = `{"name": "Sam", "age": 123}`

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.json")
	newCsvFile.Write([]byte(json_content))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.json")
	reader, _ := newCsvFile.ReadAll()

	defer fileStore.RemoveAll("temp.json")

	for reader.Next() {
		var u User

		reader.Scan(&u)

		assert.Equal(t, "Sam", u.Name)
		assert.Equal(t, 123, u.Age)
	}
}

func Test_ReadFromJSONArrayInvalidDelimitter(t *testing.T) {
	var json_content = `!@#$%^&*`

	logger := logging.NewMockLogger(logging.DEBUG)

	fileStore := New(logger)

	newCsvFile, _ := fileStore.Create("temp.json")
	newCsvFile.Write([]byte(json_content))
	newCsvFile.Close()

	newCsvFile, _ = fileStore.Open("temp.json")

	_, err := newCsvFile.ReadAll()
	defer fileStore.RemoveAll("temp.json")

	assert.IsType(t, &json.SyntaxError{}, err)

}
