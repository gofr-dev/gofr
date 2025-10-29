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
)

var (
	errGetObject     = errors.New("failed to get object from S3")
	errPutObject     = errors.New("failed to put object to S3")
	errCloseFailed   = errors.New("close failed")
	errReadAllFailed = errors.New("simulated io.ReadAll error")
	errS3Test        = errors.New("s3 error")
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

// TestS3File_Close_Success tests the successful Close operations of S3File.
func TestS3File_Close_Success(t *testing.T) {
	testCases := []struct {
		name string
		body io.ReadCloser
	}{
		{
			name: "Success_BodyNil",
			body: nil,
		},
		{
			name: "Success_BodyNotNil",
			body: &mockReadCloser{Reader: bytes.NewReader([]byte("test")), closeErr: nil},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, "test-bucket/test-file.txt", 10, 0)
			f.body = tc.body

			err := f.Close()

			assert.NoError(t, err, "Expected no error")
		})
	}
}

// TestS3File_Close_Failure tests the failure cases of S3File Close operations.
func TestS3File_Close_Failure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	f := newTestS3File(t, ctrl, "test-bucket/test-file.txt", 10, 0)
	f.body = &mockReadCloser{Reader: bytes.NewReader([]byte("test")), closeErr: errCloseFailed}

	err := f.Close()

	require.Error(t, err, "Expected an error")
	assert.True(t, errors.Is(err, errCloseFailed) || strings.Contains(err.Error(), errCloseFailed.Error()),
		"Expected error to be %v or contain %q, got %v", errCloseFailed, errCloseFailed.Error(), err)
}

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

			tc.mockGetObject(m.EXPECT())

			p := make([]byte, tc.bufferLen)

			for i := range p {
				p[i] = 0
			}

			n, err := f.Read(p)

			require.NoError(t, err, "Expected no error")
			assert.Equal(t, tc.expectedN, n, "Expected bytes read %d, got %d", tc.expectedN, n)
			assert.Equal(t, tc.expectedP[:n], string(p[:n]), "Expected content %q, got %q", tc.expectedP[:n], string(p[:n]))
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

			tc.mockGetObject(m.EXPECT())

			p := make([]byte, tc.bufferLen)

			for i := range p {
				p[i] = 0
			}

			n, err := f.Read(p)

			require.Error(t, err, "Expected an error")
			require.ErrorIs(t, err, tc.expectedErr, "Expected error %v, got %v", tc.expectedErr, err)
			assert.Equal(t, tc.expectedN, n, "Expected bytes read %d, got %d", tc.expectedN, n)
		})
	}
}

// TestS3File_ReadAt_Success tests the successful ReadAt operations of S3File.
func TestS3File_ReadAt_Success(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	content := "This is a test file content."
	fileSize := int64(len(content))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	f := newTestS3File(t, ctrl, fullPath, fileSize, 10)
	m := f.conn.(*Mocks3Client)

	m.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(content), nil)

	p := make([]byte, 4)
	n, err := f.ReadAt(p, 5)

	require.NoError(t, err, "Expected no error")
	assert.Equal(t, 4, n, "Expected bytes read 4, got %d", n)
	assert.Equal(t, "is a", string(p[:n]), "Expected content %q, got %q", "is a", string(p[:n]))
	assert.Equal(t, int64(10), f.offset, "ReadAt modified offset. Expected 10, got %d", f.offset)
}

