package s3

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	file_interface "gofr.dev/pkg/gofr/datasource/file"
	"io"
	"testing"
)

func Test_WriteRead(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		var csvContent = `Name,Age,Email
John Doe,30,johndoe@example.com
Jane Smith,25,janesmith@example.com
Emily Johnson,35,emilyj@example.com
Michael Brown,40,michaelb@example.com`

		ctrl := gomock.NewController(t)

		mockLogger := NewMockLogger(ctrl)
		mockMetrics := NewMockMetrics(ctrl)

		fs.UseLogger(mockLogger)
		fs.UseMetrics(mockMetrics)

		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

		err := fs.ChDir("gofr-bucket-2")
		require.NoError(t, err)

		newCsvFile, err := fs.Create("temp.csv")
		require.NoError(t, err)

		defer func(fs file_interface.FileSystem, name string) {
			err = fs.Remove(name)
			require.NoError(t, err)

		}(fs, "temp.csv")

		_, err = newCsvFile.Write([]byte(csvContent))
		require.NoError(t, err, "failed to write to csv file")

		buffer := make([]byte, 5)
		_, err = newCsvFile.Seek(0, io.SeekStart)
		require.NoError(t, err)

		_, err = newCsvFile.Read(buffer)
		require.NoError(t, err, "Read from file failed")
		assert.Equal(t, csvContent[:5], string(buffer), "Read from file")

		_, err = newCsvFile.Seek(0, io.SeekStart)
		require.NoError(t, err)
		_, err = newCsvFile.WriteAt([]byte("Hello World"), 5)
		require.NoError(t, err, "Write to file at offset failed")

		_, err = newCsvFile.Seek(0, io.SeekStart)
		require.NoError(t, err)
		_, err = newCsvFile.ReadAt(buffer, 5)
		require.NoError(t, err, "Read from file at offset failed")
		assert.Equal(t, "ello ", string(buffer), "Read from file at offset")
	})

}

func Test_Seek(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		var jsonContent = `0123456789`

		ctrl := gomock.NewController(t)
		mockLogger := NewMockLogger(ctrl)
		mockMetrics := NewMockMetrics(ctrl)

		fs.UseLogger(mockLogger)
		fs.UseMetrics(mockMetrics)

		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

		err := fs.ChDir("gofr-bucket-2")
		require.NoError(t, err)

		newCsvFile, err := fs.Create("temp.json")
		require.NoError(t, err)
		defer func(fs file_interface.FileSystem, name string) {
			removeErr := fs.Remove(name)
			if removeErr != nil {
				t.Error(removeErr)
			}
		}(fs, "temp.json")

		_, err = newCsvFile.Write([]byte(jsonContent))
		require.NoError(t, err)

		newOffset, err := newCsvFile.Seek(3, io.SeekStart)
		require.NoError(t, err)
		assert.Equal(t, int64(3), newOffset)

		buffer := make([]byte, 7)

		_, err = newCsvFile.Read(buffer)
		require.Error(t, err)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, jsonContent[3:], string(buffer))

		newOffset, err = newCsvFile.Seek(-5, io.SeekEnd)
		require.NoError(t, err)
		assert.Equal(t, int64(5), newOffset)

		_, err = newCsvFile.Write(buffer)
		require.NoError(t, err)

		newOffset, err = newCsvFile.Seek(-9, io.SeekEnd)

		_, err = newCsvFile.Read(buffer)
		require.NoError(t, err)
		assert.Equal(t, "3434567", string(buffer))

	})
}

// The test defined below do not use any mocking. They need an actual ftp server connection.
func Test_ReadFromCSV(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
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

		ctrl := gomock.NewController(t)

		mockLogger := NewMockLogger(ctrl)
		mockMetrics := NewMockMetrics(ctrl)

		fs.UseLogger(mockLogger)
		fs.UseMetrics(mockMetrics)

		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

		err := fs.ChDir("gofr-bucket-2")
		require.NoError(t, err)

		newCsvFile, err := fs.Create("temp.csv")
		require.NoError(t, err)

		_, err = newCsvFile.Write([]byte(csvContent))
		require.NoError(t, err)

		newCsvFile, err = fs.Open("temp.csv")
		require.NoError(t, err)

		defer func(fs file_interface.FileSystem, name string) {
			err = fs.Remove(name)
			require.NoError(t, err)

		}(fs, "temp.csv")

		var i = 0

		reader, _ := newCsvFile.ReadAll()
		for reader.Next() {
			var content string

			err := reader.Scan(&content)

			assert.Equal(t, csvValue[i], content)
			assert.NoError(t, err)

			i++
		}
	})
}

