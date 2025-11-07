package file

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalProvider_NewReaderAndNewRangeReader_LimitedAndFull(t *testing.T) {
	provider := &localProvider{}

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "data.bin")
	err := os.WriteFile(filePath, []byte("abcdefgh"), 0o600)
	require.NoError(t, err)

	// NewReader reads full file
	r, err := provider.NewReader(t.Context(), filePath)

	require.NoError(t, err)
	all, err := io.ReadAll(r)

	require.NoError(t, err)
	require.NoError(t, r.Close())
	assert.Equal(t, []byte("abcdefgh"), all)

	// NewRangeReader with offset and length > 0 returns limitedReadCloser
	rr, err := provider.NewRangeReader(t.Context(), filePath, 2, 4)

	require.NoError(t, err)

	data, err := io.ReadAll(rr)

	require.NoError(t, err)
	require.NoError(t, rr.Close())
	assert.Equal(t, []byte("cdef"), data)

	// NewRangeReader with length <= 0 returns underlying file starting at offset
	rr2, err := provider.NewRangeReader(t.Context(), filePath, 5, -1)

	require.NoError(t, err)

	rest, err := io.ReadAll(rr2)

	require.NoError(t, err)
	require.NoError(t, rr2.Close())
	assert.Equal(t, []byte("fgh"), rest)
}

func TestLocalProvider_NewWriter_CreateAndFailWriter(t *testing.T) {
	provider := &localProvider{}

	tmp := t.TempDir()

	// Successful writer: write file and verify contents
	dst := filepath.Join(tmp, "out", "file.txt")

	w := provider.NewWriter(t.Context(), dst)

	n, err := w.Write([]byte("hello"))

	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.NoError(t, w.Close())

	content, err := os.ReadFile(dst)

	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), content)

	// Simulate MkdirAll failure by making parent path a file
	parentFile := filepath.Join(tmp, "parentfile")
	err = os.WriteFile(parentFile, []byte("I am a file"), 0o600)

	require.NoError(t, err)

	badPath := filepath.Join(parentFile, "child") // filepath.Dir(badPath) == parentFile (a file)
	w2 := provider.NewWriter(t.Context(), badPath)

	// Expect a failWriter (Write/Close return error)
	_, w2Err := w2.Write([]byte("x"))

	require.Error(t, w2Err)
	require.Error(t, w2.Close())
}

func TestLocalProvider_Stat_Delete_Copy_List_ListDir(t *testing.T) {
	provider := &localProvider{}
	tmp := t.TempDir()

	// create files and directories
	err := os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755)
	require.NoError(t, err)

	aPath := filepath.Join(tmp, "a.txt")
	bPath := filepath.Join(tmp, "subdir", "b.txt")

	err = os.WriteFile(aPath, []byte("A"), 0o600)
	require.NoError(t, err)

	err = os.WriteFile(bPath, []byte("B"), 0o600)
	require.NoError(t, err)

	// StatObject on file
	info, err := provider.StatObject(t.Context(), aPath)

	require.NoError(t, err)
	assert.Equal(t, "a.txt", info.Name)
	assert.False(t, info.IsDir)
	assert.Equal(t, int64(1), info.Size)

	// DeleteObject removes file
	err = provider.DeleteObject(t.Context(), aPath)
	require.NoError(t, err)

	_, statErr := os.Stat(aPath)
	require.Error(t, statErr)

	// CopyObject copies file to nested dst (creating dirs)
	src := bPath
	dst := filepath.Join(tmp, "nested", "copied.txt")

	err = provider.CopyObject(t.Context(), src, dst)
	require.NoError(t, err)

	copied, err := os.ReadFile(dst)

	require.NoError(t, err)
	assert.Equal(t, []byte("B"), copied)

	// ListObjects returns file names only (not directories)
	objs, err := provider.ListObjects(t.Context(), tmp)
	require.NoError(t, err)
	// Ensure entries do not refer to directories
	for _, name := range objs {
		info, sErr := os.Stat(filepath.Join(tmp, name))

		require.NoError(t, sErr)
		assert.False(t, info.IsDir())
	}

	// ListDir returns object infos and trailing-slash dirs
	objsInfo, dirs, err := provider.ListDir(t.Context(), tmp)
	require.NoError(t, err)

	// Validate dirs contain "subdir/" or "nested/"
	foundDir := false

	for _, d := range dirs {
		if d == "subdir/" || d == "nested/" {
			foundDir = true
		}
	}

	assert.True(t, foundDir)

	// Validate any returned object info entries have non-empty names (it's valid for objsInfo to be empty)
	for _, oi := range objsInfo {
		assert.NotEmpty(t, oi.Name)
	}
}

func TestLimitedReadCloser_ReadsTillLimitAndEOF(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "limit.txt")

	err := os.WriteFile(path, []byte("1234567890"), 0o600)

	require.NoError(t, err)

	f, err := os.Open(path)

	require.NoError(t, err)

	lrc := &limitedReadCloser{rc: f, remaining: 4}

	buf := make([]byte, 10)
	n, err := lrc.Read(buf)

	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte("1234"), buf[:4])

	n2, err2 := lrc.Read(buf)

	assert.Equal(t, 0, n2)
	assert.Equal(t, io.EOF, err2)
	require.NoError(t, lrc.Close())
}

func TestFailWriter_WriteAndCloseReturnError(t *testing.T) {
	errExample := os.ErrPermission

	fw := &failWriter{err: errExample}
	_, wErr := fw.Write([]byte("x"))

	require.Error(t, wErr)
	require.ErrorIs(t, wErr, errExample)
	require.Error(t, fw.Close())
	require.ErrorIs(t, fw.Close(), errExample)
}