// TestS3File_ReadAt_Failure tests the failure cases of S3File ReadAt operations.
func TestS3File_ReadAt_Failure(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	content := "This is a test file content."
	fileSize := int64(len(content))

	testCases := []struct {
		name          string
		readAtOffset  int64
		bufferLen     int
		mockGetObject func(m *Mocks3ClientMockRecorder)
		expectedN     int
		expectedErr   error
	}{
		{
			name:         "Failure_GetObjectError",
			readAtOffset: 0,
			bufferLen:    10,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(nil, errS3Test)
			},
			expectedN:   0,
			expectedErr: errS3Test,
		},
		{
			name:         "Failure_NilBody",
			readAtOffset: 0,
			bufferLen:    10,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{Body: nil}, nil)
			},
			expectedN:   0,
			expectedErr: io.EOF,
		},
		{
			name:         "Failure_OutOfRange",
			readAtOffset: 25,
			bufferLen:    4,
			mockGetObject: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(content), nil)
			},
			expectedN:   0,
			expectedErr: ErrOutOfRange,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, fileSize, 10)
			m := f.conn.(*Mocks3Client)

			tc.mockGetObject(m.EXPECT())

			p := make([]byte, tc.bufferLen)
			n, err := f.ReadAt(p, tc.readAtOffset)

			require.Error(t, err, "Expected an error")
			require.ErrorIs(t, err, tc.expectedErr, "Expected error %v, got %v", tc.expectedErr, err)
			assert.Equal(t, tc.expectedN, n, "Expected bytes read %d, got %d", tc.expectedN, n)
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

			tc.mockExpectations(m.EXPECT())

			n, err := f.Write(tc.dataToWrite)

			require.NoError(t, err, "Expected no error")
			assert.Equal(t, tc.expectedN, n, "Expected bytes written %d, got %d", tc.expectedN, n)
			assert.Equal(t, tc.expectedOffset, f.offset, "Expected offset %d, got %d", tc.expectedOffset, f.offset)
			assert.Equal(t, tc.expectedSize, f.size, "Expected size %d, got %d", tc.expectedSize, f.size)
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

			tc.mockExpectations(m.EXPECT())

			n, err := f.Write(tc.dataToWrite)

			require.Error(t, err, "Expected an error")
			require.ErrorIs(t, err, tc.expectedErr, "Expected error %v, got %v", tc.expectedErr, err)
			assert.Equal(t, tc.expectedN, n, "Expected bytes written %d, got %d", tc.expectedN, n)
			assert.Equal(t, tc.expectedOffset, f.offset, "Expected offset %d, got %d", tc.expectedOffset, f.offset)
			assert.Equal(t, tc.expectedSize, f.size, "Expected size %d, got %d", tc.expectedSize, f.size)
		})
	}
}

// TestS3File_WriteAt_Success tests the successful WriteAt operations of S3File.
func TestS3File_WriteAt_Success(t *testing.T) {
	bucketName, fileName := "test-bucket", "test-file.txt"
	fullPath := bucketName + "/" + fileName
	initialContent := "Hello, World!"
	initialSize, dataToWrite := int64(len(initialContent)), []byte("GoFr")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	f := newTestS3File(t, ctrl, fullPath, initialSize, 10)
	m := f.conn.(*Mocks3Client)

	m.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(initialContent), nil)

	expectedPutBody := []byte("Hello, GoFrd!")

	m.EXPECT().PutObject(gomock.Any(), gomock.Any()).Do(func(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) {
		actualPutBody := getBodyContent(t, params.Body)
		require.True(t, bytes.Equal(expectedPutBody, actualPutBody),
			"PutObject Body mismatch. Expected: %q, Got: %q", string(expectedPutBody), string(actualPutBody))
	}).Return(putObjectOutput(), nil)

	n, err := f.WriteAt(dataToWrite, 7)

	require.NoError(t, err, "Expected no error")
	assert.Equal(t, len(dataToWrite), n, "Expected bytes written %d, got %d", len(dataToWrite), n)
	assert.Equal(t, int64(10), f.offset, "WriteAt modified offset. Expected 10, got %d", f.offset)
	assert.Equal(t, initialSize, f.size, "Expected size %d, got %d", initialSize, f.size)
}

// TestS3File_WriteAt_Failure tests the failure cases of S3File WriteAt operations.
func TestS3File_WriteAt_Failure(t *testing.T) {
	bucketName, fileName := "test-bucket", "test-file.txt"
	fullPath := bucketName + "/" + fileName
	initialContent := "Hello, World!"
	initialSize, dataToWrite := int64(len(initialContent)), []byte("GoFr")

	testCases := []struct {
		name             string
		writeAtOffset    int64
		initialOffset    int64
		mockExpectations func(m *Mocks3ClientMockRecorder)
		expectedN        int
		expectedOffset   int64
		expectedSize     int64
		expectedErr      error
	}{
		{
			name:          "Failure_GetObjectError",
			initialOffset: 10,
			writeAtOffset: 5,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(nil, errGetObject)
			},
			expectedN:      0,
			expectedOffset: 10,
			expectedSize:   initialSize,
			expectedErr:    errGetObject,
		},
		{
			name:          "Failure_PutObjectError",
			initialOffset: 10,
			writeAtOffset: 0,
			mockExpectations: func(m *Mocks3ClientMockRecorder) {
				m.GetObject(gomock.Any(), gomock.Any()).Return(getObjectOutput(initialContent), nil)
				m.PutObject(gomock.Any(), gomock.Any()).Return(nil, errPutObject)
			},
			expectedN:      0,
			expectedOffset: 10,
			expectedSize:   initialSize,
			expectedErr:    errPutObject,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, initialSize, tc.initialOffset)

			m := f.conn.(*Mocks3Client)
			tc.mockExpectations(m.EXPECT())

			n, err := f.WriteAt(dataToWrite, tc.writeAtOffset)

			require.Error(t, err, "Expected an error")
			require.ErrorIs(t, err, tc.expectedErr, "Expected error %v, got %v", tc.expectedErr, err)
			assert.Equal(t, tc.expectedN, n, "Expected bytes written %d, got %d", tc.expectedN, n)
			assert.Equal(t, tc.expectedOffset, f.offset, "WriteAt modified offset. Expected %d, got %d", tc.expectedOffset, f.offset)
			assert.Equal(t, tc.expectedSize, f.size, "Expected size %d, got %d", tc.expectedSize, f.size)
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

// TestS3File_Seek_Success tests the successful Seek operations of S3File.
func TestS3File_Seek_Success(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	fileSize := int64(20)

	testCases := []struct {
		name              string
		initialOffset     int64
		offset            int64
		whence            int
		expectedNewOffset int64
	}{
		{
			name:              "SeekStart_Success",
			initialOffset:     5,
			offset:            10,
			whence:            io.SeekStart,
			expectedNewOffset: 10,
		},
		{
			name:              "SeekCurrent_Success",
			initialOffset:     5,
			offset:            10,
			whence:            io.SeekCurrent,
			expectedNewOffset: 15,
		},
		{
			name:              "SeekEnd_Success",
			initialOffset:     5,
			offset:            -5,
			whence:            io.SeekEnd,
			expectedNewOffset: 15,
		},
		{
			name:              "SeekEnd_Success_ToStart",
			initialOffset:     5,
			offset:            -20,
			whence:            io.SeekEnd,
			expectedNewOffset: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, fileSize, tc.initialOffset)

			newOffset, err := f.Seek(tc.offset, tc.whence)

			require.NoError(t, err, "Expected no error")
			assert.Equal(t, tc.expectedNewOffset, newOffset, "Expected new offset %d, got %d", tc.expectedNewOffset, newOffset)
			assert.Equal(t, tc.expectedNewOffset, f.offset, "File struct offset was not updated. "+
				"Expected %d, got %d", tc.expectedNewOffset, f.offset)
		})
	}
}

