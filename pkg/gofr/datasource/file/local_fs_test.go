package file

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// newTestLocalFS returns the default local store, as every app gets it in ctx.File.
func newTestLocalFS(t *testing.T) FileSystem {
	t.Helper()

	return NewLocalFileSystem(nil)
}

// writeTestFile creates path with content, creating parent directories.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// readTestFile returns path's content, or "<missing>" if it cannot be read.
func readTestFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		return "<missing>"
	}

	return string(b)
}

func TestLocalFileSystem_RemoveAll_RemovesOnlyTheTarget(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)

	// Same-named files in the working directory must survive.
	writeTestFile(t, "config.yaml", "app config")
	writeTestFile(t, "b.txt", "app b")

	target := filepath.Join(t.TempDir(), "uploads")
	writeTestFile(t, filepath.Join(target, "config.yaml"), "uploaded")
	writeTestFile(t, filepath.Join(target, "sub", "b.txt"), "nested")

	fs := newTestLocalFS(t)

	require.NoError(t, fs.RemoveAll(target))

	assert.NoDirExists(t, target)
	assert.Equal(t, "app config", readTestFile(t, "config.yaml"))
	assert.Equal(t, "app b", readTestFile(t, "b.txt"))

	// Removing it again, or any missing path, succeeds like os.RemoveAll.
	require.NoError(t, fs.RemoveAll(target))
	require.NoError(t, fs.RemoveAll("missing"))
}

func TestLocalFileSystem_RemoveAll_LeavesRemovedWorkingDirectory(t *testing.T) {
	parent := t.TempDir()
	parent, err := filepath.EvalSymlinks(parent) // Getwd reports the resolved path
	require.NoError(t, err)

	testCases := []struct {
		desc   string
		wd     string
		remove string
	}{
		{desc: "working directory removed by a relative path", wd: "current", remove: "../current"},
		{desc: "working directory inside the removed directory", wd: filepath.Join("removed", "deeper"),
			remove: filepath.Join(parent, "removed")},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			wd := filepath.Join(parent, tc.wd)
			require.NoError(t, os.MkdirAll(wd, 0o755))
			t.Chdir(wd)

			require.NoError(t, newTestLocalFS(t).RemoveAll(tc.remove))

			got, err := os.Getwd()
			require.NoError(t, err)
			assert.Equal(t, parent, got)
		})
	}
}

func TestLocalFileSystem_RemoveAll_EmptyPathKeepsWorkingDirectory(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	writeTestFile(t, "keep.txt", "kept")

	wd, err := os.Getwd()
	require.NoError(t, err)

	// os.RemoveAll("") removes nothing, so the working directory must not change.
	require.NoError(t, newTestLocalFS(t).RemoveAll(""))

	got, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, wd, got)
	assert.Equal(t, "kept", readTestFile(t, "keep.txt"))
}

func TestLocalFileSystem_RemoveAll_Error(t *testing.T) {
	t.Chdir(t.TempDir())

	// os.RemoveAll rejects a path ending in ".".
	err := newTestLocalFS(t).RemoveAll(".")

	require.Error(t, err)
}

func TestLocalFileSystem_Rename(t *testing.T) {
	testCases := []struct {
		desc  string
		setup func(t *testing.T)
		from  string
		to    string
		check func(t *testing.T)
	}{
		{
			desc:  "same file written two ways is left alone",
			setup: func(t *testing.T) { t.Helper(); writeTestFile(t, "report.csv", "important") },
			from:  "report.csv",
			to:    "./report.csv",
			check: func(t *testing.T) {
				t.Helper()
				assert.Equal(t, "important", readTestFile(t, "report.csv"))
			},
		},
		{
			desc:  "file is moved into a missing directory",
			setup: func(t *testing.T) { t.Helper(); writeTestFile(t, "a.txt", "a") },
			from:  "a.txt",
			to:    filepath.Join("x", "y", "b.txt"),
			check: func(t *testing.T) {
				t.Helper()
				assert.NoFileExists(t, "a.txt")
				assert.Equal(t, "a", readTestFile(t, filepath.Join("x", "y", "b.txt")))
			},
		},
		{
			desc:  "directory is moved, leaving no stray file",
			setup: func(t *testing.T) { t.Helper(); writeTestFile(t, filepath.Join("dir", "f.txt"), "x") },
			from:  "dir",
			to:    "dir2",
			check: func(t *testing.T) {
				t.Helper()
				assert.NoDirExists(t, "dir")
				assert.Equal(t, "x", readTestFile(t, filepath.Join("dir2", "f.txt")))
			},
		},
		{
			desc: "symlink destination is replaced, not written through",
			setup: func(t *testing.T) {
				t.Helper()
				writeTestFile(t, "config-v1.json", "v1")
				require.NoError(t, os.Symlink("config-v1.json", "current.json"))
				writeTestFile(t, "tmp.json", "v2")
			},
			from: "tmp.json",
			to:   "current.json",
			check: func(t *testing.T) {
				t.Helper()
				assert.Equal(t, "v1", readTestFile(t, "config-v1.json"))
				assert.Equal(t, "v2", readTestFile(t, "current.json"))
			},
		},
		{
			desc: "permissions are kept",
			setup: func(t *testing.T) {
				t.Helper()
				writeTestFile(t, "id.key", "k")
				require.NoError(t, os.Chmod("id.key", 0o600))
			},
			from: "id.key",
			to:   "id.key.bak",
			check: func(t *testing.T) {
				t.Helper()

				info, err := os.Stat("id.key.bak")
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Chdir(t.TempDir())
			tc.setup(t)

			require.NoError(t, newTestLocalFS(t).Rename(tc.from, tc.to))
			tc.check(t)
		})
	}
}

