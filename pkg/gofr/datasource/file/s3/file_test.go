package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	ErrGetObject     = errors.New("failed to get object from S3")
	ErrPutObject     = errors.New("failed to put object to S3")
	ErrCloseFailed   = errors.New("close failed")
	ErrReadAllFailed = errors.New("simulated io.ReadAll error")
)

// Helper function to create a new S3File instance for testing.
func newTestS3File(t *testing.T, ctrl *gomock.Controller, name string, size, offset int64) *S3File {
	t.Helper()
	return newTestS3FileWithTime(t, ctrl, name, size, offset, time.Now())
}

// Helper function to create a new S3File instance for testing with custom time.
func newTestS3FileWithTime(_ *testing.T, ctrl *gomock.Controller, name string, size, offset int64,
	lastModified time.Time) *S3File {
	mockClient := NewMocks3Client(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := NewMockLogger(ctrl)

	// Set up default logger expectations for common operations
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	return &S3File{
		conn:         mockClient,
		name:         name,
		offset:       offset,
		logger:       mockLogger,
		metrics:      mockMetrics,
		size:         size,
		lastModified: lastModified,
	}
}

// Helper to create a successful GetObjectOutput.
func getObjectOutput(content string) *s3.GetObjectOutput {
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader([]byte(content))),
		ContentLength: aws.Int64(int64(len(content))),
	}
}

// Helper to create a successful PutObjectOutput.
func putObjectOutput() *s3.PutObjectOutput {
	return &s3.PutObjectOutput{}
}

// Define mock for io.ReadCloser to test Close.
type mockReadCloser struct {
	io.Reader
	closeErr error
}

func (m *mockReadCloser) Close() error {
	return m.closeErr
}

// TestClose tests the Close method of S3File.
func TestS3File_Close(t *testing.T) {
	testCases := []struct {
		name     string
		body     io.ReadCloser
		expected error
	}{
		{
			name:     "Success_BodyNil",
			body:     nil,
			expected: nil,
		},
		{
			name:     "Success_BodyNotNil",
			body:     &mockReadCloser{Reader: bytes.NewReader([]byte("test")), closeErr: nil},
			expected: nil,
		},
		{
			name:     "Failure_CloseError",
			body:     &mockReadCloser{Reader: bytes.NewReader([]byte("test")), closeErr: ErrCloseFailed},
			expected: ErrCloseFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, "test-bucket/test-file.txt", 10, 0)
			f.body = tc.body

			err := f.Close()

			if !errors.Is(err, tc.expected) {
				if tc.expected == nil && err != nil {
					t.Errorf("Expected nil error, got %v", err)
				} else if tc.expected != nil && err == nil {
					t.Errorf("Expected error %v, got nil", tc.expected)
				} else if tc.expected != nil && !strings.Contains(err.Error(), tc.expected.Error()) {
					t.Errorf("Expected error containing %q, got %q", tc.expected.Error(), err.Error())
				}
			}
		})
	}
}

var errS3Test = errors.New("s3 error")

