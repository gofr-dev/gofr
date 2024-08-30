package s3

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	file "gofr.dev/pkg/gofr/datasource/file"
)

func Test_WriteRead(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		var csvContent = `Name,Age,Email
John Doe,30,johndoe@example.com
Jane Smith,25,janesmith@example.com
Emily Johnson,35,emilyj@example.com
Michael Brown,40,michaelb@example.com`

		newCsvFile, err := fs.Create("temp.csv")
		require.NoError(t, err, "TEST WriteRead Failed. Desc: %v", "Failed to create CSV file")

		defer func(fs file.FileSystem, name string) {
			err = fs.Remove(name)
			require.NoError(t, err, "TEST WriteRead Failed. Desc: %v", "Error removing file %v", name)
		}(fs, "temp.csv")

		// Test Write
		_, err = newCsvFile.Write([]byte(csvContent))
		require.NoError(t, err, "TEST WriteRead Failed. Desc: %v", "Failed to write to CSV file")

		buffer := make([]byte, 5)

		_, err = newCsvFile.Seek(0, io.SeekStart)
		require.NoError(t, err, "TEST WriteRead Failed. Desc: %v", "Error seeking to start of file")

		// Test Read
		_, err = newCsvFile.Read(buffer)
		require.NoError(t, err, "TEST WriteRead Failed. Desc: Error reading from file")
		assert.Equal(t, csvContent[:5], string(buffer), "TEST WriteRead Failed. Desc: %v", "Read content mismatch")

		_, err = newCsvFile.Seek(0, io.SeekStart)
		require.NoError(t, err, "TEST WriteRead Failed. Desc: %v", "Error seeking to start of file")

		// Test WriteAt
		_, err = newCsvFile.WriteAt([]byte("Hello World"), 5)
		require.NoError(t, err, "TEST WriteRead Failed. Desc: %v", "Error writing to file at offset")

		_, err = newCsvFile.Seek(0, io.SeekStart)
		require.NoError(t, err, "TEST WriteRead Failed. Desc: %v", "Error seeking to start of file")

		// Test ReadAt
		_, err = newCsvFile.ReadAt(buffer, 5)
		require.NoError(t, err, "TEST WriteRead Failed. Desc: %v", "Error reading from file at offset")
		assert.Equal(t, "ello ", string(buffer), "TEST WriteRead Failed. Desc: %v", "Read content at offset mismatch")
	})
}

func Test_Seek(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		var jsonContent = `0123456789`

		newCsvFile, err := fs.Create("temp.json")
		require.NoError(t, err, "TEST Seek Failed. Desc: %v", "Failed to create JSON file")

		defer func(fs file.FileSystem, name string) {
			removeErr := fs.Remove(name)
			require.NoError(t, removeErr, "TEST Seek Failed. Desc: %v", "Error removing file %v", name)
		}(fs, "temp.json")

		_, err = newCsvFile.Write([]byte(jsonContent))
		require.NoError(t, err, "TEST Seek Failed. Desc: %v", "Error writing to JSON file")

		// Test fetching offset from start
		newOffset, err := newCsvFile.Seek(3, io.SeekStart)
		require.NoError(t, err, "TEST Seek Failed. Desc: %v", "Error seeking to offset 3")
		assert.Equal(t, int64(3), newOffset, "TEST Seek Failed. Desc: %v", "Seek offset mismatch")

		buffer := make([]byte, 7)

		_, err = newCsvFile.Read(buffer)
		require.Error(t, err, "TEST Seek Failed. Desc: %v", "Expected EOF error")
		assert.Equal(t, io.EOF, err, "TEST Seek Failed. Desc: %v", "Unexpected error type")
		assert.Equal(t, jsonContent[3:], string(buffer), "TEST Seek Failed. Desc: %v", "Read content mismatch")

		// Test fetching offset from end
		newOffset, err = newCsvFile.Seek(-5, io.SeekEnd)
		require.NoError(t, err, "TEST Seek Failed. Desc: %v", "Error seeking to offset -5 from end")
		assert.Equal(t, int64(5), newOffset, "TEST Seek Failed. Desc: %v", "Seek offset mismatch")

		_, err = newCsvFile.Write(buffer)
		require.NoError(t, err, "TEST Seek Failed. Desc: %v", "Error writing to file at offset")

		_, err = newCsvFile.Seek(-9, io.SeekEnd)
		require.NoError(t, err, "TEST Seek Failed. Desc: %v", "Error seeking to offset -9 from end")

		_, err = newCsvFile.Read(buffer)
		require.NoError(t, err, "TEST Seek Failed. Desc: %v", "Error reading from file at offset")
		assert.Equal(t, "3434567", string(buffer), "TEST Seek Failed. Desc: %v", "Read content mismatch")
	})
}

