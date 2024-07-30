package ftp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestRead(t *testing.T) {
	var tests = []struct {
		name             string
		filePath         string
		mockReadResponse func(response *MockftpResponse)
		expectError      bool
	}{
		{
			name:     "Successful read",
			filePath: "/ftp/one/testfile2.txt",
			mockReadResponse: func(response *MockftpResponse) {
				response.EXPECT().Read(gomock.Any()).Return(10, io.EOF)
				response.EXPECT().Close().Return(nil)
			},
			expectError: true,
		},
		{
			name:     "Read with error",
			filePath: "/ftp/one/nonexistent.txt",
			mockReadResponse: func(response *MockftpResponse) {
				response.EXPECT().Read(gomock.Any()).Return(0, errors.New("mocked read error"))
			},
			expectError: true,
		},
		{
			name:     "File does not exist",
			filePath: "/ftp/one/nonexistent.txt",
			mockReadResponse: func(_ *MockftpResponse) {
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	// Create ftpFileSystem instance
	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewLogger(INFO))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := NewMockftpResponse(ctrl)

			file := ftpFile{path: tt.filePath, conn: fs.conn, logger: fs.logger}

			if tt.name != "File does not exist" {
				mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(file.offset)).Return(response, nil)
			} else {
				mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(file.offset)).Return(nil, errors.New("file not found error"))
			}

			tt.mockReadResponse(response)

			s := make([]byte, 1024)

			_, err := file.Read(s)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestReadAt(t *testing.T) {
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
				response.EXPECT().Read(gomock.Any()).Return(10, io.EOF)
				response.EXPECT().Close().Return(nil)
			},
			expectError: true,
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
		{
			name:     "File does not exist",
			filePath: "/ftp/one/nonexistent.txt",
			offset:   0,
			mockReadResponse: func(_ *MockftpResponse) {
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	// Create ftpFileSystem instance
	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewLogger(INFO))

	for _, tt := range readAtTests {
		t.Run(tt.name, func(t *testing.T) {
			response := NewMockftpResponse(ctrl)

			if tt.name != "File does not exist" {
				mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(tt.offset)).Return(response, nil)
			} else {
				mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(tt.offset)).Return(nil, errors.New("file not found error"))
			}

			tt.mockReadResponse(response)

			s := make([]byte, 1024)

			// Create ftpFile instance
			file := ftpFile{path: tt.filePath, conn: fs.conn, logger: fs.logger}

			_, err := file.ReadAt(s, tt.offset)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestWrite(t *testing.T) {
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
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(0)).Return(nil)
			},
			expectError: false,
		},
		{
			name:     "Write with error",
			filePath: "/ftp/one/nonexistent.txt",
			mockWriteExpect: func(conn *MockServerConn, filePath string) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(0)).Return(errors.New("mocked write error"))
			},
			expectError: true,
		},
		{
			name:     "File does not exist",
			filePath: "/ftp/one/nonexistent.txt",
			mockWriteExpect: func(conn *MockServerConn, filePath string) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(0)).Return(errors.New("file not found error"))
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	// Create ftpFileSystem instance
	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewLogger(INFO))

	for _, tt := range writeTests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockWriteExpect(mockFtpConn, tt.filePath)

			// Create ftpFile instance
			file := ftpFile{path: tt.filePath, conn: fs.conn, logger: fs.logger}

			_, err := file.Write([]byte("test content"))

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestWriteAt(t *testing.T) {
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
		{
			name:     "File does not exist",
			filePath: "/ftp/one/nonexistent.txt",
			offset:   0,
			mockWriteExpect: func(conn *MockServerConn, filePath string, offset int64) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(offset)).Return(errors.New("file not found error"))
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	// Create ftpFileSystem instance
	fs := &ftpFileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
	}

	fs.UseLogger(NewLogger(INFO))

	for _, tt := range writeAtTests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockWriteExpect(mockFtpConn, tt.filePath, tt.offset)

			// Create ftpFile instance
			file := ftpFile{path: tt.filePath, conn: fs.conn, logger: fs.logger}

			_, err := file.WriteAt([]byte("test content"), tt.offset)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestSeek(t *testing.T) {
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
			expectedError: ErrOutOfRange,
		},
		{
			name:          "Seek from start with out-of-range offset",
			offset:        15,
			whence:        io.SeekStart,
			expectedPos:   0,
			expectedError: ErrOutOfRange,
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
			expectedError: ErrOutOfRange,
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
			expectedError: ErrOutOfRange,
		},
		{
			name:          "Invalid whence value",
			offset:        0,
			whence:        123, // Invalid whence value
			expectedPos:   0,
			expectedError: ErrOutOfRange,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockServerConn(ctrl)

	file := &ftpFile{
		path:   "/ftp/one/testfile2.txt",
		conn:   mockFtpConn,
		offset: 5, // Starting offset for the file
		logger: NewLogger(INFO),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFtpConn.EXPECT().FileSize("/ftp/one/testfile2.txt").Return(int64(10), nil)

			pos, err := file.Seek(tt.offset, tt.whence)

			assert.Equal(t, tt.expectedPos, pos)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

// The test defined below do not use any mocking. They need an actual ftp server connection.
func Test_ReadFromCSV(t *testing.T) {
	runFtpTest(t, func(fs FileSystem) {
		var csvContent = `Name,Age,Email
John Doe,30,johndoe@example.com
Jane Smith,25,janesmith@example.com
Emily Johnson,35,emilyj@example.com
Michael Brown,40,michaelb@example.com
David Lee,27,davidlee@example.com
Sarah Wilson,45,sarahw@example.com
Matthew Taylor,38,matthewt@example.com
Olivia Moore,29,oliviam@example.com
Daniel Clark,33,danielc@example.com
Sophia Garcia,42,sophiag@example.com
Andrew Martinez,31,andrewm@example.com
Alexandra Anderson,36,alexandra@example.com
Benjamin Young,39,benjaminy@example.com
Hannah Rodriguez,26,hannahr@example.com
William Hernandez,41,williamh@example.com
Samantha Martinez,34,samantham@example.com
Christopher Davis,37,christopherd@example.com
Lauren White,28,laurenw@example.com
Gabriel Scott,43,gabriels@example.com
Victoria Nguyen,32,victorian@example.com
John Doe,30,johndoe@example.com
Jane Smith,25,janesmith@example.com
Emily Johnson,35,emilyj@example.com
Michael Brown,40,michaelb@example.com
David Lee,27,davidlee@example.com
Sarah Wilson,45,sarahw@example.com
Matthew Taylor,38,matthewt@example.com
Olivia Moore,29,oliviam@example.com
Daniel Clark,33,danielc@example.com
Sophia Garcia,42,sophiag@example.com
Andrew Martinez,31,andrewm@example.com
Alexandra Anderson,36,alexandra@example.com
Benjamin Young,39,benjaminy@example.com
Hannah Rodriguez,26,hannahr@example.com
William Hernandez,41,williamh@example.com
Samantha Martinez,34,samantham@example.com
Christopher Davis,37,christopherd@example.com
Lauren White,28,laurenw@example.com
Gabriel Scott,43,gabriels@example.com
Victoria Nguyen,32,victorian@example.com`

		csvValue := []string{
			"Name,Age,Email",
			"John Doe,30,johndoe@example.com",
			"Jane Smith,25,janesmith@example.com",
			"Emily Johnson,35,emilyj@example.com",
			"Michael Brown,40,michaelb@example.com",
			"David Lee,27,davidlee@example.com",
			"Sarah Wilson,45,sarahw@example.com",
			"Matthew Taylor,38,matthewt@example.com",
			"Olivia Moore,29,oliviam@example.com",
			"Daniel Clark,33,danielc@example.com",
			"Sophia Garcia,42,sophiag@example.com",
			"Andrew Martinez,31,andrewm@example.com",
			"Alexandra Anderson,36,alexandra@example.com",
			"Benjamin Young,39,benjaminy@example.com",
			"Hannah Rodriguez,26,hannahr@example.com",
			"William Hernandez,41,williamh@example.com",
			"Samantha Martinez,34,samantham@example.com",
			"Christopher Davis,37,christopherd@example.com",
			"Lauren White,28,laurenw@example.com",
			"Gabriel Scott,43,gabriels@example.com",
			"Victoria Nguyen,32,victorian@example.com",
			"John Doe,30,johndoe@example.com",
			"Jane Smith,25,janesmith@example.com",
			"Emily Johnson,35,emilyj@example.com",
			"Michael Brown,40,michaelb@example.com",
			"David Lee,27,davidlee@example.com",
			"Sarah Wilson,45,sarahw@example.com",
			"Matthew Taylor,38,matthewt@example.com",
			"Olivia Moore,29,oliviam@example.com",
			"Daniel Clark,33,danielc@example.com",
			"Sophia Garcia,42,sophiag@example.com",
			"Andrew Martinez,31,andrewm@example.com",
			"Alexandra Anderson,36,alexandra@example.com",
			"Benjamin Young,39,benjaminy@example.com",
			"Hannah Rodriguez,26,hannahr@example.com",
			"William Hernandez,41,williamh@example.com",
			"Samantha Martinez,34,samantham@example.com",
			"Christopher Davis,37,christopherd@example.com",
			"Lauren White,28,laurenw@example.com",
			"Gabriel Scott,43,gabriels@example.com",
			"Victoria Nguyen,32,victorian@example.com",
		}

		newCsvFile, _ := fs.Create("temp.csv")

		_, err := newCsvFile.Write([]byte(csvContent))

		if err != nil {
			t.Errorf("Write method failed: %v", err)
		}

		newCsvFile, _ = fs.Open("temp.csv")

		defer func(fs FileSystem, name string) {
			_ = fs.Remove(name)
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
	runFtpTest(t, func(fs FileSystem) {
		var csvContent = `Name,Age,Email`

		newCsvFile, _ := fs.Create("temp.csv")

		_, _ = newCsvFile.Write([]byte(csvContent))
		newCsvFile, _ = fs.Open("temp.csv")

		reader, _ := newCsvFile.ReadAll()

		defer func(fs FileSystem, name string) {
			err := fs.Remove(name)
			if err != nil {
				t.Error(err)
			}
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
	runFtpTest(t, func(fs FileSystem) {
		var jsonContent = `[{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789},
{"name": "Sam", "age": 123},
{"name": "Jane", "age": 456},
{"name": "John", "age": 789}]`

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var jsonValue = []User{{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
			{"Sam", 123},
			{"Jane", 456},
			{"John", 789},
		}

		newCsvFile, _ := fs.Create("temp.json")

		_, _ = newCsvFile.Write([]byte(jsonContent))
		newCsvFile, _ = fs.Open("temp.json")

		defer func(fs FileSystem, name string) {
			err := fs.Remove(name)
			if err != nil {
				t.Error(err)
			}
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
	runFtpTest(t, func(fs FileSystem) {
		var jsonContent = `{"name": "Sam", "age": 123}`

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		newCsvFile, _ := fs.Create("temp.json")

		_, _ = newCsvFile.Write([]byte(jsonContent))
		newCsvFile, _ = fs.Open("temp.json")

		reader, _ := newCsvFile.ReadAll()

		defer func(fs FileSystem, name string) {
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
	runFtpTest(t, func(fs FileSystem) {
		var jsonContent = `!@#$%^&*`

		newCsvFile, _ := fs.Create("temp.json")

		_, _ = newCsvFile.Write([]byte(jsonContent))

		newCsvFile.Close()

		newCsvFile, _ = fs.Open("temp.json")

		_, err := newCsvFile.ReadAll()

		defer func(fs FileSystem, name string) {
			removeErr := fs.Remove(name)
			if removeErr != nil {
				t.Error(removeErr)
			}
		}(fs, "temp.json")

		assert.IsType(t, &json.SyntaxError{}, err)
	})
}

func runFtpTest(t *testing.T, testFunc func(fs FileSystem)) {
	t.Helper()

	config := &Config{
		Host:      "127.0.0.1",
		User:      "one",
		Password:  "1234",
		Port:      21,
		RemoteDir: "/ftp/one",
	}

	ftpClient := New(config)

	ftpClient.UseLogger(NewLogger(INFO))
	ftpClient.Connect()

	// Run the test function with the initialized file system
	testFunc(ftpClient)
}