// TestS3File_Read_Success tests the successful Read operations of S3File.
func TestS3File_Read_Success(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	content := "This is a test file content."

	testCases := []struct {
		name          string
		offset        int64
		bufferLen     int
		mockGetObject func(m *Mocks3ClientMockRecorder)
		expectedN     int
		expectedP     string
		expectedErr   error
	}{
		{
			name:      "Success_ReadFromStart",
			offset:    0,
			bufferLen: 5,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Eq(&s3.GetObjectInput{
					Bucket: aws.String(bucketName),
					Key:    aws.String(fileName),
				})).Return(getObjectOutput(content), nil)
			},
			expectedN:   5,
			expectedP:   "This ",
			expectedErr: nil,
		},
		{
			name:      "Success_ReadFromOffset",
			offset:    5,
			bufferLen: 4,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Eq(&s3.GetObjectInput{
					Bucket: aws.String(bucketName),
					Key:    aws.String(fileName),
				})).Return(getObjectOutput(content), nil)
			},
			expectedN:   4,
			expectedP:   "is a",
			expectedErr: nil,
		},
		{
			name:      "Success_ReadToEOF",
			offset:    0,
			bufferLen: len(content),
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(content), nil)
			},
			expectedN:   len(content),
			expectedP:   content,
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, int64(len(content)), tc.offset)

			m := f.conn.(*Mocks3Client)

			if tc.mockGetObject != nil {
				tc.mockGetObject(m.EXPECT())
			}

			// Use the buffer length defined in the test case
			p := make([]byte, tc.bufferLen)

			// Reset buffer for clean comparison
			for i := range p {
				p[i] = 0
			}

			n, err := f.Read(p)

			// Check if the expected error is the one returned (or wrapped)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if n != tc.expectedN {
					t.Errorf("Expected bytes read %d, got %d", tc.expectedN, n)
				}

				// We only check up to n bytes of the expected and actual content
				if string(p[:n]) != tc.expectedP[:n] {
					t.Errorf("Expected content %q, got %q", tc.expectedP[:n], string(p[:n]))
				}
			}
		})
	}
}

// TestS3File_Read_Failure tests the failure cases of S3File Read operations.
func TestS3File_Read_Failure(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	content := "This is a test file content."

	testCases := []struct {
		name          string
		offset        int64
		bufferLen     int
		mockGetObject func(m *Mocks3ClientMockRecorder)
		expectedN     int
		expectedP     string
		expectedErr   error
	}{
		{
			name:      "Failure_GetObjectError",
			offset:    0,
			bufferLen: 10,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(nil, errS3Test)
			},
			expectedN:   0,
			expectedP:   "",
			expectedErr: errS3Test,
		},
		{
			name:      "Failure_NilResponse",
			offset:    0,
			bufferLen: 10,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{Body: nil}, nil)
			},
			expectedN:   0,
			expectedP:   "",
			expectedErr: ErrNilResponse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, int64(len(content)), tc.offset)

			m := f.conn.(*Mocks3Client)

			if tc.mockGetObject != nil {
				tc.mockGetObject(m.EXPECT())
			}

			// Use the buffer length defined in the test case
			p := make([]byte, tc.bufferLen)

			// Reset buffer for clean comparison
			for i := range p {
				p[i] = 0
			}

			n, err := f.Read(p)

			// Check if the expected error is the one returned (or wrapped)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if n != tc.expectedN {
					t.Errorf("Expected bytes read %d, got %d", tc.expectedN, n)
				}

				// We only check up to n bytes of the expected and actual content
				if string(p[:n]) != tc.expectedP[:n] {
					t.Errorf("Expected content %q, got %q", tc.expectedP[:n], string(p[:n]))
				}
			}
		})
	}
}

// TestReadAt tests the ReadAt method of S3File.
func TestS3File_ReadAt(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	content := "This is a test file content."
	fileSize := int64(len(content))

	testCases := []struct {
		name          string
		size          int64
		readAtOffset  int64
		bufferLen     int
		mockGetObject func(m *Mocks3ClientMockRecorder)
		expectedN     int
		expectedP     string
		expectedErr   error
	}{
		{
			name:         "Success_ReadFromMiddle",
			size:         fileSize,
			readAtOffset: 5,
			bufferLen:    4,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(content), nil)
			},
			expectedN:   4,
			expectedP:   "is a",
			expectedErr: nil,
		},
		{
			name:         "Failure_GetObjectError",
			size:         fileSize,
			readAtOffset: 0,
			bufferLen:    10,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(nil, errS3Test)
			},
			expectedN:   0,
			expectedP:   "",
			expectedErr: errS3Test,
		},
		{
			name:         "Failure_NilBody",
			size:         fileSize,
			readAtOffset: 0,
			bufferLen:    10,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{Body: nil}, nil)
			},
			expectedN:   0,
			expectedP:   "",
			expectedErr: io.EOF,
		},
		{
			name:         "Failure_OutOfRange",
			size:         fileSize,
			readAtOffset: 25,
			bufferLen:    4,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(content), nil)
			},
			expectedN:   0,
			expectedP:   "",
			expectedErr: ErrOutOfRange,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, tc.size, 10)
			m := f.conn.(*Mocks3Client)

			if tc.mockGetObject != nil {
				tc.mockGetObject(m.EXPECT())
			}

			p := make([]byte, tc.bufferLen)
			n, err := f.ReadAt(p, tc.readAtOffset)

			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if n != tc.expectedN {
					t.Errorf("Expected bytes read %d, got %d", tc.expectedN, n)
				}

				if string(p[:n]) != tc.expectedP[:n] {
					t.Errorf("Expected content %q, got %q", tc.expectedP[:n], string(p[:n]))
				}

				if f.offset != 10 {
					t.Errorf("ReadAt modified offset. Expected 10, got %d", f.offset)
				}
			}
		})
	}
}

