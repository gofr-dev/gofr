package gcs

import (
	"io"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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

func TestFile_Read(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	tests := []struct {
		name        string
		body        io.ReadCloser
		readData    []byte
		wantN       int
		wantErr     bool
		expectLog   bool
		logContains string
	}{
		{
			name:     "read from valid body",
			body:     io.NopCloser(&mockReader{data: "hello"}),
			readData: make([]byte, 5),
			wantN:    5,
			wantErr:  false,
		},
		{
			name:     "read from nil body",
			body:     nil,
			readData: make([]byte, 5),
			wantN:    0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				body:    tt.body,
				logger:  mockLogger,
				metrics: mockMetrics,
				name:    "test.txt",
			}

			n, err := f.Read(tt.readData)

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, 0, n)
				require.ErrorIs(t, err, errNilGCSFileBody)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantN, n)
			}
		})
	}
}

func TestFile_Seek_ReadAt_WriteAt(t *testing.T) {
	f := &File{}

	_, err := f.Seek(0, 0)
	require.Error(t, err)
	require.ErrorIs(t, err, errSeekNotSupported)

	_, err = f.ReadAt([]byte{}, 0)
	require.Error(t, err)
	require.ErrorIs(t, err, errReadAtNotSupported)

	_, err = f.WriteAt([]byte{}, 0)
	require.Error(t, err)
	require.ErrorIs(t, err, errWriteAtNotSupported)
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