// TestS3File_Seek_Failure tests the failure cases of S3File Seek operations.
func TestS3File_Seek_Failure(t *testing.T) {
	bucketName := "test-bucket"
	fileName := "test-file.txt"
	fullPath := bucketName + "/" + fileName
	fileSize := int64(20)

	testCases := []struct {
		name          string
		initialOffset int64
		offset        int64
		whence        int
		expectedErr   error
	}{
		{
			name:          "SeekStart_Failure_Negative",
			initialOffset: 5,
			offset:        -1,
			whence:        io.SeekStart,
			expectedErr:   ErrOutOfRange,
		},
		{
			name:          "SeekStart_Failure_TooLarge",
			initialOffset: 5,
			offset:        21,
			whence:        io.SeekStart,
			expectedErr:   ErrOutOfRange,
		},
		{
			name:          "SeekCurrent_Failure_NegativeResult",
			initialOffset: 5,
			offset:        -6,
			whence:        io.SeekCurrent,
			expectedErr:   ErrOutOfRange,
		},
		{
			name:          "SeekEnd_Failure_TooLarge",
			initialOffset: 5,
			offset:        1,
			whence:        io.SeekEnd,
			expectedErr:   ErrOutOfRange,
		},
		{
			name:          "Seek_InvalidWhence",
			initialOffset: 5,
			offset:        0,
			whence:        3, // Invalid whence
			expectedErr:   os.ErrInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := newTestS3File(t, ctrl, fullPath, fileSize, tc.initialOffset)

			_, err := f.Seek(tc.offset, tc.whence)

			require.Error(t, err, "Expected an error")
			require.ErrorIs(t, err, tc.expectedErr, "Expected error %v, got %v", tc.expectedErr, err)
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
	_, _ = decoder.Token()

	jReader := jsonReader{decoder: decoder}

	require.True(t, jReader.Next(), "Expected Next to be true for the first object")

	var data1 struct {
		Name string
		Age  int
	}
	require.NoError(t, jReader.Scan(&data1), "Scan failed for first object")
	assert.Equal(t, "Alice", data1.Name)
	assert.Equal(t, 30, data1.Age)

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
	_, _ = decoder.Token()

	jReader := jsonReader{decoder: decoder}

	jReader.Next()
	err := jReader.Scan(&struct{}{})
	require.NoError(t, err, "Scan failed for null object")

	require.True(t, jReader.Next(), "Expected Next to be true for the null object")

	var data3 any
	require.NoError(t, jReader.Scan(&data3), "Scan failed for null object")
	assert.Nil(t, data3)

	assert.False(t, jReader.Next(), "Expected Next to be false at the end of the array")

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

	expectedName := "my-file.txt"
	assert.Equal(t, expectedName, f.Name())

	assert.Equal(t, testSize, f.Size())

	assert.Equal(t, testTime, f.ModTime())

	assert.False(t, f.IsDir())

	f.name = bucketName + "/path/to/my-dir/"
	assert.True(t, f.IsDir())

	assert.Equal(t, os.FileMode(0), f.Mode())
}

// createMockBodyWithError creates a MockReadCloser that returns an error when reading.
func createMockBodyWithError(bodyReadError error) *MockReadCloser {
	return &MockReadCloser{
		Reader: io.NopCloser(errorReader{err: bodyReadError}),
		CloseFunc: func() error {
			return nil
		},
	}
}

// createMockBodyWithContent creates a MockReadCloser with the provided content.
func createMockBodyWithContent(fileBody []byte) *MockReadCloser {
	return &MockReadCloser{
		Reader: bytes.NewReader(fileBody),
		CloseFunc: func() error {
			return nil
		},
	}
}

// Helper function for creating a new S3File instance for a test.
func newS3FileForReadAll(t *testing.T, ctrl *gomock.Controller, name string, body io.ReadCloser) *S3File {
	t.Helper()
	f := newTestS3File(t, ctrl, name, 0, 0)
	f.body = body

	return f
}

// TestS3File_ReadAll_JSONArray_Success tests reading JSON array from S3File.
func TestS3File_ReadAll_JSONArray_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBody := createMockBodyWithContent([]byte(`[{"id": 1}, {"id": 2}]`))
	f := newS3FileForReadAll(t, ctrl, "my-bucket/path/to/data.json", mockBody)

	reader, err := f.ReadAll()

	require.NoError(t, err, "ReadAll() unexpected error")
	require.NotNil(t, reader, "ReadAll() returned nil reader on success")
	assert.IsType(t, &jsonReader{}, reader, "ReadAll() for JSON array expected *jsonReader")
}

// TestS3File_ReadAll_JSONObject_Success tests reading JSON object from S3File.
func TestS3File_ReadAll_JSONObject_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBody := createMockBodyWithContent([]byte(`{"key": "value"}`))
	f := newS3FileForReadAll(t, ctrl, "my-bucket/path/to/config.json", mockBody)

	reader, err := f.ReadAll()

	require.NoError(t, err, "ReadAll() unexpected error")
	require.NotNil(t, reader, "ReadAll() returned nil reader on success")
	assert.IsType(t, &jsonReader{}, reader, "ReadAll() for JSON object expected *jsonReader")
}

