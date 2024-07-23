package ftp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"gofr.dev/pkg/gofr/logging"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
)

func TestRead(t *testing.T) {
	// Define test cases for Read method
	var readTests = []struct {
		name             string
		filePath         string
		mockReadResponse func(response *MockftpResponse)
		expectError      bool
	}{
		{
			name:     "Successful read",
			filePath: "/ftp/one/testfile2.txt",
			mockReadResponse: func(response *MockftpResponse) {
				response.EXPECT().Read(gomock.Any()).Return(10, io.EOF).AnyTimes()
			},
			expectError: false,
		},
		{
			name:     "Read with error",
			filePath: "/ftp/one/nonexistent.txt",
			mockReadResponse: func(response *MockftpResponse) {
				response.EXPECT().Read(gomock.Any()).Return(0, errors.New("mocked read error"))
			},
			expectError: true,
		},
	}

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for Read method
	for _, tt := range readTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Create mock response
			response := NewMockftpResponse(ctrl)
			tt.mockReadResponse(response)

			// Set expectation for Retr method
			mockFtpConn.EXPECT().Retr(tt.filePath).Return(response, nil)

			// Initialize buffer for reading
			s := make([]byte, 1024)

			// Create ftpFile instance with mock connection
			file := ftpFile{path: tt.filePath, conn: fs.conn}

			// Perform Read operation
			_, err := file.Read(s)

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful read
			if err == nil {
				t.Logf("Read successfully from %s", tt.filePath)
			}
		})
	}
}

func TestReadAt(t *testing.T) {
	// Define test cases for ReadAt method
	var readAtTests = []struct {
		name             string
		filePath         string
		offset           int64
		mockReadResponse func(response *MockftpResponse)
		expectError      bool
	}{
		{
			name:     "Successful read with offset",
			filePath: "/ftp/one/testfile2.txt",
			offset:   3,
			mockReadResponse: func(response *MockftpResponse) {
				response.EXPECT().Read(gomock.Any()).Return(10, io.EOF).AnyTimes()
			},
			expectError: false,
		},
		{
			name:     "ReadAt with error",
			filePath: "/ftp/one/nonexistent.txt",
			offset:   0,
			mockReadResponse: func(response *MockftpResponse) {
				response.EXPECT().Read(gomock.Any()).Return(0, errors.New("mocked read error"))
			},
			expectError: true,
		},
	}

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for ReadAt method
	for _, tt := range readAtTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Create mock response
			response := NewMockftpResponse(ctrl)
			tt.mockReadResponse(response)

			// Set expectation for RetrFrom method
			mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(tt.offset)).Return(response, nil)

			// Initialize buffer for reading
			s := make([]byte, 1024)

			// Create ftpFile instance with mock connection
			file := ftpFile{path: tt.filePath, conn: fs.conn}

			// Perform ReadAt operation
			_, err := file.ReadAt(s, tt.offset)

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful read
			if err == nil {
				t.Logf("Read successfully from %s at offset %d", tt.filePath, tt.offset)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	// Define test cases for Write method
	var writeTests = []struct {
		name            string
		filePath        string
		mockWriteExpect func(conn *MockServerConn, filePath string)
		expectError     bool
	}{
		{
			name:     "Successful write",
			filePath: "/ftp/one/testfile.txt",
			mockWriteExpect: func(conn *MockServerConn, filePath string) {
				emptyReader := bytes.NewBuffer([]byte("test content"))
				conn.EXPECT().Delete(filePath).Return(nil)
				conn.EXPECT().Stor(filePath, emptyReader).Return(nil)
			},
			expectError: false,
		},
		{
			name:     "Write with error",
			filePath: "/ftp/one/nonexistent.txt",
			mockWriteExpect: func(conn *MockServerConn, filePath string) {
				conn.EXPECT().Delete(filePath).Return(errors.New("file not deleted"))
			},
			expectError: true,
		},
	}

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for Write method
	for _, tt := range writeTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Set mock expectations for Stor method
			tt.mockWriteExpect(mockFtpConn, tt.filePath)

			// Create ftpFile instance with mock connection
			file := ftpFile{path: tt.filePath, conn: fs.conn}

			// Perform Write operation
			_, err := file.Write([]byte("test content"))

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful write
			if err == nil {
				t.Logf("Wrote successfully to %s", tt.filePath)
			}
		})
	}
}

