package gcs

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	errStat        = fmt.Errorf("stat error")
	errRead        = fmt.Errorf("read error")
	errRangeReader = fmt.Errorf("range reader error")
)

func TestFile_Write(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	fakeWriter := &fakeStorageWriter{written: 0}

	f := &File{
		writer:  fakeWriter,
		name:    "test.txt",
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	data := []byte("hello")
	n, err := f.Write(data)

	require.NoError(t, err)
	require.Equal(t, len(data), n)
	require.Equal(t, len(data), fakeWriter.written)
}

func TestFile_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any(),
	).AnyTimes()

	t.Run("close writer", func(t *testing.T) {
		fakeWriter := &fakeStorageWriter{}
		f := &File{
			writer:  fakeWriter,
			body:    nil,
			logger:  mockLogger,
			metrics: mockMetrics,
			name:    "test.txt",
		}

		err := f.Close()
		require.NoError(t, err)
	})

	t.Run("close reader", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader("data"))
		f := &File{
			writer:  nil,
			body:    body,
			logger:  mockLogger,
			metrics: mockMetrics,
			name:    "test.txt",
		}

		err := f.Close()
		require.NoError(t, err)
	})

	t.Run("close nil", func(t *testing.T) {
		f := &File{
			writer:  nil,
			body:    nil,
			logger:  mockLogger,
			metrics: mockMetrics,
			name:    "test.txt",
		}

		err := f.Close()
		require.NoError(t, err)
	})
}

type fakeStorageWriter struct {
	written int
}

func (w *fakeStorageWriter) Write(p []byte) (int, error) {
	written := len(p)
	w.written += written

	return written, nil
}

func (*fakeStorageWriter) Close() error {
	return nil
}

func (*fakeStorageWriter) Error() error {
	return nil
}

func TestFile_Read_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "data.txt",
		body:    io.NopCloser(&mockReader{data: "hello"}),
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	buf := make([]byte, 5)
	n, err := f.Read(buf)

	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "hello", string(buf))
}

func TestFile_Read_Error_NilBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Error("GCS file body is nil")
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "missing.txt",
		body:    nil,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	buf := make([]byte, 5)
	n, err := f.Read(buf)

	require.Error(t, err)
	require.Equal(t, 0, n)
	require.ErrorIs(t, err, errNilGCSFileBody)
}

type mockReader struct {
	data string
}

func (m *mockReader) Read(p []byte) (int, error) {
	if m.data == "" {
		return 0, io.EOF
	}

	n := copy(p, m.data)
	m.data = m.data[n:]

	return n, nil
}

func TestFile_Seek(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)

	attrs := &file.ObjectInfo{
		Name:         filePath,
		Size:         20,
		ContentType:  "text/plain",
		LastModified: time.Now(),
	}

	mockClient.EXPECT().
		StatObject(gomock.Any(), filePath).
		Return(attrs, nil)

	mockClient.EXPECT().
		NewRangeReader(gomock.Any(), objectName, int64(5), int64(-1)).
		Return(&fakeStorageReader{Reader: strings.NewReader("data starting from offset 5")}, nil)

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    "bucket/file.txt",
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	pos, err := f.Seek(5, io.SeekStart)
	require.NoError(t, err)
	require.Equal(t, int64(5), pos)
	require.NotNil(t, f.body)
}

func TestFile_Seek_StatObjectError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	expectedErr := errStat

	mockClient.EXPECT().
		StatObject(gomock.Any(), filePath).
		Return(nil, expectedErr)

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any()).Times(1)

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	_, err := f.Seek(0, io.SeekStart)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stat error")
}

func TestFile_Seek_CheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	attrs := &file.ObjectInfo{
		Name: filePath,
		Size: 10,
	}

	mockClient.EXPECT().
		StatObject(gomock.Any(), filePath).
		Return(attrs, nil)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(1)

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	_, err := f.Seek(0, 999)
	require.Error(t, err)
	require.ErrorIs(t, err, file.ErrOutOfRange)
}

func TestFile_Seek_NewRangeReaderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	attrs := &file.ObjectInfo{
		Name: filePath,
		Size: 10,
	}

	mockClient.EXPECT().
		StatObject(gomock.Any(), filePath).
		Return(attrs, nil)

	mockClient.EXPECT().
		NewRangeReader(gomock.Any(), objectName, int64(0), int64(-1)).
		Return(nil, errStat)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(1)

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	_, err := f.Seek(0, io.SeekStart)
	require.Error(t, err)
	require.Contains(t, err.Error(), errStat.Error())
}

func TestFile_ReadAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	expected := "abcd"
	buf := make([]byte, len(expected))
	mockClient.EXPECT().
		NewRangeReader(gomock.Any(), objectName, int64(1), int64(len(buf))).
		Return(&fakeStorageReader{Reader: strings.NewReader(expected)}, nil)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    "bucket/file.txt",
		logger:  mockLogger,
		metrics: mockMetrics,
	}
	n, err := f.ReadAt(buf, 1)
	require.NoError(t, err)
	require.Equal(t, len(expected), n)
	require.Equal(t, []byte(expected), buf)
}

