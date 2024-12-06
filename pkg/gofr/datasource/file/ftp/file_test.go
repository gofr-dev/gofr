package ftp

import (
	"bytes"
	"io"
	"math"
	"os"
	"testing"
	"time"

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
				response.EXPECT().Read(gomock.Any()).Return(0, errMockSentinel)
				response.EXPECT().Close().Return(nil)
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

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Create ftpFileSystem instance
	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := NewMockftpResponse(ctrl)

			f := File{path: tt.filePath, conn: fs.conn, logger: fs.logger, metrics: fs.metrics}

			if tt.name != "File does not exist" {
				//nolint:gosec // We ensure the offset is never negative in the application logic.
				mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(f.offset)).Return(response, nil)
			} else {
				//nolint:gosec // We ensure the offset is never negative in the application logic.
				mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(f.offset)).Return(nil, errNotFound)
			}

			tt.mockReadResponse(response)

			s := make([]byte, 1024)

			_, err := f.Read(s)

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
				response.EXPECT().Read(gomock.Any()).Return(0, errMockSentinel)
				response.EXPECT().Close().Return(nil)
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

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Create ftpFileSystem instance
	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}
	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range readAtTests {
		t.Run(tt.name, func(t *testing.T) {
			response := NewMockftpResponse(ctrl)

			if tt.name != "File does not exist" {
				mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(math.Abs(float64(tt.offset)))).Return(response, nil)
			} else {
				mockFtpConn.EXPECT().RetrFrom(tt.filePath, uint64(math.Abs(float64(tt.offset)))).Return(nil, errNotFound)
			}

			tt.mockReadResponse(response)

			s := make([]byte, 1024)

			// Create ftpFile instance
			f := File{path: tt.filePath, conn: fs.conn, logger: fs.logger, metrics: fs.metrics}

			_, err := f.ReadAt(s, tt.offset)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestWrite(t *testing.T) {
	var writeTests = []struct {
		name            string
		filePath        string
		mockWriteExpect func(conn *MockserverConn, filePath string)
		expectError     bool
	}{
		{
			name:     "Successful write",
			filePath: "/ftp/one/testfile.txt",
			mockWriteExpect: func(conn *MockserverConn, filePath string) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(0)).Return(nil)
				conn.EXPECT().GetTime(filePath).Return(time.Now(), nil)
			},
			expectError: false,
		},
		{
			name:     "Write with error",
			filePath: "/ftp/one/nonexistent.txt",
			mockWriteExpect: func(conn *MockserverConn, filePath string) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(0)).Return(errMockSentinel)
			},
			expectError: true,
		},
		{
			name:     "File does not exist",
			filePath: "/ftp/one/nonexistent.txt",
			mockWriteExpect: func(conn *MockserverConn, filePath string) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(0)).Return(errNotFound)
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Create ftpFileSystem instance
	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}
	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range writeTests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockWriteExpect(mockFtpConn, tt.filePath)

			// Create ftpFile instance
			f := File{path: tt.filePath, conn: fs.conn, logger: fs.logger, metrics: fs.metrics}

			_, err := f.Write([]byte("test content"))

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestWriteAt(t *testing.T) {
	var writeAtTests = []struct {
		name            string
		filePath        string
		offset          int64
		mockWriteExpect func(conn *MockserverConn, filePath string, offset int64)
		expectError     bool
	}{
		{
			name:     "Successful write at offset",
			filePath: "/ftp/one/testfile.txt",
			offset:   3,
			mockWriteExpect: func(conn *MockserverConn, filePath string, offset int64) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(math.Abs(float64(offset)))).Return(nil)
				conn.EXPECT().GetTime(filePath).Return(time.Now(), nil)
			},
			expectError: false,
		},
		{
			name:     "WriteAt with error",
			filePath: "/ftp/one/nonexistent.txt",
			offset:   0,
			mockWriteExpect: func(conn *MockserverConn, filePath string, offset int64) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(math.Abs(float64(offset)))).Return(errMockSentinel)
			},
			expectError: true,
		},
		{
			name:     "File does not exist",
			filePath: "/ftp/one/nonexistent.txt",
			offset:   0,
			mockWriteExpect: func(conn *MockserverConn, filePath string, offset int64) {
				emptyReader := bytes.NewReader([]byte("test content"))
				conn.EXPECT().StorFrom(filePath, emptyReader, uint64(math.Abs(float64(offset)))).Return(errNotFound)
			},
			expectError: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Create ftpFileSystem instance
	fs := &FileSystem{
		conn: mockFtpConn,
		config: &Config{
			Host:      "ftp.example.com",
			User:      "username",
			Password:  "password",
			Port:      21,
			RemoteDir: "/ftp/one",
		},
		logger:  mockLogger,
		metrics: mockMetrics,
	}
	// mocked logs
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range writeAtTests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockWriteExpect(mockFtpConn, tt.filePath, tt.offset)

			// Create ftpFile instance
			f := File{path: tt.filePath, conn: fs.conn, logger: fs.logger, metrics: fs.metrics}

			_, err := f.WriteAt([]byte("test content"), tt.offset)

			assert.Equal(t, tt.expectError, err != nil, tt.name)
		})
	}
}

func TestSeek(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFtpConn := NewMockserverConn(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	file := &File{
		path:    "/ftp/one/testfile2.txt",
		conn:    mockFtpConn,
		offset:  5, // Starting offset for the file
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	tests := getSeekTestCases()

	// Common mock setups for logger and metrics
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(5)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSeekTest(t, file, mockFtpConn, tt)
		})
	}
}

func getSeekTestCases() []struct {
	name          string
	offset        int64
	whence        int
	expectedPos   int64
	expectedError error
} {
	return []struct {
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
			name:          "Seek from end with valid offset",
			offset:        -3,
			whence:        io.SeekEnd,
			expectedPos:   7,
			expectedError: nil,
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
			name:          "Seek from end with positive offset",
			offset:        3,
			whence:        io.SeekEnd,
			expectedPos:   0,
			expectedError: ErrOutOfRange,
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
			expectedError: os.ErrInvalid,
		},
	}
}

func runSeekTest(
	t *testing.T,
	file *File,
	mockFtpConn *MockserverConn,
	tt struct {
		name          string
		offset        int64
		whence        int
		expectedPos   int64
		expectedError error
	},
) {
	t.Helper()

	mockFtpConn.EXPECT().FileSize(file.path).Return(int64(10), nil)

	pos, err := file.Seek(tt.offset, tt.whence)
	file.offset = 5 // Reset file offset after each test

	assert.Equal(t, tt.expectedPos, pos)
	assert.Equal(t, tt.expectedError, err)
}