// TestS3File_ReadAll_Text_Success tests reading text/CSV from S3File.
func TestS3File_ReadAll_Text_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBody := createMockBodyWithContent([]byte("col1,col2\n1,2"))
	f := newS3FileForReadAll(t, ctrl, "my-bucket/path/to/data.csv", mockBody)

	reader, err := f.ReadAll()

	require.NoError(t, err, "ReadAll() unexpected error")
	require.NotNil(t, reader, "ReadAll() returned nil reader on success")
	assert.IsType(t, &textReader{}, reader, "ReadAll() for text file expected *textReader")
}

// TestS3File_ReadAll_JSON_Error tests ReadAll error for JSON file.
func TestS3File_ReadAll_JSON_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBody := createMockBodyWithError(errReadAllFailed)
	f := newS3FileForReadAll(t, ctrl, "my-bucket/fail.json", mockBody)

	reader, err := f.ReadAll()

	require.Error(t, err, "ReadAll() expected an error, but got nil")
	assert.Nil(t, reader, "ReadAll() expected nil reader on error")
}

// TestS3File_ReadAll_Text_Error tests ReadAll error for text/CSV file.
func TestS3File_ReadAll_Text_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBody := createMockBodyWithError(errReadAllFailed)
	f := newS3FileForReadAll(t, ctrl, "my-bucket/fail.txt", mockBody)

	reader, err := f.ReadAll()

	require.Error(t, err, "ReadAll() expected an error, but got nil")
	assert.Nil(t, reader, "ReadAll() expected nil reader on error")
}

// TestS3File_ReadAll_JSONInvalidToken_Error tests ReadAll error for invalid JSON.
func TestS3File_ReadAll_JSONInvalidToken_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBody := createMockBodyWithContent([]byte(`not a json`))
	f := newS3FileForReadAll(t, ctrl, "my-bucket/invalid.json", mockBody)

	reader, err := f.ReadAll()

	require.Error(t, err, "ReadAll() expected an error, but got nil")
	assert.Nil(t, reader, "ReadAll() expected nil reader on error")
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
	return m.CloseFunc()
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

	t.Run("SuccessCase", func(t *testing.T) {
		mockLogger := NewMockLogger(ctrl)
		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()

		fs := &FileSystem{
			config: successConfig,
			logger: mockLogger,
		}

		fs.Connect()

		assert.NotNil(t, fs.conn, "Connect() failed to initialize S3 client")
	})
}