// TestS3File_Write_Success tests the successful Write operations of S3File.
func TestS3File_Write_Success(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	initialContent := "Hello, World!"
	initialSize := int64(len(initialContent))
	dataToWrite := []byte("GoFr")

	testCases := []struct {
		name             string
		initialOffset    int64
		initialSize      int64
		dataToWrite      []byte
		mockExpectations func(m *Mocks3ClientMockRecorder)
		expectedN        int
		expectedOffset   int64
		expectedSize     int64
		expectedErr      error
	}{
		{
			name:          "Success_WriteFromStart_NewFile",
			initialOffset: 0,
			initialSize:   0,
			dataToWrite:   dataToWrite,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.PutObject(gomock.Any(), gomock.Any()).Return(putObjectOutput(), nil)
			},
			expectedN:      len(dataToWrite),
			expectedOffset: int64(len(dataToWrite)),
			expectedSize:   int64(len(dataToWrite)),
			expectedErr:    nil,
		},
		{
			name:          "Success_WriteFromStart_Overwrite",
			initialOffset: 0,
			initialSize:   initialSize,
			dataToWrite:   dataToWrite,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.PutObject(gomock.Any(), gomock.Any()).Return(putObjectOutput(), nil)
			},
			expectedN:      len(dataToWrite),
			expectedOffset: int64(len(dataToWrite)),
			expectedSize:   int64(len(dataToWrite)),
			expectedErr:    nil,
		},
		{
			name:          "Success_WriteFromMiddle",
			initialOffset: 7,
			initialSize:   initialSize,
			dataToWrite:   dataToWrite,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(initialContent), nil)
				m.PutObject(gomock.Any(), gomock.Any()).Return(putObjectOutput(), nil)
			},
			expectedN:      len(dataToWrite),
			expectedOffset: 7 + int64(len(dataToWrite)),
			expectedSize:   initialSize,
			expectedErr:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, tc.initialSize, tc.initialOffset)

			m := f.conn.(*Mocks3Client)

			if tc.mockExpectations != nil {
				tc.mockExpectations(m.EXPECT())
			}

			n, err := f.Write(tc.dataToWrite)

			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if n != tc.expectedN {
					t.Errorf("Expected bytes written %d, got %d", tc.expectedN, n)
				}

				if f.offset != tc.expectedOffset {
					t.Errorf("Expected offset %d, got %d", tc.expectedOffset, f.offset)
				}

				if f.size != tc.expectedSize {
					t.Errorf("Expected size %d, got %d", tc.expectedSize, f.size)
				}
			}
		})
	}
}

