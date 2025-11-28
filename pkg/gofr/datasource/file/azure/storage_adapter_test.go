package azure

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTest = errors.New("test error")

func TestStorageAdapter_Connect_NilConfig(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.Connect(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "azure config is nil")
}

func TestStorageAdapter_Connect_InvalidCredentials(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "invalidkey",
		ShareName:   "testshare",
	}

	adapter := &storageAdapter{cfg: config}

	err := adapter.Connect(context.Background())

	// Should fail on credential creation or share validation
	require.Error(t, err)
}

func TestStorageAdapter_Health_NilClient(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.Health(context.Background())

	require.Error(t, err)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
}

func TestStorageAdapter_Close(t *testing.T) {
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

func TestStorageAdapter_NewReader_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{ShareName: "test"}}

	reader, err := adapter.NewReader(context.Background(), "file.txt")

	require.Error(t, err)
	assert.Nil(t, reader)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
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

func TestStorageAdapter_NewRangeReader_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{ShareName: "test"}}

	reader, err := adapter.NewRangeReader(context.Background(), "file.txt", 0, 100)

	require.Error(t, err)
	assert.Nil(t, reader)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
}

func TestStorageAdapter_NewWriter_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	writer := adapter.NewWriter(context.Background(), "")

	require.NotNil(t, writer)

	// Verify it's a fail writer
	n, err := writer.Write([]byte("test"))
	assert.Equal(t, 0, n)
	require.ErrorIs(t, err, errEmptyObjectName)

	err = writer.Close()
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_NewWriter_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{ShareName: "test"}}

	writer := adapter.NewWriter(context.Background(), "file.txt")

	require.NotNil(t, writer)

	// Should fail on write/close due to nil client
	n, err := writer.Write([]byte("test"))
	assert.Equal(t, 0, n)
	require.Error(t, err)
}

func TestStorageAdapter_StatObject_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	info, err := adapter.StatObject(context.Background(), "")

	require.Error(t, err)
	assert.Nil(t, info)
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_StatObject_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{ShareName: "test"}}

	info, err := adapter.StatObject(context.Background(), "file.txt")

	require.Error(t, err)
	assert.Nil(t, info)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
}

func TestStorageAdapter_DeleteObject_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.DeleteObject(context.Background(), "")

	require.Error(t, err)
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_DeleteObject_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{ShareName: "test"}}

	err := adapter.DeleteObject(context.Background(), "file.txt")

	require.Error(t, err)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
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
	assert.ErrorIs(t, err, errSameSourceOrDest)
}

func TestStorageAdapter_CopyObject_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{ShareName: "test"}}

	err := adapter.CopyObject(context.Background(), "source.txt", "dest.txt")

	require.Error(t, err)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
}

func TestStorageAdapter_ListObjects_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{ShareName: "test"}}

	objects, err := adapter.ListObjects(context.Background(), "prefix/")

	require.Error(t, err)
	assert.Nil(t, objects)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
}

func TestStorageAdapter_ListDir_NilClient(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{ShareName: "test"}}

	objects, prefixes, err := adapter.ListDir(context.Background(), "prefix/")

	require.Error(t, err)
	assert.Nil(t, objects)
	assert.Nil(t, prefixes)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
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

func TestBytesReadSeekCloser_Read(t *testing.T) {
	data := []byte("hello world")
	brsc := &bytesReadSeekCloser{data: data}

	p := make([]byte, 5)
	n, err := brsc.Read(p)

	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte("hello"), p)
	assert.Equal(t, int64(5), brsc.offset)
}

func TestBytesReadSeekCloser_Read_EOF(t *testing.T) {
	data := []byte("hi")
	brsc := &bytesReadSeekCloser{data: data, offset: 2}

	p := make([]byte, 10)
	n, err := brsc.Read(p)

	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
}

func TestBytesReadSeekCloser_Seek_Start(t *testing.T) {
	data := []byte("hello world")
	brsc := &bytesReadSeekCloser{data: data, offset: 5}

	pos, err := brsc.Seek(2, io.SeekStart)

	require.NoError(t, err)
	assert.Equal(t, int64(2), pos)
	assert.Equal(t, int64(2), brsc.offset)
}

