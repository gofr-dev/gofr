package file

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// recordingLogger keeps the OperationLog of every Debug call.
type recordingLogger struct {
	mu   sync.Mutex
	logs []*OperationLog
}

func (l *recordingLogger) Debug(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, a := range args {
		if ol, ok := a.(*OperationLog); ok {
			l.logs = append(l.logs, ol)
		}
	}
}

func (*recordingLogger) Debugf(string, ...any) {}
func (*recordingLogger) Info(...any)           {}
func (*recordingLogger) Infof(string, ...any)  {}
func (*recordingLogger) Error(...any)          {}
func (*recordingLogger) Errorf(string, ...any) {}
func (*recordingLogger) Warn(...any)           {}
func (*recordingLogger) Warnf(string, ...any)  {}

// statuses returns "OPERATION STATUS" for every recorded log.
func (l *recordingLogger) statuses() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]string, 0, len(l.logs))
	for _, ol := range l.logs {
		out = append(out, ol.Operation+" "+*ol.Status)
	}

	return out
}

func TestLocalFile_ObservesEachOperation(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "data.txt", "abc")

	logger := &recordingLogger{}
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	metrics.EXPECT().NewHistogram(AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()
	metrics.EXPECT().RecordHistogram(gomock.Any(), AppFileStats, gomock.Any(),
		"type", OpRead, "status", "SUCCESS", "provider", "FILE").Times(2)
	metrics.EXPECT().RecordHistogram(gomock.Any(), AppFileStats, gomock.Any(), gomock.Any()).AnyTimes()

	fs := NewLocalFileSystem(logger)
	fs.UseMetrics(metrics)

	f, err := fs.Open("data.txt")
	require.NoError(t, err)

	_, err = io.ReadAll(f) // one read of the data, one at EOF: both succeed
	require.NoError(t, err)

	_, err = f.Write([]byte("x")) // opened read-only
	require.Error(t, err)

	require.NoError(t, f.Close())

	assert.Equal(t, []string{
		"OPEN SUCCESS",
		"READ SUCCESS",
		"READ SUCCESS",
		"WRITE ERROR",
		"CLOSE SUCCESS",
	}, logger.statuses())
}

func TestLocalFile_FileInfo(t *testing.T) {
	t.Chdir(t.TempDir())

	modTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	writeTestFile(t, "data.txt", "hello")
	require.NoError(t, os.Chmod("data.txt", 0o640))
	require.NoError(t, os.Chtimes("data.txt", modTime, modTime))
	require.NoError(t, os.Mkdir("dir", 0o755))

	fs := newTestLocalFS(t)

	f, err := fs.Open("data.txt")
	require.NoError(t, err)

	assertInfo := func(t *testing.T) {
		t.Helper()

		assert.Equal(t, "data.txt", f.Name())
		assert.Equal(t, int64(5), f.Size())
		assert.True(t, f.ModTime().Equal(modTime))
		assert.False(t, f.IsDir())
		assert.Equal(t, os.FileMode(0o640), f.Mode())
		assert.NotNil(t, f.Sys())
	}

	assertInfo(t)

	// After Close the metadata saved at Close is returned.
	require.NoError(t, f.Close())
	assertInfo(t)

	d, err := fs.Open("dir")
	require.NoError(t, err)
	assert.True(t, d.IsDir())
	assert.True(t, d.Mode().IsDir())
	require.NoError(t, d.Close())
}

func TestLocalFile_FileInfo_WithoutMetadata(t *testing.T) {
	osFile, err := os.Create(filepath.Join(t.TempDir(), "f.txt"))
	require.NoError(t, err)
	require.NoError(t, osFile.Close())

	// The file could not be stat'ed when it was opened, nor since.
	f := newLocalFile(osFile, nil, nil, localLocation)

	assert.Zero(t, f.Size())
	assert.True(t, f.ModTime().IsZero())
	assert.False(t, f.IsDir())
	assert.Zero(t, f.Mode())
	assert.Nil(t, f.Sys())
}

func TestLocalFile_ReadAll(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "users.json", `[{"name":"a"},{"name":"b"}]`)
	writeTestFile(t, "users.csv", "a\nb\n")

	fs := newTestLocalFS(t)

	jsonFile, err := fs.Open("users.json")
	require.NoError(t, err)

	reader, err := jsonFile.ReadAll()
	require.NoError(t, err)

	var names []string

	for reader.Next() {
		var u struct{ Name string }

		require.NoError(t, reader.Scan(&u))

		names = append(names, u.Name)
	}

	assert.Equal(t, []string{"a", "b"}, names)
	require.NoError(t, jsonFile.Close())

	csvFile, err := fs.Open("users.csv")
	require.NoError(t, err)

	reader, err = csvFile.ReadAll()
	require.NoError(t, err)

	var lines []string

	for reader.Next() {
		var line string

		require.NoError(t, reader.Scan(&line))

		lines = append(lines, line)
	}

	assert.Equal(t, []string{"a", "b"}, lines)
	require.NoError(t, csvFile.Close())
}

