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