func TestFile_WriteAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	initialContent := "abcdef"
	mockClient.EXPECT().
		NewReader(gomock.Any(), objectName).
		Return(&fakeStorageReader{Reader: strings.NewReader(initialContent)}, nil)

	var written []byte
	mockClient.EXPECT().
		NewWriter(gomock.Any(), objectName).
		Return(&mockWriteCloser{save: func(p []byte) { written = append(written, p...) }})

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    "bucket/file.txt",
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	overwrite := []byte("ZZ")
	n, err := f.WriteAt(overwrite, 2)
	require.NoError(t, err)
	require.Equal(t, len(overwrite), n)
	require.Equal(t, "abZZef", string(written))
}

func TestFile_ReadAll_Success_JSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	jsonData := `{"name":"test","value":123}`
	body := io.NopCloser(strings.NewReader(jsonData))

	f := &File{
		name:    "data.json",
		body:    body,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.ReadAll()

	require.NoError(t, err)
	require.NotNil(t, reader)
}

func TestFile_ReadAll_Success_Text(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	textData := "line1\nline2\nline3"
	body := io.NopCloser(strings.NewReader(textData))

	f := &File{
		name:    "data.txt",
		body:    body,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.ReadAll()

	require.NoError(t, err)
	require.NotNil(t, reader)
}

func TestFile_ReadAll_Error_NilBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Error("file body is nil")
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "data.json",
		body:    nil,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.ReadAll()

	require.Error(t, err)
	require.Nil(t, reader)
	require.ErrorIs(t, err, errNilGCSFileBody)
}

func TestFile_ReadAll_Error_JSONReaderCreation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	// Use a reader that fails immediately
	body := &errorReadCloser{err: errRead}

	f := &File{
		name:    "data.json",
		body:    body,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.ReadAll()

	require.Error(t, err)
	require.Nil(t, reader)
}