// TestS3File_Write_Failure tests the failure cases of S3File Write operations.
func TestS3File_Write_Failure(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	initialContent := "Hello, World!"
	initialSize := int64(len(initialContent))
	dataToWrite := []byte("GoFr")

	testCases := []struct {
		name             string
		initialOffset    int64
		initialSize      int64
		dataToWrite      []byte
		mockExpectations func(m *Mocks3ClientMockRecorder)
		expectedN        int
		expectedOffset   int64
		expectedSize     int64
		expectedErr      error
	}{
		{
			name:          "Failure_GetObjectError",
			initialOffset: 5,
			initialSize:   initialSize,
			dataToWrite:   dataToWrite,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(nil, errS3Test)
			},
			expectedN:      0,
			expectedOffset: 5,
			expectedSize:   initialSize,
			expectedErr:    errS3Test,
		},
		{
			name:          "Failure_PutObjectError",
			initialOffset: 0,
			initialSize:   initialSize,
			dataToWrite:   dataToWrite,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.PutObject(gomock.Any(), gomock.Any()).Return(nil, errS3Test)
			},
			expectedN:      0,
			expectedOffset: 0,
			expectedSize:   initialSize,
			expectedErr:    errS3Test,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, tc.initialSize, tc.initialOffset)

			m := f.conn.(*Mocks3Client)

			if tc.mockExpectations != nil {
				tc.mockExpectations(m.EXPECT())
			}

			n, err := f.Write(tc.dataToWrite)

			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if n != tc.expectedN {
					t.Errorf("Expected bytes written %d, got %d", tc.expectedN, n)
				}

				if f.offset != tc.expectedOffset {
					t.Errorf("Expected offset %d, got %d", tc.expectedOffset, f.offset)
				}

				if f.size != tc.expectedSize {
					t.Errorf("Expected size %d, got %d", tc.expectedSize, f.size)
				}
			}
		})
	}
}

// TestWriteAt tests the WriteAt method of S3File.
func TestS3File_WriteAt(t *testing.T) {
	bucketName, fileName := "test-bucket", "test-file.txt"
	fullPath := bucketName + "/" + fileName
	initialContent := "Hello, World!"
	initialSize, dataToWrite := int64(len(initialContent)), []byte("GoFr")

	testCases := []struct {
		name                         string
		writeAtOffset, initialOffset int64
		dataToWrite                  []byte
		mockExpectations             func(m *Mocks3ClientMockRecorder)
		expectedN                    int
		expectedOffset, expectedSize int64
		expectedErr                  error
	}{
		{
			name:          "Success_WriteAtMiddle",
			initialOffset: 10, writeAtOffset: 7,
			dataToWrite: dataToWrite,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(initialContent), nil)

				expectedPutBody := []byte("Hello, GoFrd!")

				m.PutObject(gomock.Any(), gomock.Any()).Do(func(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) {
					actualPutBody := getBodyContent(t, params.Body)
					if !bytes.Equal(expectedPutBody, actualPutBody) {
						t.Errorf("PutObject Body mismatch. Expected: %q, Got: %q", string(expectedPutBody), string(actualPutBody))
						t.FailNow()
					}
				}).Return(putObjectOutput(), nil)
			},
			expectedN: len(dataToWrite), expectedOffset: 10, expectedSize: initialSize,
			expectedErr: nil,
		},
		{
			name:          "Failure_GetObjectError",
			initialOffset: 10, writeAtOffset: 5,
			dataToWrite: dataToWrite,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(nil, ErrGetObject)
			},
			expectedN: 0, expectedOffset: 10, expectedSize: initialSize,
			expectedErr: ErrGetObject,
		},
		{
			name:          "Failure_PutObjectError",
			initialOffset: 10, writeAtOffset: 0,
			dataToWrite: dataToWrite,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(initialContent), nil)
				m.PutObject(gomock.Any(), gomock.Any()).Return(nil, ErrPutObject)
			},
			expectedN: 0, expectedOffset: 10, expectedSize: initialSize,
			expectedErr: ErrPutObject,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, initialSize, tc.initialOffset)

			m := f.conn.(*Mocks3Client)
			if tc.mockExpectations != nil {
				tc.mockExpectations(m.EXPECT())
			}

			n, err := f.WriteAt(tc.dataToWrite, tc.writeAtOffset)

			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if n != tc.expectedN {
					t.Errorf("Expected bytes written %d, got %d", tc.expectedN, n)
				}

				if f.offset != tc.expectedOffset {
					t.Errorf("WriteAt modified offset. Expected %d, got %d", tc.expectedOffset, f.offset)
				}

				if f.size != tc.expectedSize {
					t.Errorf("Expected size %d, got %d", tc.expectedSize, f.size)
				}
			}
		})
	}
}