func Test_ReadFromCSVScanError(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		var csvContent = `Name,Age,Email`

		ctrl := gomock.NewController(t)
		mockLogger := NewMockLogger(ctrl)
		mockMetrics := NewMockMetrics(ctrl)

		fs.UseLogger(mockLogger)
		fs.UseMetrics(mockMetrics)

		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

		err := fs.ChDir("gofr-bucket-2")
		require.NoError(t, err)

		newCsvFile, err := fs.Create("temp.csv")
		require.NoError(t, err)

		_, err = newCsvFile.Write([]byte(csvContent))
		require.NoError(t, err)

		newCsvFile, err = fs.Open("temp.csv")
		require.NoError(t, err)

		reader, err := newCsvFile.ReadAll()
		require.NoError(t, err)

		defer func(fs file_interface.FileSystem, name string) {
			err := fs.Remove(name)
			require.NoError(t, err)

		}(fs, "temp.csv")

		for reader.Next() {
			var content string

			err := reader.Scan(content)

			assert.Error(t, err)
			assert.Equal(t, "", content)
		}
	})
}

func Test_ReadFromJSONArray(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		var jsonContent = `[{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123}]`

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var jsonValue = []User{{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
		}

		ctrl := gomock.NewController(t)
		mockLogger := NewMockLogger(ctrl)
		mockMetrics := NewMockMetrics(ctrl)

		fs.UseLogger(mockLogger)
		fs.UseMetrics(mockMetrics)

		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

		err := fs.ChDir("gofr-bucket-2")
		require.NoError(t, err)

		newCsvFile, err := fs.Create("temp.json")
		require.NoError(t, err)

		_, err = newCsvFile.Write([]byte(jsonContent))
		require.NoError(t, err)

		newCsvFile, err = fs.Open("temp.json")
		require.NoError(t, err)

		defer func(fs file_interface.FileSystem, name string) {
			err := fs.Remove(name)
			require.NoError(t, err)
		}(fs, "temp.json")

		var i = 0

		reader, readerError := newCsvFile.ReadAll()
		if readerError == nil {
			for reader.Next() {
				var u User

				err := reader.Scan(&u)

				assert.Equal(t, jsonValue[i].Name, u.Name)
				assert.Equal(t, jsonValue[i].Age, u.Age)
				assert.NoError(t, err)

				i++
			}
		}
	})
}

func Test_ReadFromJSONObject(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		var jsonContent = `{"name": "Sam", "age": 123}`

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		ctrl := gomock.NewController(t)
		mockLogger := NewMockLogger(ctrl)
		mockMetrics := NewMockMetrics(ctrl)

		fs.UseLogger(mockLogger)
		fs.UseMetrics(mockMetrics)

		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

		err := fs.ChDir("gofr-bucket-2")
		require.NoError(t, err)

		newCsvFile, _ := fs.Create("temp.json")

		_, err = newCsvFile.Write([]byte(jsonContent))
		require.NoError(t, err)

		newCsvFile, err = fs.Open("temp.json")
		require.NoError(t, err)

		reader, _ := newCsvFile.ReadAll()
		require.NoError(t, err)

		defer func(fs file_interface.FileSystem, name string) {
			err := fs.Remove(name)
			if err != nil {
				t.Error(err)
			}
		}(fs, "temp.json")

		for reader.Next() {
			var u User

			err := reader.Scan(&u)

			assert.Equal(t, "Sam", u.Name)
			assert.Equal(t, 123, u.Age)

			assert.NoError(t, err)
		}
	})
}

func Test_ReadFromJSONArrayInvalidDelimiter(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		var jsonContent = `!@#$%^&*`

		ctrl := gomock.NewController(t)
		mockLogger := NewMockLogger(ctrl)
		mockMetrics := NewMockMetrics(ctrl)

		fs.UseLogger(mockLogger)
		fs.UseMetrics(mockMetrics)

		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

		err := fs.ChDir("gofr-bucket-2")
		require.NoError(t, err)

		newCsvFile, err := fs.Create("temp.json")
		require.NoError(t, err)

		_, err = newCsvFile.Write([]byte(jsonContent))
		require.NoError(t, err)

		newCsvFile.Close()

		newCsvFile, _ = fs.Open("temp.json")

		_, err = newCsvFile.ReadAll()

		defer func(fs file_interface.FileSystem, name string) {
			removeErr := fs.Remove(name)
			if removeErr != nil {
				t.Error(removeErr)
			}
		}(fs, "temp.json")

		assert.IsType(t, &json.SyntaxError{}, err)
	})
}