func TestWriteAt(t *testing.T) {
	// Define test cases for WriteAt method
	var writeAtTests = []struct {
		name            string
		filePath        string
		offset          int64
		mockWriteExpect func(conn *MockServerConn, filePath string, offset int64)
		expectError     bool
	}{
		{
			name:     "Successful write at offset",
			filePath: "/ftp/one/testfile.txt",
			offset:   3,
			mockWriteExpect: func(conn *MockServerConn, filePath string, offset int64) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(offset)).Return(nil)
			},
			expectError: false,
		},
		{
			name:     "WriteAt with error",
			filePath: "/ftp/one/nonexistent.txt",
			offset:   0,
			mockWriteExpect: func(conn *MockServerConn, filePath string, offset int64) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(offset)).Return(errors.New("mocked write error"))
			},
			expectError: true,
		},
	}

	// Initialize gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Iterate over test cases for WriteAt method
	for _, tt := range writeAtTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock FTP server connection
			mockFtpConn := NewMockServerConn(ctrl)

			// Create ftpFileSystem instance with mock dependencies
			fs := &ftpFileSystem{
				conn: mockFtpConn,
				config: &Config{
					Host:      "ftp.example.com",
					User:      "username",
					Password:  "password",
					Port:      "21",
					RemoteDir: "/ftp/one",
				},
			}

			// Set mock expectations for StorAt method
			tt.mockWriteExpect(mockFtpConn, tt.filePath, tt.offset)

			// Create ftpFile instance with mock connection
			file := ftpFile{path: tt.filePath, conn: fs.conn}

			// Perform WriteAt operation
			_, err := file.WriteAt([]byte("test content"), tt.offset)

			// Check for errors
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}

			// Log successful write
			if err == nil {
				t.Logf("Wrote successfully to %s at offset %d", tt.filePath, tt.offset)
			}
		})
	}
}