// Helper to read the content of the PutObjectInput Body.
func getBodyContent(t *testing.T, body io.Reader) []byte {
	t.Helper()

	b, err := io.ReadAll(body)
	require.NoError(t, err, "Failed to read PutObject body")

	return b
}

// TestS3File_Seek_Basic tests the basic Seek operations (SeekStart and SeekCurrent) of S3File.
func TestS3File_Seek_Basic(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	fileSize := int64(20) // Mock file size

	testCases := []struct {
		name              string
		initialOffset     int64
		offset            int64
		whence            int
		expectedNewOffset int64
		expectedErr       error
	}{
		{
			name:              "SeekStart_Success",
			initialOffset:     5,
			offset:            10,
			whence:            io.SeekStart,
			expectedNewOffset: 10,
			expectedErr:       nil,
		},
		{
			name:              "SeekStart_Failure_Negative",
			initialOffset:     5,
			offset:            -1,
			whence:            io.SeekStart,
			expectedNewOffset: 0,
			expectedErr:       ErrOutOfRange,
		},
		{
			name:              "SeekStart_Failure_TooLarge",
			initialOffset:     5,
			offset:            21,
			whence:            io.SeekStart,
			expectedNewOffset: 0,
			expectedErr:       ErrOutOfRange,
		},
		{
			name:              "SeekCurrent_Success",
			initialOffset:     5,
			offset:            10,
			whence:            io.SeekCurrent,
			expectedNewOffset: 15,
			expectedErr:       nil,
		},
		{
			name:              "SeekCurrent_Failure_NegativeResult",
			initialOffset:     5,
			offset:            -6,
			whence:            io.SeekCurrent,
			expectedNewOffset: 0,
			expectedErr:       ErrOutOfRange,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, fileSize, tc.initialOffset)

			newOffset, err := f.Seek(tc.offset, tc.whence)

			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if newOffset != tc.expectedNewOffset {
					t.Errorf("Expected new offset %d, got %d", tc.expectedNewOffset, newOffset)
				}

				if f.offset != tc.expectedNewOffset {
					t.Errorf("File struct offset was not updated. Expected %d, got %d", tc.expectedNewOffset, f.offset)
				}
			}
		})
	}
}

// TestS3File_Seek_Advanced tests the advanced Seek operations (SeekEnd and InvalidWhence) of S3File.
func TestS3File_Seek_Advanced(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	fileSize := int64(20) // Mock file size

	testCases := []struct {
		name              string
		initialOffset     int64
		offset            int64
		whence            int
		expectedNewOffset int64
		expectedErr       error
	}{
		{
			name:              "SeekEnd_Success",
			initialOffset:     5,
			offset:            -5,
			whence:            io.SeekEnd,
			expectedNewOffset: 15,
			expectedErr:       nil,
		},
		{
			name:              "SeekEnd_Failure_TooLarge",
			initialOffset:     5,
			offset:            1,
			whence:            io.SeekEnd,
			expectedNewOffset: 0,
			expectedErr:       ErrOutOfRange,
		},
		{
			name:              "SeekEnd_Success_ToStart",
			initialOffset:     5,
			offset:            -20,
			whence:            io.SeekEnd,
			expectedNewOffset: 0,
			expectedErr:       nil,
		},
		{
			name:              "Seek_InvalidWhence",
			initialOffset:     5,
			offset:            0,
			whence:            3, // Invalid whence
			expectedNewOffset: 0,
			expectedErr:       os.ErrInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, fileSize, tc.initialOffset)

			newOffset, err := f.Seek(tc.offset, tc.whence)

			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if newOffset != tc.expectedNewOffset {
					t.Errorf("Expected new offset %d, got %d", tc.expectedNewOffset, newOffset)
				}

				if f.offset != tc.expectedNewOffset {
					t.Errorf("File struct offset was not updated. Expected %d, got %d", tc.expectedNewOffset, f.offset)
				}
			}
		})
	}
}

