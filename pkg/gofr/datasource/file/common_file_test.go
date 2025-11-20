package file

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCommonFile_Name(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
	}{
		{
			name:     "simple file name",
			fileName: "test.txt",
		},
		{
			name:     "file with path",
			fileName: "path/to/file.txt",
		},
		{
			name:     "empty name",
			fileName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{name: tt.fileName}

			assert.Equal(t, tt.fileName, f.Name())
		})
	}
}

func TestCommonFile_Size(t *testing.T) {
	tests := []struct {
		name         string
		size         int64
		expectedSize int64
	}{
		{
			name:         "zero size",
			size:         0,
			expectedSize: 0,
		},
		{
			name:         "small file",
			size:         1024,
			expectedSize: 1024,
		},
		{
			name:         "large file",
			size:         1073741824, // 1GB
			expectedSize: 1073741824,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{size: tt.size}

			assert.Equal(t, tt.expectedSize, f.Size())
		})
	}
}

func TestCommonFile_ModTime(t *testing.T) {
	now := time.Now()
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name         string
		lastModified time.Time
	}{
		{
			name:         "current time",
			lastModified: now,
		},
		{
			name:         "past time",
			lastModified: past,
		},
		{
			name:         "zero time",
			lastModified: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{lastModified: tt.lastModified}

			assert.Equal(t, tt.lastModified, f.ModTime())
		})
	}
}

func TestCommonFile_IsDir(t *testing.T) {
	tests := []struct {
		name        string
		isDir       bool
		contentType string
		expected    bool
	}{
		{
			name:     "explicit directory flag",
			isDir:    true,
			expected: true,
		},
		{
			name:     "explicit file flag",
			isDir:    false,
			expected: false,
		},
		{
			name:        "directory content type",
			isDir:       false,
			contentType: "application/x-directory",
			expected:    true,
		},
		{
			name:        "file content type",
			isDir:       false,
			contentType: "text/plain",
			expected:    false,
		},
		{
			name:        "both directory indicators",
			isDir:       true,
			contentType: "application/x-directory",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{
				isDir:       tt.isDir,
				contentType: tt.contentType,
			}

			assert.Equal(t, tt.expected, f.IsDir())
		})
	}
}

func TestCommonFile_Mode(t *testing.T) {
	tests := []struct {
		name         string
		isDir        bool
		contentType  string
		expectedMode fs.FileMode
	}{
		{
			name:         "directory mode",
			isDir:        true,
			expectedMode: DefaultDirMode,
		},
		{
			name:         "file mode",
			isDir:        false,
			expectedMode: DefaultFileMode,
		},
		{
			name:         "directory by content type",
			contentType:  "application/x-directory",
			expectedMode: DefaultFileMode,
		},
		{
			name:         "regular file content type",
			contentType:  "text/plain",
			expectedMode: DefaultFileMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CommonFile{
				isDir:       tt.isDir,
				contentType: tt.contentType,
			}

			assert.Equal(t, tt.expectedMode, f.Mode())
		})
	}
}

func TestCommonFile_Sys(t *testing.T) {
	f := &CommonFile{
		name:  "test.txt",
		size:  100,
		isDir: false,
	}

	result := f.Sys()

	assert.Nil(t, result, "Sys() should always return nil for cloud storage")
}

func TestCommonFile_CompleteFileInfo(t *testing.T) {
	now := time.Now()

	f := &CommonFile{
		name:         "document.pdf",
		size:         2048,
		contentType:  "application/pdf",
		lastModified: now,
		isDir:        false,
	}

	assert.Equal(t, "document.pdf", f.Name())
	assert.Equal(t, int64(2048), f.Size())
	assert.Equal(t, now, f.ModTime())
	assert.False(t, f.IsDir())
	assert.Equal(t, DefaultFileMode, f.Mode())
	assert.Nil(t, f.Sys())
}

func TestCommonFile_CompleteDirectoryInfo(t *testing.T) {
	now := time.Now()

	f := &CommonFile{
		name:         "mydir",
		size:         0,
		contentType:  "application/x-directory",
		lastModified: now,
		isDir:        true,
	}

	assert.Equal(t, "mydir", f.Name())
	assert.Equal(t, int64(0), f.Size())
	assert.Equal(t, now, f.ModTime())
	assert.True(t, f.IsDir())
	assert.Equal(t, DefaultDirMode, f.Mode())
	assert.Nil(t, f.Sys())
}

func TestCommonFile_Read_SuccessAndNoBody(t *testing.T) {
	f := &CommonFile{body: io.NopCloser(strings.NewReader("hello")), name: "r.txt"}
	buf := make([]byte, 10)
	n, err := f.Read(buf)
	assert.Equal(t, 5, n)
	require.NoError(t, err)

	// No body
	f2 := &CommonFile{name: "no-body"}
	n2, err2 := f2.Read(buf)
	assert.Equal(t, 0, n2)
	assert.Equal(t, errFileNotOpenForReading, err2)
}