func TestSeekFile(t *testing.T) {
	tests := []struct {
		name          string
		offset        int64
		whence        int
		expectedPos   int64
		expectedError error
	}{
		{
			name:          "Seek from start with valid offset",
			offset:        5,
			whence:        io.SeekStart,
			expectedPos:   5,
			expectedError: nil,
		},
		{
			name:          "Seek from start with negative offset",
			offset:        -3,
			whence:        io.SeekStart,
			expectedPos:   0,
			expectedError: datasource.ErrOutOfRange,
		},
		{
			name:          "Seek from start with out-of-range offset",
			offset:        15,
			whence:        io.SeekStart,
			expectedPos:   0,
			expectedError: datasource.ErrOutOfRange,
		},
		{
			name:          "Seek from end with valid offset",
			offset:        -3,
			whence:        io.SeekEnd,
			expectedPos:   7,
			expectedError: nil,
		},
		{
			name:          "Seek from end with positive offset",
			offset:        3,
			whence:        io.SeekEnd,
			expectedPos:   0,
			expectedError: datasource.ErrOutOfRange,
		},
		{
			name:          "Seek from current with valid offset",
			offset:        2,
			whence:        io.SeekCurrent,
			expectedPos:   7,
			expectedError: nil,
		},
		{
			name:          "Seek from current with negative offset",
			offset:        -5,
			whence:        io.SeekCurrent,
			expectedPos:   0,
			expectedError: nil,
		},
		{
			name:          "Seek from current with out-of-range offset",
			offset:        10,
			whence:        io.SeekCurrent,
			expectedPos:   0,
			expectedError: datasource.ErrOutOfRange,
		},
		{
			name:          "Invalid whence value",
			offset:        0,
			whence:        123, // Invalid whence value
			expectedPos:   0,
			expectedError: datasource.ErrOutOfRange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFtpConn := NewMockServerConn(ctrl)

			// Mock response for Retr method
			response := NewMockftpResponse(ctrl)
			mockFtpConn.EXPECT().Retr("/ftp/one/testfile2.txt").Return(response, nil)

			// Mock ReadAll method of response
			response.EXPECT().Read(gomock.Any()).Return(10, io.EOF).AnyTimes()

			// Create ftpFile instance with mock dependencies
			file := &ftpFile{
				path:   "/ftp/one/testfile2.txt",
				conn:   mockFtpConn,
				offset: 5, // Starting offset for the file
			}

			// Perform Seek operation
			pos, err := file.Seek(tt.offset, tt.whence)

			// Assert the results
			assert.Equal(t, tt.expectedPos, pos)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

// The test defined below do not use any mocking. They need an actual ftp server connection.
func Test_ReadFromCSV(t *testing.T) {
	runFtpTest(t, func(fs *ftpFileSystem) {

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

		newCsvFile, _ := fs.Create("temp.csv")

		_, err := newCsvFile.Write([]byte(csvContent))
		if err != nil {
			t.Errorf("Write method failed: %v", err)
		}

		newCsvFile, _ = fs.Open("temp.csv")

		reader, _ := newCsvFile.ReadAll()

		defer func(fs datasource.FileSystem, name string) {
			_ = fs.RemoveAll(name)
		}(fs, "temp.csv")

		var i = 0

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
	runFtpTest(t, func(fs *ftpFileSystem) {
		var csvContent = `Name,Age,Email`

		newCsvFile, _ := fs.Create("temp.csv")

		_, _ = newCsvFile.Write([]byte(csvContent))
		newCsvFile, _ = fs.Open("temp.csv")

		reader, _ := newCsvFile.ReadAll()

		defer func(fs datasource.FileSystem, name string) {
			_ = fs.RemoveAll(name)
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
	runFtpTest(t, func(fs *ftpFileSystem) {
		var jsonContent = `[{"name": "Sam", "age": 123},{"name": "Jane", "age": 456},{"name": "John", "age": 789}]`

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var jsonValue = []User{{"Sam", 123}, {"Jane", 456}, {"John", 789}}

		newCsvFile, _ := fs.Create("temp.json")

		_, _ = newCsvFile.Write([]byte(jsonContent))
		newCsvFile, _ = fs.Open("temp.json")

		reader, _ := newCsvFile.ReadAll()

		defer func(fs datasource.FileSystem, name string) {
			_ = fs.RemoveAll(name)
		}(fs, "temp.json")

		var i = 0

		for reader.Next() {
			var u User

			err := reader.Scan(&u)

			assert.Equal(t, jsonValue[i].Name, u.Name)
			assert.Equal(t, jsonValue[i].Age, u.Age)

			assert.NoError(t, err)

			i++
		}
	})
}

func Test_ReadFromJSONObject(t *testing.T) {
	runFtpTest(t, func(fs *ftpFileSystem) {
		var jsonContent = `{"name": "Sam", "age": 123}`

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		newCsvFile, _ := fs.Create("temp.json")

		_, _ = newCsvFile.Write([]byte(jsonContent))
		newCsvFile, _ = fs.Open("temp.json")

		reader, _ := newCsvFile.ReadAll()

		defer func(fs datasource.FileSystem, name string) {
			_ = fs.RemoveAll(name)
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
	runFtpTest(t, func(fs *ftpFileSystem) {
		var jsonContent = `!@#$%^&*`

		newCsvFile, _ := fs.Create("temp.json")

		_, _ = newCsvFile.Write([]byte(jsonContent))

		newCsvFile.Close()

		newCsvFile, _ = fs.Open("temp.json")

		_, err := newCsvFile.ReadAll()

		defer func(fileStore datasource.FileSystem, name string) {
			_ = fileStore.RemoveAll(name)
		}(fs, "temp.json")

		assert.IsType(t, &json.SyntaxError{}, err)
	})
}

// Helper function to run FTP tests requiring actual ftp connection.
// LoadConfig loads FTP configuration from environment variables.
func LoadConfig() *Config {
	err := godotenv.Load("./configs/.env")
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	config := Config{
		Host:      os.Getenv("FtpHost"),
		User:      os.Getenv("FtpUser"),
		Password:  os.Getenv("FtpPassword"),
		Port:      os.Getenv("FtpPort"),
		RemoteDir: os.Getenv("FtpRemoteDir"),
	}

	return &config
}

func runFtpTest(t *testing.T, testFunc func(fs *ftpFileSystem)) {
	t.Helper()

	config := LoadConfig()

	ftpClient := New(config)

	val, ok := ftpClient.(*ftpFileSystem)
	if ok {
		logger := logging.NewMockLogger(logging.DEBUG)

		val.UseLogger(logger)
		val.Connect()
	}

	// Run the test function with the initialized file system
	testFunc(val)
}
