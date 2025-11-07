package gcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/option"
)

var errTest = errors.New("test error")

func TestStorageAdapter_Connect(t *testing.T) {
	adapter := &storageAdapter{}

	err := adapter.Connect(context.Background())

	assert.NoError(t, err)
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

func TestStorageAdapter_ListObjects_Success(t *testing.T) {
	// Fake GCS server returning two objects for the given prefix
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Expect path like /storage/v1/b/<bucket>/o
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"kind":"storage#objects",
			"items":[
				{"name":"prefix/file1.txt"},
				{"name":"prefix/file2.txt"}
			]
		}`))
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Create client pointing to our fake server, and without auth
	client, err := storage.NewClient(t.Context(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{
		client: client,
		bucket: client.Bucket("test-bucket"),
	}

	objects, err := adapter.ListObjects(t.Context(), "prefix/")

	require.NoError(t, err)
	assert.Equal(t, []string{"prefix/file1.txt", "prefix/file2.txt"}, objects)
}

func TestStorageAdapter_ListDir_Success(t *testing.T) {
	// Response with one file item and one prefix (subdir)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"kind":"storage#objects",
			"prefixes":["prefix/subdir/"],
			"items":[
				{
					"name":"prefix/file1.txt",
					"size":"123",
					"contentType":"text/plain",
					"updated":"2020-01-01T00:00:00.000Z"
				}
			]
		}`))
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := storage.NewClient(t.Context(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{
		client: client,
		bucket: client.Bucket("test-bucket"),
	}

	files, dirs, err := adapter.ListDir(t.Context(), "prefix/")

	require.NoError(t, err)

	// Expect one file with matching fields and one dir prefix
	require.Len(t, files, 1)
	assert.Equal(t, "prefix/file1.txt", files[0].Name)
	assert.Equal(t, int64(123), files[0].Size)
	assert.Equal(t, "text/plain", files[0].ContentType)

	require.Len(t, dirs, 1)
	assert.Equal(t, "prefix/subdir/", dirs[0])
}

func TestStorageAdapter_ListDir_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})

	srv := httptest.NewServer(handler)

	defer srv.Close()

	client, err := storage.NewClient(t.Context(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{
		client: client,
		bucket: client.Bucket("test-bucket"),
	}

	_, _, err = adapter.ListDir(t.Context(), "prefix/")

	require.Error(t, err)
	assert.Contains(t, err.Error(), errFailedToListDirectory.Error())
}

func TestStorageAdapter_NewReader_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond with raw bytes for any request that references the object name
		// or explicitly asks for media/range. This covers variations of client requests.
		if r.URL.Query().Get("alt") == "media" ||
			r.Header.Get("Range") != "" ||
			strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/octet-stream") ||
			strings.Contains(r.URL.Path, "file.txt") ||
			strings.Contains(r.URL.RawQuery, "name=file.txt") {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("hello world"))

			return
		}

		// fallback: return JSON metadata for other calls
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"file.txt","size":"11","contentType":"text/plain","updated":"2020-01-01T00:00:00.000Z"}`))
	})

	srv := httptest.NewServer(handler)

	defer srv.Close()

	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{client: client, bucket: client.Bucket("bucket")}
	rc, err := adapter.NewReader(ctx, "file.txt")

	require.NoError(t, err)

	defer rc.Close()

	data, err := io.ReadAll(rc)

	require.NoError(t, err)

	assert.Equal(t, "hello world", string(data))
}

func TestStorageAdapter_StatObject_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// metadata path (JSON)
		if strings.Contains(r.URL.Path, "/o/") && !strings.Contains(r.URL.RawQuery, "alt=media") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"name":"prefix/file1.txt",
				"size":"123",
				"contentType":"text/plain",
				"updated":"2020-01-01T00:00:00.000Z",
				"generation":"42"
			}`))

			return
		}

		http.NotFound(w, r)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{client: client, bucket: client.Bucket("bucket")}
	info, err := adapter.StatObject(ctx, "prefix/file1.txt")

	require.NoError(t, err)

	assert.Equal(t, "prefix/file1.txt", info.Name)
	assert.Equal(t, int64(123), info.Size)
	assert.Equal(t, "text/plain", info.ContentType)
	assert.False(t, info.IsDir)
}