func TestLocalFileSystem_Rename_Errors(t *testing.T) {
	t.Chdir(t.TempDir())

	fs := newTestLocalFS(t)

	// A missing source fails without creating the destination's directories.
	require.ErrorIs(t, fs.Rename("missing.txt", filepath.Join("new", "dir", "b.txt")), os.ErrNotExist)
	assert.NoDirExists(t, "new")

	// The destination's parent cannot be created because it is a file.
	writeTestFile(t, "a.txt", "a")
	writeTestFile(t, "parent", "a file")
	require.Error(t, fs.Rename("a.txt", filepath.Join("parent", "b.txt")))
	assert.Equal(t, "a", readTestFile(t, "a.txt"))

	// os.Rename refuses to replace a non-empty directory.
	writeTestFile(t, filepath.Join("src", "f.txt"), "src")
	writeTestFile(t, filepath.Join("dst", "f.txt"), "dst")
	require.Error(t, fs.Rename("src", "dst"))
	assert.Equal(t, "dst", readTestFile(t, filepath.Join("dst", "f.txt")))
}

func TestMoveAcrossDevices(t *testing.T) {
	t.Chdir(t.TempDir())

	errRename := &os.LinkError{Op: "rename", Old: "src", New: "dst", Err: os.ErrInvalid}
	modTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	writeTestFile(t, "src.bin", "payload")
	require.NoError(t, os.Chmod("src.bin", 0o600))
	require.NoError(t, os.Chtimes("src.bin", modTime, modTime))
	require.NoError(t, os.Mkdir("dst", 0o755)) // Rename creates it before calling moveAcrossDevices

	require.NoError(t, moveAcrossDevices("src.bin", filepath.Join("dst", "out.bin"), errRename))

	assert.NoFileExists(t, "src.bin")
	assert.Equal(t, "payload", readTestFile(t, filepath.Join("dst", "out.bin")))

	info, err := os.Stat(filepath.Join("dst", "out.bin"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.True(t, info.ModTime().Equal(modTime))

	// A missing source fails.
	require.ErrorIs(t, moveAcrossDevices("missing", "out", errRename), os.ErrNotExist)

	// The temporary file cannot be created in a missing directory.
	writeTestFile(t, "src2.bin", "payload")
	require.Error(t, moveAcrossDevices("src2.bin", filepath.Join("no-such-dir", "out.bin"), errRename))

	// The final rename onto a directory fails, and the temporary file is cleaned up.
	require.NoError(t, os.MkdirAll(filepath.Join("busy", "out.bin", "child"), 0o755))
	require.Error(t, moveAcrossDevices("src2.bin", filepath.Join("busy", "out.bin"), errRename))
	assert.Equal(t, "payload", readTestFile(t, "src2.bin"))

	entries, err := os.ReadDir("busy")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "temporary file should have been removed")
}

func TestMoveAcrossDevices_OnlyMovesRegularFiles(t *testing.T) {
	t.Chdir(t.TempDir())

	errRename := &os.LinkError{Op: "rename", Old: "src", New: "dst", Err: os.ErrInvalid}

	// A directory is not copied.
	require.NoError(t, os.Mkdir("dir", 0o755))
	require.ErrorIs(t, moveAcrossDevices("dir", "dir2", errRename), errRename)
	assert.DirExists(t, "dir")

	// A symlink is not replaced by a copy of its target.
	writeTestFile(t, "target.txt", "target")
	require.NoError(t, os.Symlink("target.txt", "link"))
	require.ErrorIs(t, moveAcrossDevices("link", "link2", errRename), errRename)

	linkTarget, err := os.Readlink("link")
	require.NoError(t, err)
	assert.Equal(t, "target.txt", linkTarget)
	assert.NoFileExists(t, "link2")
}

func TestMoveAcrossDevices_SameFileIsLeftAlone(t *testing.T) {
	t.Chdir(t.TempDir())

	errRename := &os.LinkError{Op: "rename", Old: "src", New: "dst", Err: os.ErrInvalid}

	// Two names for one file, as a bind mount gives: the file must survive.
	writeTestFile(t, "a.txt", "only copy")
	require.NoError(t, os.Link("a.txt", "b.txt"))

	require.NoError(t, moveAcrossDevices("a.txt", "b.txt", errRename))

	assert.Equal(t, "only copy", readTestFile(t, "a.txt"))
	assert.Equal(t, "only copy", readTestFile(t, "b.txt"))
}

func TestLocalFileSystem_OpenFile(t *testing.T) {
	testCases := []struct {
		desc    string
		initial string
		flag    int
		perm    os.FileMode
		write   string
		want    string
	}{
		{desc: "O_APPEND keeps existing content", initial: "line1\n",
			flag: os.O_APPEND | os.O_CREATE | os.O_WRONLY, perm: 0o644, write: "line2\n", want: "line1\nline2\n"},
		{desc: "O_WRONLY without O_TRUNC overwrites in place", initial: "hello world",
			flag: os.O_WRONLY, perm: 0o644, write: "J", want: "Jello world"},
		{desc: "O_TRUNC empties the file", initial: "old",
			flag: os.O_WRONLY | os.O_TRUNC, perm: 0o644, write: "new", want: "new"},
		{desc: "O_RDWR writes in place", initial: "hello world",
			flag: os.O_RDWR, perm: 0o644, write: "H", want: "Hello world"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeTestFile(t, "f.txt", tc.initial)

			f, err := newTestLocalFS(t).OpenFile("f.txt", tc.flag, tc.perm)
			require.NoError(t, err)

			_, err = f.Write([]byte(tc.write))
			require.NoError(t, err)
			require.NoError(t, f.Close())

			assert.Equal(t, tc.want, readTestFile(t, "f.txt"))
		})
	}
}