// TestJsonReader_ValidObjects tests reading valid JSON objects from a jsonReader.
func TestJsonReader_ValidObjects(t *testing.T) {
	jsonContent := `[
		{"name": "Alice", "age": 30},
		{"name": "Bob", "age": 25}
	]`
	reader := bytes.NewReader([]byte(jsonContent))
	decoder := json.NewDecoder(reader)
	// Must consume the '[' token
	_, _ = decoder.Token()

	jReader := jsonReader{decoder: decoder}

	// Test first object
	require.True(t, jReader.Next(), "Expected Next to be true for the first object")

	var data1 struct {
		Name string
		Age  int
	}
	require.NoError(t, jReader.Scan(&data1), "Scan failed for first object")
	assert.Equal(t, "Alice", data1.Name)
	assert.Equal(t, 30, data1.Age)

	// Test second object
	require.True(t, jReader.Next(), "Expected Next to be true for the second object")

	var data2 struct {
		Name string
		Age  int
	}
	require.NoError(t, jReader.Scan(&data2), "Scan failed for second object")
	assert.Equal(t, "Bob", data2.Name)
	assert.Equal(t, 25, data2.Age)
}

// TestJsonReader_NullAndEnd tests reading null values and end of array from a jsonReader.
func TestJsonReader_NullAndEnd(t *testing.T) {
	jsonContent := `[
		{"name": "Alice", "age": 30},
		null
	]`
	reader := bytes.NewReader([]byte(jsonContent))
	decoder := json.NewDecoder(reader)
	// Must consume the '[' token
	_, _ = decoder.Token()

	jReader := jsonReader{decoder: decoder}

	// Skip first object
	jReader.Next()
	err := jReader.Scan(&struct{}{})
	require.NoError(t, err, "Scan failed for null object")

	// Test null object
	require.True(t, jReader.Next(), "Expected Next to be true for the null object")

	var data3 any
	require.NoError(t, jReader.Scan(&data3), "Scan failed for null object")
	assert.Nil(t, data3)

	// Test end of array
	assert.False(t, jReader.Next(), "Expected Next to be false at the end of the array")

	// Test Scan error after array end
	var invalidScanTarget struct{}
	require.Error(t, jReader.Scan(&invalidScanTarget), "Expected Scan to fail after array end")
}