func TestStorageAdapter_DeleteObject_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For attr check: GET metadata
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/o/") && !strings.Contains(r.URL.RawQuery, "alt=media") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"name":"to-delete.txt",
				"size":"10",
				"contentType":"text/plain",
				"updated":"2020-01-01T00:00:00.000Z",
				"generation":"99"
			}`))

			return
		}

		// For delete calls the client issues DELETE (may include ifGenerationMatch param)
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/o/") {
			w.WriteHeader(http.StatusNoContent)

			return
		}

		http.NotFound(w, r)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{client: client, bucket: client.Bucket("bucket")}
	err = adapter.DeleteObject(ctx, "to-delete.txt")

	require.NoError(t, err)
}

func TestStorageAdapter_NewRangeReader_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// treat as download if Range header present or alt=media query
		if r.Header.Get("Range") != "" || r.URL.Query().Get("alt") == "media" {
			const totalSize = int64(20)

			start, end, err := parseRangeHeader(r.Header.Get("Range"), totalSize)
			if err != nil {
				http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
				return
			}

			length := end - start + 1
			if length < 0 {
				http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
				return
			}

			// build body that begins with "partial"
			body := []byte("partial")
			if int64(len(body)) < length {
				body = append(body, bytes.Repeat([]byte("x"), int(length)-len(body))...)
			} else if int64(len(body)) > length {
				body = body[:length]
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize))
			w.Header().Set("Content-Length", strconv.FormatInt(int64(len(body)), 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(body)

			return
		}

		// metadata fallback
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"file.txt","size":"20","contentType":"text/plain","updated":"2020-01-01T00:00:00.000Z"}`))
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := storage.NewClient(t.Context(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{client: client, bucket: client.Bucket("bucket")}

	rc, err := adapter.NewRangeReader(t.Context(), "file.txt", 5, 10)

	require.NoError(t, err)

	defer rc.Close()

	b := make([]byte, 7)
	n, err := rc.Read(b)

	require.NoError(t, err)

	assert.Equal(t, 7, n)
	assert.Equal(t, "partial", string(b))
}

func TestStorageAdapter_CopyObject_Success(t *testing.T) {
	// Minimal handler that returns a completed rewrite response for copier.Run
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/rewriteTo/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"kind":"storage#rewriteResponse","done":true,"resource":{}}`))

			return
		}
		// unexpected -> 404
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := storage.NewClient(t.Context(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{client: client, bucket: client.Bucket("bucket")}

	err = adapter.CopyObject(t.Context(), "source.txt", "dest.txt")
	require.NoError(t, err)
}

// parseRangeHeader parses a "Range" header like "bytes=5-14".
// Returns start,end (inclusive) or an error for invalid values.
func parseRangeHeader(rangeHdr string, total int64) (first, last int64, err error) {
	start := int64(0)
	end := total - 1

	if rangeHdr == "" {
		return start, end, nil
	}

	p0, p1, err := splitRange(rangeHdr)
	if err != nil {
		return 0, 0, err
	}

	if p0 != "" {
		v, err := parseNonNegativeInt(p0)
		if err != nil {
			return 0, 0, err
		}

		start = v
	}

	if p1 != "" {
		v, err := parseNonNegativeInt(p1)
		if err != nil {
			return 0, 0, err
		}

		end = v
	}

	if start < 0 {
		start = 0
	}

	if end >= total {
		end = total - 1
	}

	if end < start {
		return 0, 0, errTest
	}

	return start, end, nil
}

// splitRange validates the header prefix and returns the two side strings.
// e.g. "bytes=5-14" -> "5","14".
func splitRange(rangeHdr string) (firstString, secondString string, err error) {
	const prefix = "bytes="
	if !strings.HasPrefix(rangeHdr, prefix) {
		return "", "", errTest
	}

	parts := strings.SplitN(strings.TrimPrefix(rangeHdr, prefix), "-", 2)
	if len(parts) != 2 {
		return "", "", errTest
	}

	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// parseNonNegativeInt parses a base-10 integer and rejects negatives.
func parseNonNegativeInt(s string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}

	if v < 0 {
		return 0, errTest
	}

	return v, nil
}

func TestStorageAdapter_StatObject_NotFound(t *testing.T) {
	// Handler returns 404 for object metadata requests to simulate object-not-exist.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/o/") && !strings.Contains(r.URL.RawQuery, "alt=media") {
			http.Error(w, "not found", http.StatusNotFound)

			return
		}

		http.NotFound(w, r)
	})

	srv := httptest.NewServer(handler)

	defer srv.Close()

	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{client: client, bucket: client.Bucket("bucket")}
	_, err = adapter.StatObject(ctx, "missing.txt")

	require.Error(t, err)
	require.ErrorIs(t, err, errObjectNotFound)
	assert.Contains(t, err.Error(), "missing.txt")
}

func TestStorageAdapter_Health_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/b/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"test-bucket","id":"1"}`))

			return
		}

		http.NotFound(w, r)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithEndpoint(srv.URL), option.WithoutAuthentication())

	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{client: client, bucket: client.Bucket("test-bucket")}
	err = adapter.Health(ctx)

	require.NoError(t, err)
}