func TestCommonFile_ReadAt_Various(t *testing.T) {
	ctrl, mockProvider, _ := setupCommonFS(t)
	defer ctrl.Finish()

	// Success: provider.NewRangeReader returns a reader that has enough bytes.
	f := &CommonFile{
		provider: mockProvider,
		name:     "file.bin",
		size:     100,
	}

	p := make([]byte, 4)

	mockProvider.EXPECT().
		NewRangeReader(gomock.Any(), "file.bin", int64(2), int64(len(p))).
		Return(io.NopCloser(bytes.NewReader([]byte("abcdefgh"))), nil)

	n, err := f.ReadAt(p, 2)

	assert.Equal(t, 4, n)
	require.NoError(t, err)

	// Offset out of range
	f2 := &CommonFile{size: 5, name: "oob.bin"}
	_, err2 := f2.ReadAt(make([]byte, 2), 5)

	assert.Equal(t, ErrOutOfRange, err2)

	// NewRangeReader returns error.
	f3 := &CommonFile{
		provider: mockProvider,
		name:     "err.bin",
		size:     10,
	}

	mockProvider.EXPECT().
		NewRangeReader(gomock.Any(), "err.bin", int64(1), int64(2)).
		Return(nil, errTest)

	_, err3 := f3.ReadAt(make([]byte, 2), 1)

	assert.Equal(t, errTest, err3)
}

func TestCommonFile_Seek_SuccessAndProviderError(t *testing.T) {
	ctrl, mockProvider, _ := setupCommonFS(t)
	defer ctrl.Finish()

	// Success: seek to start.
	reader := io.NopCloser(strings.NewReader("content"))

	mockProvider.EXPECT().
		NewRangeReader(gomock.Any(), "seekfile", int64(0), int64(-1)).
		Return(reader, nil)

	f := &CommonFile{
		provider:   mockProvider,
		name:       "seekfile",
		currentPos: 5,
		size:       100,
	}

	pos, err := f.Seek(0, io.SeekStart)

	require.NoError(t, err)
	assert.Equal(t, int64(0), pos)
	assert.Equal(t, int64(0), f.currentPos)

	// Provider NewRangeReader error.
	mockProvider.EXPECT().
		NewRangeReader(gomock.Any(), "badseek", int64(1), int64(-1)).
		Return(nil, errTest)

	f2 := &CommonFile{
		provider:   mockProvider,
		name:       "badseek",
		currentPos: 0,
		size:       10,
	}

	_, err2 := f2.Seek(1, io.SeekStart)

	assert.Equal(t, errTest, err2)
}

func TestCommonFile_ReadAll_NoBody_JSON_Text(t *testing.T) {
	// No body -> error.
	f := &CommonFile{name: "no", body: nil}
	_, err := f.ReadAll()

	assert.Equal(t, errFileNotOpenForReading, err)

	// JSON path.
	bodyJSON := io.NopCloser(strings.NewReader(`[{"a":1}]`))
	fj := &CommonFile{name: "data.json", body: bodyJSON, logger: nil}
	rr, errj := fj.ReadAll()

	require.NoError(t, errj)
	assert.NotNil(t, rr)

	// Text/CSV path.
	bodyTxt := io.NopCloser(strings.NewReader("a,b\n1,2\n"))
	ft := &CommonFile{name: "data.txt", body: bodyTxt, logger: nil}
	r2, errt := ft.ReadAll()

	require.NoError(t, errt)
	assert.NotNil(t, r2)
}

// simple in-memory WriteCloser used by tests.
type bufWriteCloser struct {
	bytes.Buffer
}

func (*bufWriteCloser) Close() error { return nil }

// ReadCloser whose Close returns an error (used to test Close error accumulation).
type badReadCloser struct{}

func (badReadCloser) Read([]byte) (int, error) { return 0, io.EOF }
func (badReadCloser) Close() error             { return errTest }

// WriteCloser whose Close returns an error (used to test Close error accumulation).
type badWriteCloser struct{}

func (badWriteCloser) Write([]byte) (int, error) { return 0, nil }
func (badWriteCloser) Close() error              { return errTest }

func TestCommonFile_Write_SuccessAndNoWriter(t *testing.T) {
	// Success with simple in-memory WriteCloser.
	bw := &bufWriteCloser{}
	f := &CommonFile{writer: bw, name: "w.txt"}

	n, err := f.Write([]byte("abc"))

	require.NoError(t, err)
	assert.Equal(t, 3, n)

	// No writer
	f2 := &CommonFile{name: "nopen"}
	_, err2 := f2.Write([]byte("x"))

	assert.Equal(t, errFileNotOpenForWriting, err2)
}

func TestCommonFile_WriteAt_LocalAndUnsupported(t *testing.T) {
	f := &CommonFile{writer: &bufWriteCloser{}}
	_, err := f.WriteAt([]byte("x"), 0)

	assert.Equal(t, errWriteAtNotSupported, err)

	tmp, err := os.CreateTemp(t.TempDir(), "writetest_*")

	require.NoError(t, err)

	defer os.Remove(tmp.Name())
	defer tmp.Close()

	f2 := &CommonFile{writer: tmp, name: "local"}
	n, err2 := f2.WriteAt([]byte("xyz"), 0)

	require.NoError(t, err2)
	assert.Equal(t, 3, n)
}

func TestCommonFile_Close_SuccessAndErrors(t *testing.T) {
	// Success case: both close fine.
	body := io.NopCloser(bytes.NewReader([]byte("x")))
	w := &bufWriteCloser{}

	f := &CommonFile{body: body, writer: w, name: "cfile"}
	err := f.Close()

	require.NoError(t, err)
	assert.Nil(t, f.body)
	assert.Nil(t, f.writer)

	// Both Close return errors -> returned error should wrap errTest
	f2 := &CommonFile{
		body:   badReadCloser{},
		writer: badWriteCloser{},
		name:   "cerr",
	}

	err2 := f2.Close()
	require.Error(t, err2)
	assert.ErrorIs(t, err2, errTest)
}
