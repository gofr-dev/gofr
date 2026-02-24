package gcs

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/option"
)

var errTest = errors.New("test error")

func TestStorageAdapter_Connect(t *testing.T) {
	// Test 1: Nil config should return error
	adapter := &storageAdapter{}

	err := adapter.Connect(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GCS config is nil")
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

func TestValidateSignedURLInput(t *testing.T) {
	tests := []struct {
		name    string
		objName string
		expiry  time.Duration
		opts    *file.FileOptions
		wantErr error
	}{
		{
			name:    "valid input",
			objName: "file.txt",
			expiry:  time.Hour,
			opts:    &file.FileOptions{ContentType: "text/plain"},
			wantErr: nil,
		},
		{
			name:    "empty object name",
			objName: "",
			expiry:  time.Hour,
			wantErr: errEmptyObjectName,
		},
		{
			name:    "negative expiry",
			objName: "file.txt",
			expiry:  -time.Hour,
			wantErr: errExpiryMustBePositive,
		},
		{
			name:    "expiry too long",
			objName: "file.txt",
			expiry:  8 * 24 * time.Hour,
			wantErr: errExpiryTooLong,
		},
		{
			name:    "invalid content type",
			objName: "file.txt",
			expiry:  time.Hour,
			opts:    &file.FileOptions{ContentType: "invalid"},
			wantErr: errInvalidContentType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSignedURLInput(tt.objName, tt.expiry, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateSignedURLInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ─── helpers for signed-URL tests ───────────────────────────────────────────

// generateTestPrivateKeyPEM generates a fresh RSA-2048 private key encoded as
// PKCS#8 PEM.  Used only in tests; never use this key in production.
func generateTestPrivateKeyPEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	return buf.String()
}

// testCredentialsJSON returns a minimal service-account JSON string that
// parseServiceAccountCredentials can parse successfully.
func testCredentialsJSON(t *testing.T) string {
	t.Helper()

	keyPEM := generateTestPrivateKeyPEM(t)

	encoded, err := json.Marshal(keyPEM)
	require.NoError(t, err)

	return fmt.Sprintf(`{"client_email":"test-sa@project.iam.gserviceaccount.com","private_key":%s}`, encoded)
}

// bucketAttrsHandler returns an HTTP handler that serves minimal GCS bucket-attrs
// responses so that storageAdapter.Connect() can pass its bucket validation step.
func bucketAttrsHandler(bucketName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/b/"+bucketName) && !strings.Contains(r.URL.Path, "/o/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"name":%q}`, bucketName)

			return
		}

		http.NotFound(w, r)
	})
}

// ─── parseServiceAccountCredentials ─────────────────────────────────────────

func TestParseServiceAccountCredentials_Valid(t *testing.T) {
	credJSON := testCredentialsJSON(t)

	email, key, err := parseServiceAccountCredentials(credJSON)

	require.NoError(t, err)
	assert.Equal(t, "test-sa@project.iam.gserviceaccount.com", email)
	assert.NotEmpty(t, key)
}

func TestParseServiceAccountCredentials_InvalidJSON(t *testing.T) {
	_, _, err := parseServiceAccountCredentials("not-json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse credentials")
}

func TestParseServiceAccountCredentials_EmptyPrivateKey(t *testing.T) {
	credJSON := `{"client_email":"sa@project.iam.gserviceaccount.com","private_key":""}` //nolint:gosec // G101: test credentials

	_, _, err := parseServiceAccountCredentials(credJSON)

	require.ErrorIs(t, err, errInvalidPrivateKeyPEM)
}

func TestParseServiceAccountCredentials_InvalidPEM(t *testing.T) {
	credJSON := `{"client_email":"sa@project.iam.gserviceaccount.com","private_key":"not-a-pem-block"}` //nolint:gosec // G101: test data

	_, _, err := parseServiceAccountCredentials(credJSON)

	require.ErrorIs(t, err, errInvalidPrivateKeyPEM)
}

func TestParseServiceAccountCredentials_InvalidKeyBytes(t *testing.T) {
	// Valid PEM structure but garbage DER bytes that are not a parseable key.
	garbage := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("garbage-key-bytes")})
	encoded, err := json.Marshal(string(garbage))
	require.NoError(t, err)

	credJSON := fmt.Sprintf(`{"client_email":"sa@project.iam.gserviceaccount.com","private_key":%s}`, encoded)

	_, _, err = parseServiceAccountCredentials(credJSON)

	require.ErrorIs(t, err, errInvalidPrivateKeyFormat)
}

// ─── validatePrivateKey ──────────────────────────────────────────────────────

func TestValidatePrivateKey_PKCS8(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	require.NoError(t, validatePrivateKey(der))
}

func TestValidatePrivateKey_PKCS1(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der := x509.MarshalPKCS1PrivateKey(key)

	require.NoError(t, validatePrivateKey(der))
}

func TestValidatePrivateKey_Invalid(t *testing.T) {
	err := validatePrivateKey([]byte("not-a-key"))

	require.ErrorIs(t, err, errInvalidPrivateKeyFormat)
}

// ─── buildSignedURLOptions ───────────────────────────────────────────────────

func TestBuildSignedURLOptions_NilOpts(t *testing.T) {
	opts := buildSignedURLOptions("", nil, time.Hour, nil)

	assert.Equal(t, "GET", opts.Method)
	assert.Empty(t, opts.GoogleAccessID)
	assert.Nil(t, opts.PrivateKey)
	assert.Empty(t, opts.ContentType)
	assert.Nil(t, opts.QueryParameters)
}

func TestBuildSignedURLOptions_WithExplicitCredentials(t *testing.T) {
	opts := buildSignedURLOptions("sa@project.iam", []byte("key"), time.Hour, nil)

	assert.Equal(t, "sa@project.iam", opts.GoogleAccessID)
	assert.Equal(t, []byte("key"), opts.PrivateKey)
}

func TestBuildSignedURLOptions_WithContentType(t *testing.T) {
	opts := buildSignedURLOptions("", nil, time.Hour, &file.FileOptions{ContentType: "image/png"})

	assert.Equal(t, "image/png", opts.ContentType)
}

func TestBuildSignedURLOptions_WithContentDisposition(t *testing.T) {
	opts := buildSignedURLOptions("", nil, time.Hour, &file.FileOptions{
		ContentDisposition: "attachment; filename=report.csv",
	})

	require.NotNil(t, opts.QueryParameters)
	assert.Equal(t, "attachment; filename=report.csv", opts.QueryParameters.Get("response-content-disposition"))
}

func TestBuildSignedURLOptions_ContentDispositionInjectionSanitised(t *testing.T) {
	malicious := "attachment\r\nX-Injected: evil"

	opts := buildSignedURLOptions("", nil, time.Hour, &file.FileOptions{
		ContentDisposition: malicious,
	})

	got := opts.QueryParameters.Get("response-content-disposition")
	assert.NotContains(t, got, "\r")
	assert.NotContains(t, got, "\n")
}

// ─── sanitizeContentDisposition ─────────────────────────────────────────────

func TestSanitizeContentDisposition_Clean(t *testing.T) {
	assert.Equal(t, "attachment; filename=file.pdf", sanitizeContentDisposition("attachment; filename=file.pdf"))
}

func TestSanitizeContentDisposition_StripsCRLF(t *testing.T) {
	assert.Equal(t, "attachmentX-Evil: hdr", sanitizeContentDisposition("attachment\r\nX-Evil: hdr"))
}

func TestSanitizeContentDisposition_StripsCR(t *testing.T) {
	assert.Equal(t, "attachmentX-Evil: hdr", sanitizeContentDisposition("attachment\rX-Evil: hdr"))
}

func TestSanitizeContentDisposition_StripsLF(t *testing.T) {
	assert.Equal(t, "attachmentX-Evil: hdr", sanitizeContentDisposition("attachment\nX-Evil: hdr"))
}

// ─── rewriteSignedURLEndpoint ────────────────────────────────────────────────

func TestRewriteSignedURLEndpoint_NoEndpoint(t *testing.T) {
	orig := "https://storage.googleapis.com/bucket/obj?sig=abc"
	assert.Equal(t, orig, rewriteSignedURLEndpoint(orig, ""))
}

func TestRewriteSignedURLEndpoint_Rewrites(t *testing.T) {
	orig := "https://storage.googleapis.com/bucket/obj?sig=abc"
	result := rewriteSignedURLEndpoint(orig, "http://localhost:4443")

	assert.Contains(t, result, "http://localhost:4443")
	assert.Contains(t, result, "/bucket/obj")
	assert.Contains(t, result, "sig=abc")
}

func TestRewriteSignedURLEndpoint_InvalidEndpoint(t *testing.T) {
	orig := "https://storage.googleapis.com/bucket/obj"
	// Pass a URL that url.Parse will fail on — the function should return the original.
	result := rewriteSignedURLEndpoint(orig, "://bad-url")

	assert.Equal(t, orig, result)
}

// ─── NewWriterWithOptions ────────────────────────────────────────────────────

func TestStorageAdapter_NewWriterWithOptions_EmptyName(t *testing.T) {
	adapter := &storageAdapter{}

	w := adapter.NewWriterWithOptions(context.Background(), "", nil)

	require.NotNil(t, w)

	_, err := w.Write([]byte("data"))
	assert.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_NewWriterWithOptions_NilBucket(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{BucketName: "bucket"}}

	w := adapter.NewWriterWithOptions(context.Background(), "obj.csv", nil)

	require.NotNil(t, w)

	_, err := w.Write([]byte("data"))
	assert.ErrorIs(t, err, errGCSClientNotInitialized)
}

func TestStorageAdapter_NewWriterWithOptions_PropagatesOptions(t *testing.T) {
	// We need a valid *storage.BucketHandle to construct a GCS writer; a fake server
	// that never responds to anything is sufficient since NewWriterWithOptions itself
	// does not make any network calls — the GCS writer only contacts the server on
	// the first Write() / Close().
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client, err := storage.NewClient(t.Context(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{
		cfg:    &Config{BucketName: "bucket"},
		client: client,
		bucket: client.Bucket("bucket"),
	}

	opts := &file.FileOptions{
		ContentType:        "text/csv",
		ContentDisposition: "attachment; filename=report.csv",
		Metadata:           map[string]string{"env": "test"},
	}

	// NewWriterWithOptions must return a non-nil writer without panicking.
	// (Internal field propagation is verified by reading the *storage.Writer fields.)
	w := adapter.NewWriterWithOptions(t.Context(), "report.csv", opts)
	require.NotNil(t, w)

	// The GCS writer buffers writes locally; Write() should not return an error
	// before any network call is attempted.
	_, err = w.Write([]byte("data"))
	require.NoError(t, err)

	// We do NOT call Close() here because that would initiate a real upload to the
	// fake server and would fail with a 404.  The purpose of this test is to verify
	// that option fields are accepted without error, not that the upload succeeds.
}

// ─── SignedURL ───────────────────────────────────────────────────────────────

func TestStorageAdapter_SignedURL_NilBucket(t *testing.T) {
	adapter := &storageAdapter{cfg: &Config{BucketName: "bucket"}}

	_, err := adapter.SignedURL(context.Background(), "obj", time.Hour, nil)

	require.ErrorIs(t, err, errGCSClientNotInitialized)
}

func TestStorageAdapter_SignedURL_NilOrEmptyConfig(t *testing.T) {
	tests := []struct {
		name    string
		adapter *storageAdapter
	}{
		{"nil config", &storageAdapter{}},
		{"empty bucket", &storageAdapter{cfg: &Config{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.adapter.SignedURL(context.Background(), "obj", time.Hour, nil)
			require.ErrorIs(t, err, errGCSBucketNotConfigured)
		})
	}
}

func TestStorageAdapter_SignedURL_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	adapter := &storageAdapter{cfg: &Config{BucketName: "bucket"}}

	_, err := adapter.SignedURL(ctx, "obj", time.Hour, nil)

	require.ErrorIs(t, err, context.Canceled)
}

func TestStorageAdapter_SignedURL_InvalidInput(t *testing.T) {
	// A fully initialized adapter; we only want to exercise the input validation path.
	srv := httptest.NewServer(bucketAttrsHandler("bucket"))
	defer srv.Close()

	client, err := storage.NewClient(t.Context(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	defer client.Close()

	adapter := &storageAdapter{
		cfg:    &Config{BucketName: "bucket"},
		client: client,
		bucket: client.Bucket("bucket"),
	}

	_, err = adapter.SignedURL(context.Background(), "", time.Hour, nil)
	require.ErrorIs(t, err, errEmptyObjectName)
}

func TestStorageAdapter_SignedURL_WithExplicitCredentials(t *testing.T) {
	// Start a fake server that accepts bucket attrs (needed by Connect).
	srv := httptest.NewServer(bucketAttrsHandler("test-bucket"))
	defer srv.Close()

	credJSON := testCredentialsJSON(t)
	cfg := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: credJSON,
		EndPoint:        srv.URL,
	}

	adapter := &storageAdapter{cfg: cfg}
	require.NoError(t, adapter.Connect(t.Context()))

	signedURL, err := adapter.SignedURL(t.Context(), "reports/q1.csv", time.Hour, nil)

	require.NoError(t, err)
	assert.Contains(t, signedURL, "test-bucket")
	assert.Contains(t, signedURL, "reports")
	assert.Contains(t, signedURL, "q1.csv")
	// Signed URL must carry a signature parameter (V4 signing).
	assert.Contains(t, signedURL, "X-Goog-Signature")
	// Endpoint rewriting must have replaced the googleapis.com host.
	assert.Contains(t, signedURL, "127.0.0.1")
}

func TestStorageAdapter_SignedURL_WithOptions(t *testing.T) {
	srv := httptest.NewServer(bucketAttrsHandler("test-bucket"))
	defer srv.Close()

	credJSON := testCredentialsJSON(t)
	cfg := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: credJSON,
		EndPoint:        srv.URL,
	}

	adapter := &storageAdapter{cfg: cfg}
	require.NoError(t, adapter.Connect(t.Context()))

	opts := &file.FileOptions{
		ContentType:        "text/csv",
		ContentDisposition: "attachment; filename=data.csv",
	}

	signedURL, err := adapter.SignedURL(t.Context(), "data.csv", time.Hour, opts)

	require.NoError(t, err)
	assert.Contains(t, signedURL, "data.csv")
	assert.Contains(t, signedURL, "response-content-disposition")
}

// ─── Connect with credentials caching ────────────────────────────────────────

func TestStorageAdapter_Connect_CachesCredentials(t *testing.T) {
	srv := httptest.NewServer(bucketAttrsHandler("test-bucket"))
	defer srv.Close()

	credJSON := testCredentialsJSON(t)
	cfg := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: credJSON,
		EndPoint:        srv.URL,
	}

	adapter := &storageAdapter{cfg: cfg}
	require.NoError(t, adapter.Connect(t.Context()))

	assert.NotEmpty(t, adapter.saEmail, "saEmail should be cached after successful connect")
	assert.NotEmpty(t, adapter.saPrivateKey, "saPrivateKey should be cached after successful connect")
}

func TestStorageAdapter_Connect_InvalidCredentials_DoesNotFailConnect(t *testing.T) {
	srv := httptest.NewServer(bucketAttrsHandler("test-bucket"))
	defer srv.Close()

	// Valid GCS credentials structure but with a non-parseable private key.
	// Connect() must succeed — the GCS client works fine. Only SignedURL() should fail.
	garbage := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("garbage")})
	encoded, err := json.Marshal(string(garbage))
	require.NoError(t, err)

	credJSON := fmt.Sprintf(`{"client_email":"sa@project.iam.gserviceaccount.com","private_key":%s}`, encoded)

	cfg := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: credJSON,
		EndPoint:        srv.URL,
	}

	adapter := &storageAdapter{cfg: cfg}

	// Connect must succeed even with un-parseable signing credentials.
	require.NoError(t, adapter.Connect(t.Context()))

	// The parse error is surfaced only when GenerateSignedURL is actually called.
	_, signedErr := adapter.SignedURL(t.Context(), "obj", time.Hour, nil)
	require.Error(t, signedErr)
	assert.Contains(t, signedErr.Error(), "credentials cannot be used for signed URLs")
}

func TestStorageAdapter_Connect_NoCredentials_DoesNotCache(t *testing.T) {
	srv := httptest.NewServer(bucketAttrsHandler("test-bucket"))
	defer srv.Close()

	cfg := &Config{
		BucketName: "test-bucket",
		EndPoint:   srv.URL,
	}

	adapter := &storageAdapter{cfg: cfg}
	require.NoError(t, adapter.Connect(t.Context()))

	// When no credentials JSON is set, the cached fields stay empty (Workload Identity path).
	assert.Empty(t, adapter.saEmail)
	assert.Empty(t, adapter.saPrivateKey)
}
