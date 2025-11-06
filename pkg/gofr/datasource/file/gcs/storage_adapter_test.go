package gcs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
)

var errTest = errors.New("test error")

// Mock writer for testing.
type mockWriter struct {
	buf *bytes.Buffer
	err error
}

func (m *mockWriter) Write(p []byte) (int, error) {
	if m.err != nil {
		return 0, m.err
	}

	return m.buf.Write(p)
}

func (m *mockWriter) Close() error {
	return m.err
}

func TestStorageAdapter_Connect(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.Connect(context.Background())

	assert.NoError(t, err)
}

func TestStorageAdapter_Health_Success(t *testing.T) {
	// This requires actual GCS client, skip in unit tests
	t.Skip("Requires GCS client")
}

func TestStorageAdapter_Health_NilClient(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.Health(context.Background())

	require.Error(t, err)
	assert.ErrorIs(t, err, errGCSClientNotInitialized)
}

func TestStorageAdapter_Close_NilClient(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.Close()

	assert.NoError(t, err)
}

func TestStorageAdapter_NewReader_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	reader, err := adapter.NewReader(context.Background(), "")

	require.Error(t, err)
	assert.Nil(t, reader)
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_NewRangeReader_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	reader, err := adapter.NewRangeReader(context.Background(), "", 0, 100)

	require.Error(t, err)
	assert.Nil(t, reader)
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_NewRangeReader_InvalidOffset(t *testing.T) {
	adapter := &storageAdapter{}

	reader, err := adapter.NewRangeReader(context.Background(), "file.txt", -1, 100)

	require.Error(t, err)
	assert.Nil(t, reader)
	assert.ErrorIs(t, err, errInvalidOffset)
}

func TestStorageAdapter_NewWriter_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	writer := adapter.NewWriter(context.Background(), "")

	require.NotNil(t, writer)

	// Verify it's a fail writer
	n, err := writer.Write([]byte("test"))
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_StatObject_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	info, err := adapter.StatObject(context.Background(), "")

	require.Error(t, err)
	assert.Nil(t, info)
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_DeleteObject_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.DeleteObject(context.Background(), "")

	require.Error(t, err)
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_CopyObject_EmptySource(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.CopyObject(context.Background(), "", "dest.txt")

	require.Error(t, err)
	assert.ErrorIs(t, err, errEmptySourceOrDest)
}

func TestStorageAdapter_CopyObject_EmptyDestination(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.CopyObject(context.Background(), "source.txt", "")

	require.Error(t, err)
	assert.ErrorIs(t, err, errEmptySourceOrDest)
}

func TestStorageAdapter_CopyObject_BothEmpty(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.CopyObject(context.Background(), "", "")

	require.Error(t, err)
	assert.ErrorIs(t, err, errEmptySourceOrDest)
}

func TestStorageAdapter_CopyObject_SameSourceAndDest(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.CopyObject(context.Background(), "file.txt", "file.txt")

	require.Error(t, err)
	assert.ErrorIs(t, err, errSameSourceAndDest)
}

func TestFailWriter_Write(t *testing.T) {
	testErr := errTest
	fw := &failWriter{err: testErr}

	n, err := fw.Write([]byte("test"))

	assert.Equal(t, 0, n)
	assert.Equal(t, testErr, err)
}

func TestFailWriter_Close(t *testing.T) {
	testErr := errTest
	fw := &failWriter{err: testErr}

	err := fw.Close()

	assert.Equal(t, testErr, err)
}

func TestFailWriter_WriteEmptyObjectNameError(t *testing.T) {
	fw := &failWriter{err: errEmptyObjectName}

	n, err := fw.Write([]byte("data"))

	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestFailWriter_CloseEmptyObjectNameError(t *testing.T) {
	fw := &failWriter{err: errEmptyObjectName}

	err := fw.Close()

	assert.ErrorIs(t, err, errEmptyObjectName)
}

// Tests using MockStorageProvider interface

func TestMockStorageProvider_NewReader_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedReader := io.NopCloser(strings.NewReader("test data"))

	mockProvider.EXPECT().
		NewReader(gomock.Any(), "test.txt").
		Return(expectedReader, nil)

	reader, err := mockProvider.NewReader(context.Background(), "test.txt")

	require.NoError(t, err)
	assert.Equal(t, expectedReader, reader)
}

func TestMockStorageProvider_NewRangeReader_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedReader := io.NopCloser(strings.NewReader("partial"))

	mockProvider.EXPECT().
		NewRangeReader(gomock.Any(), "file.txt", int64(0), int64(100)).
		Return(expectedReader, nil)

	reader, err := mockProvider.NewRangeReader(context.Background(), "file.txt", 0, 100)

	require.NoError(t, err)
	assert.Equal(t, expectedReader, reader)
}

func TestMockStorageProvider_NewWriter_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	buf := &bytes.Buffer{}
	expectedWriter := &mockWriter{buf: buf}

	mockProvider.EXPECT().
		NewWriter(gomock.Any(), "output.txt").
		Return(expectedWriter)

	writer := mockProvider.NewWriter(context.Background(), "output.txt")

	assert.Equal(t, expectedWriter, writer)
}

func TestMockStorageProvider_StatObject_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedInfo := &file.ObjectInfo{
		Name: "test.txt",
		Size: int64(1024),
	}

	mockProvider.EXPECT().
		StatObject(gomock.Any(), "test.txt").
		Return(expectedInfo, nil)

	info, err := mockProvider.StatObject(context.Background(), "test.txt")

	require.NoError(t, err)
	assert.Equal(t, "test.txt", info.Name)
	assert.Equal(t, int64(1024), info.Size)
}