func Test_ReadFromCSV(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		var csvContent = `Name,Age,Email
John Doe,30,johndoe@example.com
Jane Smith,25,janesmith@example.com
Emily Johnson,35,emilyj@example.com
Michael Brown,40,michaelb@example.com`

		csvValue := []string{
			"Name,Age,Email",
			"John Doe,30,johndoe@example.com",
			"Jane Smith,25,janesmith@example.com",
			"Emily Johnson,35,emilyj@example.com",
			"Michael Brown,40,michaelb@example.com",
		}

		newCsvFile, err := fs.Create("temp.csv")
		require.NoError(t, err, "TEST ReadFromCSV Failed. Desc: %v", "Failed to create CSV file")

		defer func(fs file.FileSystem, name string) {
			err = fs.Remove(name)
			require.NoError(t, err, "Error removing file %v", name)
		}(fs, "temp.csv")

		_, err = newCsvFile.Write([]byte(csvContent))
		require.NoError(t, err, "TEST ReadFromCSV Failed. Desc: %v", "Failed to write to CSV file")

		newCsvFile, err = fs.Open("temp.csv")
		require.NoError(t, err, "TEST ReadFromCSV Failed. Desc: %v", "Failed to open CSV file")

		var i = 0

		reader, _ := newCsvFile.ReadAll()
		for reader.Next() {
			var content string

			err := reader.Scan(&content)

			assert.Equal(t, csvValue[i], content, "TEST ReadFromCSV Failed. Desc: %v", "CSV content mismatch")
			assert.NoError(t, err, "TEST ReadFromCSV Failed. Desc: %v", "Error scanning CSV content")

			i++
		}
	})
}

func Test_ReadFromCSVScanError(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		var csvContent = `Name,Age,Email`

		newCsvFile, err := fs.Create("temp.csv")
		require.NoError(t, err, "TEST ReadFromCSVScanError Failed. Desc: %v", "Failed to create CSV file")

		defer func(fs file.FileSystem, name string) {
			err = fs.Remove(name)
			require.NoError(t, err, "TEST ReadFromCSVScanError Failed. Desc: %v", "Error removing file %v", name)
		}(fs, "temp.csv")

		_, err = newCsvFile.Write([]byte(csvContent))
		require.NoError(t, err, "TEST ReadFromCSVScanError Failed. Desc: %v", "Failed to write to CSV file")

		newCsvFile, err = fs.Open("temp.csv")
		require.NoError(t, err, "TEST ReadFromCSVScanError Failed. Desc: %v", "Failed to open CSV file")

		reader, err := newCsvFile.ReadAll()
		require.NoError(t, err, "TEST ReadFromCSVScanError Failed. Desc: %v", "Failed to create reader")

		for reader.Next() {
			var content string

			err = reader.Scan(content)

			require.Error(t, err, "TEST ReadFromCSVScanError Failed. Desc: %v", "expected error during scan")
			assert.Equal(t, "", content, "TEST ReadFromCSVScanError Failed. Desc: %v", "Content should be empty on scan error")
		}
	})
}