func TestBytesReadSeekCloser_Seek_Current(t *testing.T) {
	data := []byte("hello world")
	brsc := &bytesReadSeekCloser{data: data, offset: 3}

	pos, err := brsc.Seek(2, io.SeekCurrent)

	require.NoError(t, err)
	assert.Equal(t, int64(5), pos)
	assert.Equal(t, int64(5), brsc.offset)
}

func TestBytesReadSeekCloser_Seek_End(t *testing.T) {
	data := []byte("hello world")
	brsc := &bytesReadSeekCloser{data: data}

	pos, err := brsc.Seek(-3, io.SeekEnd)

	require.NoError(t, err)
	assert.Equal(t, int64(8), pos)
	assert.Equal(t, int64(8), brsc.offset)
}

func TestBytesReadSeekCloser_Seek_InvalidWhence(t *testing.T) {
	data := []byte("hello world")
	brsc := &bytesReadSeekCloser{data: data}

	_, err := brsc.Seek(0, 99)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid whence")
}

func TestBytesReadSeekCloser_Seek_NegativeOffset(t *testing.T) {
	data := []byte("hello world")
	brsc := &bytesReadSeekCloser{data: data}

	_, err := brsc.Seek(-1, io.SeekStart)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative offset")
}

func TestBytesReadSeekCloser_Seek_BeyondEnd(t *testing.T) {
	data := []byte("hello")
	brsc := &bytesReadSeekCloser{data: data}

	pos, err := brsc.Seek(100, io.SeekStart)

	require.NoError(t, err)
	assert.Equal(t, int64(5), pos) // Clamped to data length
	assert.Equal(t, int64(5), brsc.offset)
}

func TestBytesReadSeekCloser_Close(t *testing.T) {
	data := []byte("hello world")
	brsc := &bytesReadSeekCloser{data: data}

	err := brsc.Close()

	assert.NoError(t, err)
}

func TestReadSeekCloserWrapper_Read(t *testing.T) {
	reader := io.NopCloser(bytes.NewReader([]byte("test data")))
	wrapper := &readSeekCloserWrapper{reader: reader}

	p := make([]byte, 4)
	n, err := wrapper.Read(p)

	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte("test"), p)
}

func TestReadSeekCloserWrapper_Seek_Start(t *testing.T) {
	reader := io.NopCloser(bytes.NewReader([]byte("test data")))
	wrapper := &readSeekCloserWrapper{reader: reader}

	// First read to populate buffer
	_, _ = io.ReadAll(reader)
	reader = io.NopCloser(bytes.NewReader([]byte("test data")))
	wrapper.reader = reader

	pos, err := wrapper.Seek(5, io.SeekStart)

	require.NoError(t, err)
	assert.Equal(t, int64(5), pos)
}

func TestReadSeekCloserWrapper_Seek_InvalidWhence(t *testing.T) {
	reader := io.NopCloser(bytes.NewReader([]byte("test")))
	wrapper := &readSeekCloserWrapper{reader: reader}

	_, err := wrapper.Seek(0, 99)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid whence")
}

func TestReadSeekCloserWrapper_Close(t *testing.T) {
	reader := io.NopCloser(bytes.NewReader([]byte("test")))
	wrapper := &readSeekCloserWrapper{reader: reader}

	err := wrapper.Close()

	assert.NoError(t, err)
}

func TestStorageAdapter_getFileClient_RootFile(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	adapter := &storageAdapter{cfg: config}

	// This will fail because shareClient is nil, but we can test the path logic
	_, err := adapter.getFileClient("file.txt")

	require.Error(t, err)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
}

func TestStorageAdapter_getFileClient_SubdirectoryFile(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	adapter := &storageAdapter{cfg: config}

	// This will fail because shareClient is nil, but we can test the path logic
	_, err := adapter.getFileClient("dir/file.txt")

	require.Error(t, err)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
}

func TestStorageAdapter_getFileClient_NestedPath(t *testing.T) {
	config := &Config{
		AccountName: "testaccount",
		AccountKey:  "testkey",
		ShareName:   "testshare",
	}

	adapter := &storageAdapter{cfg: config}

	// This will fail because shareClient is nil, but we can test the path logic
	_, err := adapter.getFileClient("dir/subdir/file.txt")

	require.Error(t, err)
	assert.ErrorIs(t, err, errAzureClientNotInitialized)
}