func TestLocalFileSystem_OpenFile_CreateFlags(t *testing.T) {
	t.Chdir(t.TempDir())

	fs := newTestLocalFS(t)

	// O_EXCL fails on an existing file and leaves it untouched.
	writeTestFile(t, "job.lock", "owner=A")

	_, err := fs.OpenFile("job.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	require.ErrorIs(t, err, os.ErrExist)
	assert.Equal(t, "owner=A", readTestFile(t, "job.lock"))

	// perm is used for a new file, and missing parent directories are created.
	path := filepath.Join("keys", "secret.key")

	f, err := fs.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Parent directories cannot be created under a file.
	writeTestFile(t, "parent", "a file")

	_, err = fs.OpenFile(filepath.Join("parent", "child.txt"), os.O_CREATE|os.O_WRONLY, 0o644)
	require.Error(t, err)

	// A missing file without O_CREATE is reported as not found.
	_, err = fs.OpenFile("missing.txt", os.O_RDONLY, 0)
	require.ErrorIs(t, err, ErrFileNotFound)
}

func TestLocalFileSystem_OpenFile_ReadWriteHandle(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "data.txt", "hello world")

	f, err := newTestLocalFS(t).OpenFile("data.txt", os.O_RDWR, 0o644)
	require.NoError(t, err)

	pos, err := f.Seek(6, io.SeekStart)
	require.NoError(t, err)
	assert.Equal(t, int64(6), pos)

	buf := make([]byte, 5)
	_, err = f.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(buf))

	_, err = f.WriteAt([]byte("H"), 0)
	require.NoError(t, err)

	_, err = f.Write([]byte("W"))
	require.NoError(t, err)

	require.NoError(t, f.Close())
	assert.Equal(t, "Hello World", readTestFile(t, "data.txt"))
}

func TestLocalFileSystem_Create(t *testing.T) {
	t.Chdir(t.TempDir())

	fs := newTestLocalFS(t)
	path := filepath.Join("out", "data.bin")

	f, err := fs.Create(path)
	require.NoError(t, err)

	_, err = f.Write([]byte("hello world"))
	require.NoError(t, err)

	// Seek moves the write offset, so the next Write overwrites.
	end, err := f.Seek(0, io.SeekEnd)
	require.NoError(t, err)
	assert.Equal(t, int64(11), end)

	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)

	_, err = f.Write([]byte("HELLO"))
	require.NoError(t, err)
	assert.Equal(t, int64(11), f.Size())
	require.NoError(t, f.Close())

	assert.Equal(t, "HELLO world", readTestFile(t, path))

	// Create returns the error instead of a file that fails on every write.
	writeTestFile(t, "parent", "a file")

	_, err = fs.Create(filepath.Join("parent", "child.txt"))
	require.Error(t, err)

	require.NoError(t, os.Mkdir("dir", 0o755))

	_, err = fs.Create("dir")
	require.Error(t, err)
}

func TestLocalFileSystem_Open(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "cfg.txt", "OLD-0123456789")

	fs := newTestLocalFS(t)

	f, err := fs.Open("cfg.txt")
	require.NoError(t, err)

	head := make([]byte, 4)
	_, err = io.ReadFull(f, head)
	require.NoError(t, err)

	// Another writer replaces the file. The open handle keeps reading the file it opened.
	writeTestFile(t, "cfg.tmp", "NEW-abcdefghij")
	require.NoError(t, os.Rename("cfg.tmp", "cfg.txt"))

	_, err = f.Seek(4, io.SeekStart)
	require.NoError(t, err)

	rest, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "OLD-0123456789", string(head)+string(rest))

	buf := make([]byte, 3)
	_, err = f.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, "OLD", string(buf))
	require.NoError(t, f.Close())

	_, err = fs.Open("missing.txt")
	require.ErrorIs(t, err, ErrFileNotFound)
}