func Test_ReadFromJSONArray(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		var jsonContent = `[{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123}]`

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var jsonValue = []User{
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
		}

		newCsvFile, err := fs.Create("temp.json")
		require.NoError(t, err, "TEST ReadFromJSONArray Failed. Desc: %v", "Failed to create JSON file")

		defer func(fs file.FileSystem, name string) {
			err = fs.Remove(name)
			require.NoError(t, err, "TEST ReadFromJSONArray Failed. Desc: %v", "Error removing file %v", name)
		}(fs, "temp.json")

		_, err = newCsvFile.Write([]byte(jsonContent))
		require.NoError(t, err, "TEST ReadFromJSONArray Failed. Desc: %v", "Failed to write to JSON file")

		newCsvFile, err = fs.Open("temp.json")
		require.NoError(t, err, "TEST ReadFromJSONArray Failed. Desc: %v", "Failed to open JSON file")

		var i = 0

		reader, readerError := newCsvFile.ReadAll()
		require.NoError(t, readerError, "TEST ReadFromJSONArray Failed. Desc: %v", "Error creating Reader")

		if readerError == nil {
			for reader.Next() {
				var u User

				err = reader.Scan(&u)

				require.NoError(t, err, "TEST ReadFromJSONArray Failed. Desc: %v", "Error scanning json content")
				assert.Equal(t, jsonValue[i].Name, u.Name, "TEST ReadFromJSONArray Failed. Desc: %v", "Expected json field Name to be equal")
				assert.Equal(t, jsonValue[i].Age, u.Age, "TEST ReadFromJSONArray Failed. Desc: %v", "Expected json field Age to be equal")

				i++
			}
		}
	})
}

func Test_ReadFromJSONObject(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		var jsonContent = `{"name": "Sam", "age": 123}`

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		newCsvFile, err := fs.Create("temp.json")
		require.NoError(t, err, "TEST ReadFromJSONObject Failed. Desc: %v", "Failed to create JSON file")

		_, err = newCsvFile.Write([]byte(jsonContent))
		require.NoError(t, err, "TEST ReadFromJSONObject Failed. Desc: %v", "Failed to write to JSON file")

		newCsvFile, err = fs.Open("temp.json")
		require.NoError(t, err, "TEST ReadFromJSONObject Failed. Desc: %v", "Failed to open JSON file")

		reader, err := newCsvFile.ReadAll()
		require.NoError(t, err, "TEST ReadFromJSONObject Failed. Desc: %v", "Failed to create reader")

		defer func(fs file.FileSystem, name string) {
			err = fs.Remove(name)
			require.NoError(t, err, "TEST ReadFromJSONObject Failed. Desc: %v", "Error removing file %v", name)
		}(fs, "temp.json")

		for reader.Next() {
			var u User

			err = reader.Scan(&u)

			require.NoError(t, err, "TEST ReadFromJSONObject Failed. Desc: %v", "Error reading from JSON object")
			assert.Equal(t, "Sam", u.Name, "TEST ReadFromJSONObject Failed. Desc: %v", "Expected json field Name to be equal")
			assert.Equal(t, 123, u.Age, "TEST ReadFromJSONObject Failed. Desc: %v", "Expected json field Age to be equal")
		}
	})
}

func Test_ReadFromJSONArrayInvalidDelimiter(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		var jsonContent = `!@#$%^&*`

		newCsvFile, err := fs.Create("temp.json")
		require.NoError(t, err, "TEST ReadFromJSONArrayInvalidDelimiter Failed. Desc: %v", "Failed to create JSON file")

		defer func(fs file.FileSystem, name string) {
			removeErr := fs.Remove(name)
			require.NoError(t, removeErr, "Error removing file %v", name)
		}(fs, "temp.json")

		_, err = newCsvFile.Write([]byte(jsonContent))
		require.NoError(t, err, "TEST ReadFromJSONArrayInvalidDelimiter Failed. Desc: %v", "Failed to write to JSON file")

		err = newCsvFile.Close()
		require.NoError(t, err, "TEST ReadFromJSONArrayInvalidDelimiter Failed. Desc: %v", "Error closing JSON file after write")

		newCsvFile, err = fs.Open("temp.json")
		require.NoError(t, err, "TEST ReadFromJSONArrayInvalidDelimiter Failed. Desc: %v", "Error opening JSON file")

		_, err = newCsvFile.ReadAll()
		require.Error(t, err, "TEST ReadFromJSONArrayInvalidDelimiter Failed. Desc: %v", "Expected Error reading from invalid JSON")

		assert.IsType(t, &json.SyntaxError{}, err)
	})
}