func TestLocalFile_MetadataAfterWritesAndClose(t *testing.T) {
	t.Chdir(t.TempDir())

	f, err := newTestLocalFS(t).Create("out.bin")
	require.NoError(t, err)

	_, err = f.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// The size written before Close is kept, not the empty size at Create.
	assert.Equal(t, int64(11), f.Size())
}

func TestLocalFile_ConcurrentUse(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "data.txt", "hello world")

	f, err := newTestLocalFS(t).Open("data.txt")
	require.NoError(t, err)

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			buf := make([]byte, 5)

			_, _ = f.ReadAt(buf, 0)
			_ = f.Size()
			_ = f.Mode()
		})
	}

	wg.Go(func() { _ = f.Close() })

	wg.Wait()

	assert.Equal(t, int64(11), f.Size())
}

func TestLocalFile_ReadAt(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "data.txt", "0123456789")

	f, err := newTestLocalFS(t).Open("data.txt")
	require.NoError(t, err)

	t.Cleanup(func() { _ = f.Close() })

	tests := []struct {
		desc    string
		off     int64
		want    string
		wantErr error
	}{
		{desc: "inside the file", off: 2, want: "2345"},
		{desc: "running into the end", off: 8, want: "89", wantErr: io.EOF},
		{desc: "at the end", off: 10, wantErr: io.EOF},
		{desc: "past the end", off: 11, wantErr: ErrOutOfRange},
		{desc: "negative offset", off: -1, wantErr: ErrOutOfRange},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			buf := make([]byte, 4)

			n, readErr := f.ReadAt(buf, tc.off)
			assert.Equal(t, tc.want, string(buf[:n]))

			if tc.wantErr == nil {
				require.NoError(t, readErr)

				return
			}

			require.ErrorIs(t, readErr, tc.wantErr)
		})
	}

	// io.SectionReader relies on io.EOF at the end of the file.
	tail, err := io.ReadAll(io.NewSectionReader(f, 8, 4))
	require.NoError(t, err)
	assert.Equal(t, "89", string(tail))
}

func TestLocalFile_Seek(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "data.txt", "0123456789")

	f, err := newTestLocalFS(t).OpenFile("data.txt", os.O_RDWR, 0)
	require.NoError(t, err)

	// The offset can move past the end, and a Write there extends the file.
	pos, err := f.Seek(2, io.SeekEnd)
	require.NoError(t, err)
	assert.Equal(t, int64(12), pos)

	_, err = f.Write([]byte("X"))
	require.NoError(t, err)

	invalid := []struct {
		desc   string
		offset int64
		whence int
	}{
		{desc: "negative from the start", offset: -1, whence: io.SeekStart},
		{desc: "before the start from the current offset", offset: -14, whence: io.SeekCurrent},
		{desc: "before the start from the end", offset: -14, whence: io.SeekEnd},
		{desc: "unknown whence", offset: 0, whence: 7},
	}

	for _, tc := range invalid {
		_, err = f.Seek(tc.offset, tc.whence)
		require.ErrorIs(t, err, ErrOutOfRange, tc.desc)
	}

	// A failed seek leaves the offset where it was.
	pos, err = f.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(13), pos)

	require.NoError(t, f.Close())
	assert.Equal(t, "0123456789\x00\x00X", readTestFile(t, "data.txt"))

	// Seeking a closed file is not an offset error, even where the current offset or
	// the size would be needed to tell.
	closedSeeks := []struct {
		offset int64
		whence int
	}{{0, io.SeekStart}, {-1, io.SeekCurrent}, {-1, io.SeekEnd}}

	for _, s := range closedSeeks {
		_, err = f.Seek(s.offset, s.whence)
		require.ErrorIs(t, err, os.ErrClosed)
		require.NotErrorIs(t, err, ErrOutOfRange)
	}
}

func TestLocalFile_CloseTwice(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "data.txt", "abc")

	logger := &recordingLogger{}

	f, err := NewLocalFileSystem(logger).Open("data.txt")
	require.NoError(t, err)

	require.NoError(t, f.Close())
	require.NoError(t, f.Close())

	assert.Equal(t, []string{"OPEN SUCCESS", "CLOSE SUCCESS", "CLOSE SUCCESS"}, logger.statuses())
}
