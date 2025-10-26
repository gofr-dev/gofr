package gcs

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	errStat        = fmt.Errorf("stat error")
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
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

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
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
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
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

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

	mockLogger.EXPECT().Debug("GCS file body is nil")
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
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

func (*mockReader) Close() error {
	return nil
}

type fakeStorageReader struct {
	io.Reader
}

func (*fakeStorageReader) Close() error { return nil }

func TestFile_Seek(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := getObjectName(filePath)
	attrs := &storage.ObjectAttrs{Size: 20}

	mockClient.EXPECT().
		StatObject(gomock.Any(), filePath).
		Return(attrs, nil)

	mockClient.EXPECT().
		NewRangeReader(gomock.Any(), objectName, int64(5), int64(-1)).
		Return(&fakeStorageReader{Reader: strings.NewReader("data starting from offset 5")}, nil)

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(), "type", gomock.Any(), "status", gomock.Any(),
	).AnyTimes()

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

	mockClient := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	expectedErr := errStat

	mockClient.EXPECT().
		StatObject(gomock.Any(), filePath).
		Return(nil, expectedErr)

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(), "type", gomock.Any(), "status", gomock.Any(),
	).AnyTimes()
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

	mockClient := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	attrs := &storage.ObjectAttrs{Size: 10}

	mockClient.EXPECT().
		StatObject(gomock.Any(), filePath).
		Return(attrs, nil)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(1)

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(), "type", gomock.Any(), "status", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	_, err := f.Seek(0, 999)
	require.Error(t, err)
	require.ErrorIs(t, err, errOffesetOutOfRange)
}
func TestFile_Seek_NewRangeReaderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := getObjectName(filePath)
	attrs := &storage.ObjectAttrs{Size: 10}

	mockClient.EXPECT().
		StatObject(gomock.Any(), filePath).
		Return(attrs, nil)

	mockClient.EXPECT().
		NewRangeReader(gomock.Any(), objectName, int64(0), int64(-1)).
		Return(nil, errRangeReader)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(1)

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(), "type", gomock.Any(), "status", gomock.Any(),
	).AnyTimes()

	f := &File{
		conn:    mockClient,
		name:    filePath,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	_, err := f.Seek(0, io.SeekStart)
	require.Error(t, err)
	require.Contains(t, err.Error(), "range reader error")
}

func TestFile_ReadAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := getObjectName(filePath)
	expected := "abcd"
	buf := make([]byte, len(expected))
	mockClient.EXPECT().
		NewRangeReader(gomock.Any(), objectName, int64(1), int64(len(buf))).
		Return(&fakeStorageReader{Reader: strings.NewReader(expected)}, nil)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

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

type mockWriteCloser struct {
	buf  []byte
	save func([]byte)
}

func (w *mockWriteCloser) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *mockWriteCloser) Close() error {
	if w.save != nil {
		w.save(w.buf)
	}

	return nil
}
func TestFile_WriteAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	filePath := "bucket/file.txt"
	objectName := getObjectName(filePath)
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
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

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