func TestMockStorageProvider_DeleteObject_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)

	mockProvider.EXPECT().
		DeleteObject(gomock.Any(), "delete-me.txt").
		Return(nil)

	err := mockProvider.DeleteObject(context.Background(), "delete-me.txt")

	assert.NoError(t, err)
}

func TestMockStorageProvider_CopyObject_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)

	mockProvider.EXPECT().
		CopyObject(gomock.Any(), "source.txt", "dest.txt").
		Return(nil)

	err := mockProvider.CopyObject(context.Background(), "source.txt", "dest.txt")

	assert.NoError(t, err)
}

func TestMockStorageProvider_ListObjects_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedObjects := []string{"file1.txt", "file2.txt"}

	mockProvider.EXPECT().
		ListObjects(gomock.Any(), "prefix/").
		Return(expectedObjects, nil)

	objects, err := mockProvider.ListObjects(context.Background(), "prefix/")

	require.NoError(t, err)
	assert.Equal(t, expectedObjects, objects)
}

func TestMockStorageProvider_ListDir_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedFiles := []file.ObjectInfo{
		{Name: "file1.txt", Size: 100},
		{Name: "file2.txt", Size: 200},
	}
	expectedDirs := []string{"subdir1/", "subdir2/"}

	mockProvider.EXPECT().
		ListDir(gomock.Any(), "root/").
		Return(expectedFiles, expectedDirs, nil)

	files, dirs, err := mockProvider.ListDir(context.Background(), "root/")

	require.NoError(t, err)
	assert.Equal(t, expectedFiles, files)
	assert.Equal(t, expectedDirs, dirs)
}

func TestMockStorageProvider_ListDir_EmptyPrefix(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedFiles := []file.ObjectInfo{{Name: "root-file.txt"}}

	mockProvider.EXPECT().
		ListDir(gomock.Any(), "").
		Return(expectedFiles, []string{}, nil)

	files, dirs, err := mockProvider.ListDir(context.Background(), "")

	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Empty(t, dirs)
}

func TestMockStorageProvider_ListDir_WithPrefix(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	prefix := "data/"
	expectedFiles := []file.ObjectInfo{
		{Name: "data/file.json", Size: 512},
	}

	mockProvider.EXPECT().
		ListDir(gomock.Any(), prefix).
		Return(expectedFiles, []string{"data/subdir/"}, nil)

	files, dirs, err := mockProvider.ListDir(context.Background(), prefix)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Len(t, dirs, 1)
}

func TestMockStorageProvider_ListDir_OnlyFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedFiles := []file.ObjectInfo{
		{Name: "file1.txt"},
		{Name: "file2.txt"},
	}

	mockProvider.EXPECT().
		ListDir(gomock.Any(), "files/").
		Return(expectedFiles, []string{}, nil)

	files, dirs, err := mockProvider.ListDir(context.Background(), "files/")

	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Empty(t, dirs)
}

func TestMockStorageProvider_ListDir_OnlyDirectories(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedDirs := []string{"dir1/", "dir2/", "dir3/"}

	mockProvider.EXPECT().
		ListDir(gomock.Any(), "root/").
		Return([]file.ObjectInfo{}, expectedDirs, nil)

	files, dirs, err := mockProvider.ListDir(context.Background(), "root/")

	require.NoError(t, err)
	assert.Empty(t, files)
	assert.Len(t, dirs, 3)
}

func TestMockStorageProvider_ListDir_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)

	mockProvider.EXPECT().
		ListDir(gomock.Any(), "empty/").
		Return([]file.ObjectInfo{}, []string{}, nil)

	files, dirs, err := mockProvider.ListDir(context.Background(), "empty/")

	require.NoError(t, err)
	assert.Empty(t, files)
	assert.Empty(t, dirs)
}

func TestMockStorageProvider_ListDir_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)
	expectedErr := errTest

	mockProvider.EXPECT().
		ListDir(gomock.Any(), "forbidden/").
		Return(nil, nil, expectedErr)

	files, dirs, err := mockProvider.ListDir(context.Background(), "forbidden/")

	require.Error(t, err)
	assert.Nil(t, files)
	assert.Nil(t, dirs)
	assert.Equal(t, expectedErr, err)
}

func TestMockStorageProvider_ListDir_MixedContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := file.NewMockStorageProvider(ctrl)

	expectedFiles := []file.ObjectInfo{
		{
			Name:         "data/file1.json",
			Size:         int64(1024),
			ContentType:  "application/json",
			LastModified: time.Now(),
			IsDir:        false,
		},
		{
			Name:         "data/file2.txt",
			Size:         int64(2048),
			ContentType:  "text/plain",
			LastModified: time.Now(),
			IsDir:        false,
		},
	}

	expectedDirs := []string{"data/images/", "data/docs/"}

	mockProvider.EXPECT().
		ListDir(gomock.Any(), "data/").
		Return(expectedFiles, expectedDirs, nil)

	files, dirs, err := mockProvider.ListDir(context.Background(), "data/")

	require.NoError(t, err)
	require.Len(t, files, 2)
	require.Len(t, dirs, 2)

	assert.Equal(t, "data/file1.json", files[0].Name)
	assert.Equal(t, int64(1024), files[0].Size)
	assert.Equal(t, "application/json", files[0].ContentType)
	assert.False(t, files[0].IsDir)

	assert.Equal(t, "data/file2.txt", files[1].Name)
	assert.Equal(t, expectedDirs, dirs)
}