func TestGetLocation(t *testing.T) {
	tests := []struct {
		name     string
		bucket   string
		expected string
	}{
		{
			name:     "simple bucket name",
			bucket:   "my-bucket",
			expected: "/my-bucket",
		},
		{
			name:     "bucket with special chars",
			bucket:   "my-bucket-123",
			expected: "/my-bucket-123",
		},
		{
			name:     "empty bucket",
			bucket:   "",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLocation(tt.bucket)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFile_Name(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "file in root",
			filePath: "bucket/file.txt",
			expected: "file.txt",
		},
		{
			name:     "file in nested path",
			filePath: "bucket/folder/subfolder/document.pdf",
			expected: "document.pdf",
		},
		{
			name:     "file with no path",
			filePath: "file.csv",
			expected: "file.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{name: tt.filePath}
			result := f.Name()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFile_Size(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		expected int64
	}{
		{
			name:     "zero size",
			size:     0,
			expected: 0,
		},
		{
			name:     "positive size",
			size:     1024,
			expected: 1024,
		},
		{
			name:     "large size",
			size:     1073741824, // 1GB
			expected: 1073741824,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{size: tt.size}
			result := f.Size()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFile_ModTime(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	tests := []struct {
		name     string
		modTime  time.Time
		expected time.Time
	}{
		{
			name:     "current time",
			modTime:  now,
			expected: now,
		},
		{
			name:     "past time",
			modTime:  yesterday,
			expected: yesterday,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{lastModified: tt.modTime}
			result := f.ModTime()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFile_IsDir(t *testing.T) {
	tests := []struct {
		name        string
		isDir       bool
		contentType string
		expected    bool
	}{
		{
			name:        "directory flag true",
			isDir:       true,
			contentType: "",
			expected:    true,
		},
		{
			name:        "directory content type",
			isDir:       false,
			contentType: "application/x-directory",
			expected:    true,
		},
		{
			name:        "regular file",
			isDir:       false,
			contentType: "text/plain",
			expected:    false,
		},
		{
			name:        "both true",
			isDir:       true,
			contentType: "application/x-directory",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				isDir:       tt.isDir,
				contentType: tt.contentType,
			}
			result := f.IsDir()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFile_Mode(t *testing.T) {
	tests := []struct {
		name        string
		isDir       bool
		contentType string
		expected    os.FileMode
	}{
		{
			name:        "directory mode",
			isDir:       true,
			contentType: "",
			expected:    dirPermissions | os.ModeDir,
		},
		{
			name:        "directory by content type",
			isDir:       false,
			contentType: "application/x-directory",
			expected:    dirPermissions | os.ModeDir,
		},
		{
			name:        "regular file mode",
			isDir:       false,
			contentType: "text/plain",
			expected:    filePermissions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				isDir:       tt.isDir,
				contentType: tt.contentType,
			}
			result := f.Mode()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFile_Sys(t *testing.T) {
	f := &File{}
	result := f.Sys()
	require.Nil(t, result)
}

func TestFile_ReadAt_Error_CreateRangeReader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	expectedErr := errRangeReader

	mockClient.EXPECT().
		NewRangeReader(gomock.Any(), objectName, int64(5), int64(10)).
		Return(nil, expectedErr)

	mockLogger.EXPECT().Errorf("failed to create range reader: %v", expectedErr)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	buf := make([]byte, 10)
	n, err := f.ReadAt(buf, 5)

	require.Error(t, err)
	require.Equal(t, 0, n)
	require.Equal(t, expectedErr, err)
}

func TestFile_ReadAt_Error_ReadFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	expectedErr := errRead

	mockClient.EXPECT().
		NewRangeReader(gomock.Any(), objectName, int64(0), int64(5)).
		Return(&errorReadCloser{err: expectedErr}, nil)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	buf := make([]byte, 5)
	_, err := f.ReadAt(buf, 0)

	require.Error(t, err)
}

func TestFile_WriteAt_Error_NewReaderFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)

	// Return error when trying to read existing data
	mockClient.EXPECT().
		NewReader(gomock.Any(), objectName).
		Return(nil, errRead)

	// Still expect NewWriter to be called since code continues on reader error
	mockClient.EXPECT().
		NewWriter(gomock.Any(), objectName).
		Return(&mockWriteCloser{})

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	data := []byte("new")
	n, err := f.WriteAt(data, 0)

	require.NoError(t, err)
	require.Equal(t, len(data), n)
}

func TestFile_WriteAt_Error_WriteFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	expectedErr := errRangeReader

	mockClient.EXPECT().
		NewReader(gomock.Any(), objectName).
		Return(&fakeStorageReader{Reader: strings.NewReader("old")}, nil)

	mockClient.EXPECT().
		NewWriter(gomock.Any(), objectName).
		Return(&errorWriter{writeErr: expectedErr})

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	data := []byte("new")
	n, err := f.WriteAt(data, 0)

	require.Error(t, err)
	require.Equal(t, 0, n)
	require.Equal(t, expectedErr, err)
}

func TestFile_WriteAt_Error_CloseFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	expectedErr := errStat

	mockClient.EXPECT().
		NewReader(gomock.Any(), objectName).
		Return(&fakeStorageReader{Reader: strings.NewReader("old")}, nil)

	mockClient.EXPECT().
		NewWriter(gomock.Any(), objectName).
		Return(&errorWriter{closeErr: expectedErr})

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	data := []byte("new")
	n, err := f.WriteAt(data, 0)

	require.Error(t, err)
	require.Equal(t, 0, n)
	require.Equal(t, expectedErr, err)
}
func TestFile_WriteAt_PaddingRequired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	initialContent := "abc"

	mockClient.EXPECT().
		NewReader(gomock.Any(), objectName).
		Return(&fakeStorageReader{Reader: strings.NewReader(initialContent)}, nil)

	var written []byte
	mockClient.EXPECT().
		NewWriter(gomock.Any(), objectName).
		Return(&mockWriteCloser{save: func(p []byte) { written = p }})

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	overwrite := []byte("XY")
	n, err := f.WriteAt(overwrite, 10) // Write at offset 10, requires padding

	require.NoError(t, err)
	require.Equal(t, len(overwrite), n)
	require.Len(t, written, 12) // 3 (initial) + 7 (padding) + 2 (new data)
}

func TestFile_WriteAt_ExtendFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockStorageProvider(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := file.GetObjectName(filePath)
	initialContent := "abc"

	mockClient.EXPECT().
		NewReader(gomock.Any(), objectName).
		Return(&fakeStorageReader{Reader: strings.NewReader(initialContent)}, nil)

	var written []byte
	mockClient.EXPECT().
		NewWriter(gomock.Any(), objectName).
		Return(&mockWriteCloser{save: func(p []byte) { written = p }})

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	overwrite := []byte("XYZ123")
	n, err := f.WriteAt(overwrite, 2)

	require.NoError(t, err)
	require.Equal(t, len(overwrite), n)
	require.Equal(t, "abXYZ123", string(written))
}

type errorWriter struct {
	writeErr error
	closeErr error
}

func (e *errorWriter) Write([]byte) (int, error) {
	if e.writeErr != nil {
		return 0, e.writeErr
	}

	return 0, nil
}

func (e *errorWriter) Close() error {
	if e.closeErr != nil {
		return e.closeErr
	}

	return nil
}

func TestFile_Read_Error_ReadFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	expectedErr := errRead

	mockLogger.EXPECT().Errorf("Read failed: read error")
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
		"provider", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "data.txt",
		body:    &errorReadCloser{err: expectedErr},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	buf := make([]byte, 5)
	_, err := f.Read(buf)

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

// Helper types for error testing

type errorReadCloser struct {
	err error
}

func (e *errorReadCloser) Read([]byte) (int, error) {
	return 0, e.err
}

func (*errorReadCloser) Close() error {
	return nil
}