// TestS3File_Metadata_Methods tests simple metadata methods.
func TestS3File_Metadata_Methods(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "path/to/my-file.txt"
	fullPath := bucketName + "/" + fileName
	testSize := int64(4096)
	testTime := time.Date(2023, 10, 10, 12, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	f := newTestS3FileWithTime(t, ctrl, fullPath, testSize, 0, testTime)

	// Test Name
	expectedName := "my-file.txt"
	assert.Equal(t, expectedName, f.Name())

	// Test Size
	assert.Equal(t, testSize, f.Size())

	// Test ModTime
	assert.Equal(t, testTime, f.ModTime())

	// Test IsDir
	assert.False(t, f.IsDir())

	// Test IsDir for a directory-like name
	f.name = bucketName + "/path/to/my-dir/"
	assert.True(t, f.IsDir())

	// Test Mode (placeholder)
	assert.Equal(t, os.FileMode(0), f.Mode())
}

// createMockBody creates a MockReadCloser for testing based on the test case parameters.
func createMockBody(fileBody []byte, bodyReadError error) *MockReadCloser {
	if bodyReadError != nil {
		return &MockReadCloser{
			Reader: io.NopCloser(errorReader{err: bodyReadError}),
			CloseFunc: func() error {
				return nil
			},
		}
	}

	return &MockReadCloser{Reader: bytes.NewReader(fileBody)}
}

// assertErrorCase validates that ReadAll returns an error as expected.
func assertErrorCase(t *testing.T, err error, reader file.RowReader, _ string) {
	t.Helper()
	require.Error(t, err, "ReadAll() expected an error, but got nil")
	assert.Nil(t, reader, "ReadAll() expected nil reader on error")
}

// assertSuccessCase validates that ReadAll succeeds and returns the correct reader type.
func assertSuccessCase(t *testing.T, err error, reader file.RowReader, expectedType string) {
	t.Helper()
	require.NoError(t, err, "ReadAll() unexpected error")
	require.NotNil(t, reader, "ReadAll() returned nil reader on success")

	switch expectedType {
	case "json-array", "json-object":
		assert.IsType(t, &jsonReader{}, reader, "ReadAll() for JSON file expected *jsonReader")
	case "text":
		assert.IsType(t, &textReader{}, reader, "ReadAll() for text file expected *textReader")
	}
}

func TestS3File_ReadAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// A sample error for simulating I/O failures.
	readError := ErrReadAllFailed

	// Helper function for creating a new S3File instance for a test
	newS3File := func(name string, body io.ReadCloser) *S3File {
		f := newTestS3File(t, ctrl, name, 0, 0)
		f.body = body

		return f
	}

	// --- Test Cases ---
	tests := []struct {
		name          string
		fileName      string
		fileBody      []byte
		bodyReadError error
		expectedType  string
	}{
		{
			name:          "JSON Array Success",
			fileName:      "my-bucket/path/to/data.json",
			fileBody:      []byte(`[{"id": 1}, {"id": 2}]`),
			bodyReadError: nil,
			expectedType:  "json-array",
		},
		{
			name:          "JSON Object Success",
			fileName:      "my-bucket/path/to/config.json",
			fileBody:      []byte(`{"key": "value"}`),
			bodyReadError: nil,
			expectedType:  "json-object",
		},
		{
			name:          "Text/CSV Success",
			fileName:      "my-bucket/path/to/data.csv",
			fileBody:      []byte("col1,col2\n1,2"),
			bodyReadError: nil,
			expectedType:  "text",
		},
		{
			name:          "JSON ReadAll Error",
			fileName:      "my-bucket/fail.json",
			fileBody:      nil,
			bodyReadError: readError,
			expectedType:  "error",
		},
		{
			name:          "Text/CSV ReadAll Error",
			fileName:      "my-bucket/fail.txt",
			fileBody:      nil,
			bodyReadError: readError,
			expectedType:  "error",
		},
		{
			name:          "JSON Invalid Token Error",
			fileName:      "my-bucket/invalid.json",
			fileBody:      []byte(`not a json`),
			bodyReadError: nil,
			expectedType:  "json-error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBody := createMockBody(tt.fileBody, tt.bodyReadError)
			f := newS3File(tt.fileName, mockBody)

			reader, err := f.ReadAll()

			if tt.expectedType == "error" || tt.expectedType == "json-error" {
				assertErrorCase(t, err, reader, tt.expectedType)
				return
			}

			assertSuccessCase(t, err, reader, tt.expectedType)
		})
	}
}

// errorReader is a helper to simulate an io.ReadAll failure for testing.
type errorReader struct {
	err error
}

func (er errorReader) Read(_ []byte) (n int, err error) {
	return 0, er.err
}

// MockReadCloser is a minimal mock for the io.ReadCloser field 'f.body'.
type MockReadCloser struct {
	io.Reader
	CloseFunc func() error
}

func (m *MockReadCloser) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}

	return nil
}

func TestFileSystem_Connect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	successConfig := &Config{
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "AKIA_SUCCESS",
		SecretAccessKey: "SECRET_SUCCESS",
		EndPoint:        "http://localhost:9000",
	}

	// --- Test Case: Successful Connection ---
	t.Run("SuccessCase", func(t *testing.T) {
		mockLogger := NewMockLogger(ctrl)
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()

		fs := &FileSystem{
			config: successConfig,
			logger: mockLogger,
		}

		// Execute the function under test
		fs.Connect()

		assert.NotNil(t, fs.conn, "Connect() failed to initialize S3 client")
	})
}
