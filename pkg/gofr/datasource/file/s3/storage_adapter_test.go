package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource/file"
)

const testBucket = "bkt"

// recordedRequest is one request the fake S3 server received.
type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   []byte
}

// fakeS3 is an httptest server that records every request and delegates the response to handler.
type fakeS3 struct {
	*httptest.Server

	mu   sync.Mutex
	reqs []recordedRequest
}

func newFakeS3(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *fakeS3 {
	t.Helper()

	f := &fakeS3{}

	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))

		f.mu.Lock()
		f.reqs = append(f.reqs, recordedRequest{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Header: r.Header.Clone(), Body: body,
		})
		f.mu.Unlock()

		handler(w, r)
	}))

	t.Cleanup(f.Close)

	return f
}

func (f *fakeS3) requests() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]recordedRequest(nil), f.reqs...)
}

// newTestClient builds a real S3 client pointed at the fake server.
func newTestClient(endpoint string, pathStyle bool) *s3.Client {
	cfg := aws.Config{
		Region:           "us-east-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKID", "secret", ""),
		RetryMaxAttempts: 1,
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = pathStyle
	})
}

// newTestAdapter returns a connected adapter whose client talks to the fake server.
func newTestAdapter(srv *fakeS3) *storageAdapter {
	c := newTestClient(srv.URL, true)

	return &storageAdapter{
		cfg:       &Config{BucketName: testBucket, Region: "us-east-1", EndPoint: srv.URL},
		client:    c,
		presigner: s3.NewPresignClient(c),
	}
}

func writeS3Error(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, code)
}

func TestStorageAdapter_Connect(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "bucket reachable", status: http.StatusOK},
		{name: "bucket forbidden", status: http.StatusForbidden, wantErr: true},
		{name: "bucket missing", status: http.StatusNotFound, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tt.status) })

			a := &storageAdapter{cfg: &Config{
				EndPoint: srv.URL, BucketName: testBucket, Region: "us-east-1",
				AccessKeyID: "AKID", SecretAccessKey: "secret", Flavor: FlavorMinIO,
			}}

			err := a.Connect(context.Background())

			reqs := srv.requests()
			require.NotEmpty(t, reqs)
			assert.Equal(t, http.MethodHead, reqs[0].Method)
			assert.Equal(t, "/"+testBucket, reqs[0].Path)

			if tt.wantErr {
				require.Error(t, err)

				// A failed Connect leaves the adapter unusable with a clear error.
				_, readErr := a.NewReader(context.Background(), "a.txt")
				require.ErrorIs(t, readErr, errS3ClientNotInitialized)

				return
			}

			require.NoError(t, err)

			// Connect on a connected adapter is a no-op.
			require.NoError(t, a.Connect(context.Background()))
			assert.Len(t, srv.requests(), 1)
		})
	}
}

func TestStorageAdapter_Connect_NilConfig(t *testing.T) {
	err := (&storageAdapter{}).Connect(context.Background())

	require.ErrorIs(t, err, errS3ConfigNil)
}

func TestStorageAdapter_NotConnected(t *testing.T) {
	a := &storageAdapter{cfg: &Config{BucketName: testBucket}}
	ctx := context.Background()

	_, err := a.NewReader(ctx, "a")
	require.ErrorIs(t, err, errS3ClientNotInitialized)

	_, err = a.StatObject(ctx, "a")
	require.ErrorIs(t, err, errS3ClientNotInitialized)

	require.ErrorIs(t, a.DeleteObject(ctx, "a"), errS3ClientNotInitialized)
	require.ErrorIs(t, a.CopyObject(ctx, "a", "b"), errS3ClientNotInitialized)

	_, err = a.ListObjects(ctx, "")
	require.ErrorIs(t, err, errS3ClientNotInitialized)

	_, _, err = a.ListDir(ctx, "")
	require.ErrorIs(t, err, errS3ClientNotInitialized)

	require.ErrorIs(t, a.NewWriter(ctx, "a").Close(), errS3ClientNotInitialized)
}

func TestStorageAdapter_StatObject(t *testing.T) {
	modified := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name    string
		key     string
		handler func(w http.ResponseWriter, r *http.Request)
		want    file.ObjectInfo
		wantErr error
	}{
		{
			name: "existing object",
			key:  "dir/a.txt",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "5")
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
			},
			want: file.ObjectInfo{Name: "dir/a.txt", Size: 5, ContentType: "text/plain", LastModified: modified},
		},
		{
			name:    "directory marker",
			key:     "dir/",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Length", "0") },
			want:    file.ObjectInfo{Name: "dir/", IsDir: true},
		},
		{
			name:    "missing Content-Length gives size 0",
			key:     "r2.bin",
			handler: func(http.ResponseWriter, *http.Request) {},
			want:    file.ObjectInfo{Name: "r2.bin"},
		},
		{
			name:    "missing object",
			key:     "none.txt",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
			wantErr: os.ErrNotExist,
		},
		{
			name:    "empty name",
			key:     "",
			handler: func(http.ResponseWriter, *http.Request) {},
			wantErr: errEmptyObjectName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAdapter(newFakeS3(t, tt.handler))

			got, err := a.StatObject(context.Background(), tt.key)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.Size, got.Size)
			assert.Equal(t, tt.want.ContentType, got.ContentType)
			assert.Equal(t, tt.want.IsDir, got.IsDir)
			assert.True(t, tt.want.LastModified.Equal(got.LastModified))
		})
	}
}

func TestStorageAdapter_NewReader(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		handler  func(w http.ResponseWriter, r *http.Request)
		wantBody string
		wantErr  error
	}{
		{
			name:     "existing object",
			key:      "a.txt",
			handler:  func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hello")) },
			wantBody: "hello",
		},
		{
			name:    "NoSuchKey maps to ErrNotExist",
			key:     "none.txt",
			handler: func(w http.ResponseWriter, _ *http.Request) { writeS3Error(w, http.StatusNotFound, "NoSuchKey") },
			wantErr: os.ErrNotExist,
		},
		{name: "empty name", key: "", handler: func(http.ResponseWriter, *http.Request) {}, wantErr: errEmptyObjectName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeS3(t, tt.handler)
			a := newTestAdapter(srv)

			r, err := a.NewReader(context.Background(), tt.key)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			defer r.Close()

			b, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, string(b))
			assert.Equal(t, "/"+testBucket+"/"+tt.key, srv.requests()[0].Path)
		})
	}
}

func TestStorageAdapter_NewRangeReader(t *testing.T) {
	tests := []struct {
		name       string
		offset     int64
		length     int64
		handler    func(w http.ResponseWriter, r *http.Request)
		wantRange  string
		wantBody   string
		wantErr    error
		wantNoCall bool
	}{
		{
			name: "range is sent and honored", offset: 3, length: 4, wantRange: "bytes=3-6", wantBody: "defg",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Range", "bytes 3-6/10")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte("defg"))
			},
		},
		{
			name: "200 instead of 206 is rejected", offset: 3, length: 4, wantRange: "bytes=3-6", wantErr: ErrRangeNotHonored,
			handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("abcdefghij")) },
		},
		{
			name: "416 reads as empty", offset: 0, length: 4, wantRange: "bytes=0-3", wantBody: "",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeS3Error(w, http.StatusRequestedRangeNotSatisfiable, "InvalidRange")
			},
		},
		{
			name: "missing object", offset: 0, length: 4, wantRange: "bytes=0-3", wantErr: os.ErrNotExist,
			handler: func(w http.ResponseWriter, _ *http.Request) { writeS3Error(w, http.StatusNotFound, "NoSuchKey") },
		},
		{
			name: "negative offset", offset: -1, length: 4, wantErr: errInvalidOffset, wantNoCall: true,
			handler: func(http.ResponseWriter, *http.Request) {},
		},
		{
			name: "zero length reads nothing", offset: 2, length: 0, wantBody: "", wantNoCall: true,
			handler: func(http.ResponseWriter, *http.Request) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeS3(t, tt.handler)
			a := newTestAdapter(srv)

			r, err := a.NewRangeReader(context.Background(), "a.txt", tt.offset, tt.length)

			if tt.wantNoCall {
				assert.Empty(t, srv.requests())
			} else {
				require.NotEmpty(t, srv.requests())
				assert.Equal(t, tt.wantRange, srv.requests()[0].Header.Get("Range"))
			}

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			defer r.Close()

			b, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, string(b))
		})
	}
}

func TestStorageAdapter_ListObjects(t *testing.T) {
	tests := []struct {
		name    string
		pages   []string
		want    []string
		wantErr error
	}{
		{
			name: "paginates past one page",
			pages: []string{
				`<ListBucketResult><IsTruncated>true</IsTruncated><NextContinuationToken>tok1</NextContinuationToken>` +
					`<Contents><Key>p/a</Key></Contents><Contents><Key>p/b</Key></Contents></ListBucketResult>`,
				`<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>p/c</Key></Contents></ListBucketResult>`,
			},
			want: []string{"p/a", "p/b", "p/c"},
		},
		{
			name:  "empty prefix result",
			pages: []string{`<ListBucketResult><IsTruncated>false</IsTruncated></ListBucketResult>`},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int

			srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tt.pages[calls])
				calls++
			})

			got, err := newTestAdapter(srv).ListObjects(context.Background(), "p/")

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			reqs := srv.requests()
			require.Len(t, reqs, len(tt.pages))
			assert.Equal(t, "p/", reqs[0].Query.Get("prefix"))
			assert.Empty(t, reqs[0].Query.Get("delimiter"))

			if len(tt.pages) > 1 {
				assert.Equal(t, "tok1", reqs[1].Query.Get("continuation-token"))
			}
		})
	}
}

func TestStorageAdapter_ListObjects_Error(t *testing.T) {
	srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) { writeS3Error(w, http.StatusForbidden, "AccessDenied") })

	_, err := newTestAdapter(srv).ListObjects(context.Background(), "p/")

	require.Error(t, err)
}

func TestStorageAdapter_ListDir(t *testing.T) {
	page1 := `<ListBucketResult><IsTruncated>true</IsTruncated><NextContinuationToken>tok1</NextContinuationToken>` +
		`<Contents><Key>d/a.txt</Key><Size>3</Size><LastModified>2026-01-02T03:04:05.000Z</LastModified></Contents>` +
		`<CommonPrefixes><Prefix>d/sub1/</Prefix></CommonPrefixes></ListBucketResult>`
	page2 := `<ListBucketResult><IsTruncated>false</IsTruncated>` +
		`<Contents><Key>d/b.txt</Key><Size>4</Size></Contents>` +
		`<CommonPrefixes><Prefix>d/sub2/</Prefix></CommonPrefixes></ListBucketResult>`

	tests := []struct {
		name         string
		pages        []string
		wantObjects  []string
		wantPrefixes []string
	}{
		{name: "objects and prefixes across pages", pages: []string{page1, page2},
			wantObjects: []string{"d/a.txt", "d/b.txt"}, wantPrefixes: []string{"d/sub1/", "d/sub2/"}},
		{name: "single page", pages: []string{page2},
			wantObjects: []string{"d/b.txt"}, wantPrefixes: []string{"d/sub2/"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int

			srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tt.pages[calls])
				calls++
			})

			objects, prefixes, err := newTestAdapter(srv).ListDir(context.Background(), "d/")

			require.NoError(t, err)

			names := make([]string, 0, len(objects))
			for _, o := range objects {
				names = append(names, o.Name)
				assert.False(t, o.IsDir)
			}

			assert.Equal(t, tt.wantObjects, names)
			assert.Equal(t, tt.wantPrefixes, prefixes)

			reqs := srv.requests()
			require.Len(t, reqs, len(tt.pages))
			assert.Equal(t, "/", reqs[0].Query.Get("delimiter"))
			assert.Equal(t, "d/", reqs[0].Query.Get("prefix"))

			if len(tt.pages) > 1 {
				assert.Equal(t, "tok1", reqs[1].Query.Get("continuation-token"))
			}

			if tt.pages[0] == page1 {
				assert.EqualValues(t, 3, objects[0].Size)
				assert.True(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Equal(objects[0].LastModified))
			}
		})
	}
}

func TestStorageAdapter_ListDir_Error(t *testing.T) {
	srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) { writeS3Error(w, http.StatusForbidden, "AccessDenied") })

	_, _, err := newTestAdapter(srv).ListDir(context.Background(), "d/")

	require.Error(t, err)
}

func TestStorageAdapter_CopyObject(t *testing.T) {
	tests := []struct {
		name       string
		src, dst   string
		status     int
		wantSource string
		wantPath   string
		wantErr    error
	}{
		{
			name: "source is URL-escaped", src: "in dir/a b+c.txt", dst: "out/x.txt", status: http.StatusOK,
			wantSource: testBucket + "/in%20dir/a%20b%2Bc.txt", wantPath: "/" + testBucket + "/out/x.txt",
		},
		{name: "missing source", src: "none", dst: "x", status: http.StatusNotFound, wantErr: os.ErrNotExist},
		{name: "empty names", src: "", dst: "x", wantErr: errEmptySourceOrDest},
		{name: "same source and destination", src: "a", dst: "a", wantErr: errSameSourceAndDest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) {
				if tt.status == http.StatusNotFound {
					writeS3Error(w, tt.status, "NoSuchKey")
					return
				}

				_, _ = io.WriteString(w, `<CopyObjectResult><ETag>"x"</ETag></CopyObjectResult>`)
			})

			err := newTestAdapter(srv).CopyObject(context.Background(), tt.src, tt.dst)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			req := srv.requests()[0]
			assert.Equal(t, http.MethodPut, req.Method)
			assert.Equal(t, tt.wantPath, req.Path)
			assert.Equal(t, tt.wantSource, req.Header.Get("X-Amz-Copy-Source"))
		})
	}
}

func TestStorageAdapter_DeleteObject(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		status  int
		wantErr bool
	}{
		{name: "deletes", key: "d/a.txt", status: http.StatusNoContent},
		{name: "empty name", key: "", wantErr: true},
		{name: "access denied", key: "a", status: http.StatusForbidden, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) {
				if tt.status == http.StatusForbidden {
					writeS3Error(w, tt.status, "AccessDenied")
					return
				}

				w.WriteHeader(tt.status)
			})

			err := newTestAdapter(srv).DeleteObject(context.Background(), tt.key)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			req := srv.requests()[0]
			assert.Equal(t, http.MethodDelete, req.Method)
			assert.Equal(t, "/"+testBucket+"/"+tt.key, req.Path)
		})
	}
}

func TestStorageAdapter_NewWriter(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		body            string
		status          int
		wantContentType string
		wantErr         bool
	}{
		{name: "txt gets text/plain", key: "a/b.txt", body: "hello", status: http.StatusOK, wantContentType: "text/plain; charset=utf-8"},
		{name: "json gets application/json", key: "a.json", body: `{"a":1}`, status: http.StatusOK, wantContentType: "application/json"},
		{name: "empty body (directory marker)", key: "d/", body: "", status: http.StatusOK, wantContentType: "application/octet-stream"},
		{name: "upload failure surfaces on Close", key: "a.txt", body: "x", status: http.StatusForbidden, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) {
				if tt.status != http.StatusOK {
					writeS3Error(w, tt.status, "AccessDenied")
					return
				}
			})

			w := newTestAdapter(srv).NewWriter(context.Background(), tt.key)

			n, err := w.Write([]byte(tt.body))
			require.NoError(t, err)
			assert.Equal(t, len(tt.body), n)

			// Nothing is uploaded until Close.
			assert.Empty(t, srv.requests())

			err = w.Close()

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			reqs := srv.requests()
			require.Len(t, reqs, 1)
			assert.Equal(t, http.MethodPut, reqs[0].Method)
			assert.Equal(t, "/"+testBucket+"/"+tt.key, reqs[0].Path)
			assert.Equal(t, tt.body, string(reqs[0].Body))
			assert.Equal(t, tt.wantContentType, reqs[0].Header.Get("Content-Type"))
			assert.Equal(t, "attachment", reqs[0].Header.Get("Content-Disposition"))
		})
	}
}

func TestStorageAdapter_NewWriter_EmptyName(t *testing.T) {
	w := newTestAdapter(newFakeS3(t, func(http.ResponseWriter, *http.Request) {})).NewWriter(context.Background(), "")

	_, err := w.Write([]byte("x"))
	require.ErrorIs(t, err, errEmptyObjectName)
	require.ErrorIs(t, w.Close(), errEmptyObjectName)
}

func TestStorageAdapter_NewWriterWithOptions(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		opts            *file.FileOptions
		wantContentType string
		wantDisp        string
		wantMeta        map[string]string
		wantErr         error
	}{
		{
			name: "nil options (3805 regression)", key: "a.txt", opts: nil,
			wantContentType: "text/plain; charset=utf-8", wantDisp: "attachment",
		},
		{
			name: "all options", key: "a.txt",
			opts: &file.FileOptions{
				ContentType: "application/pdf", ContentDisposition: "inline; filename=\"a.pdf\"\r\n",
				Metadata: map[string]string{"owner": "me"},
			},
			wantContentType: "application/pdf", wantDisp: "inline; filename=\"a.pdf\"", wantMeta: map[string]string{"Owner": "me"},
		},
		{
			name: "empty options keep defaults", key: "a.json", opts: &file.FileOptions{},
			wantContentType: "application/json", wantDisp: "attachment",
		},
		{
			name: "malformed content type is rejected", key: "a.txt", opts: &file.FileOptions{ContentType: "bogus"},
			wantErr: errInvalidContentType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeS3(t, func(http.ResponseWriter, *http.Request) {})

			w := newTestAdapter(srv).NewWriterWithOptions(context.Background(), tt.key, tt.opts)

			_, writeErr := w.Write([]byte("data"))
			closeErr := w.Close()

			if tt.wantErr != nil {
				require.ErrorIs(t, closeErr, tt.wantErr)
				assert.Empty(t, srv.requests())

				return
			}

			require.NoError(t, writeErr)
			require.NoError(t, closeErr)

			req := srv.requests()[0]
			assert.Equal(t, "data", string(req.Body))
			assert.Equal(t, tt.wantContentType, req.Header.Get("Content-Type"))
			assert.Equal(t, tt.wantDisp, req.Header.Get("Content-Disposition"))

			for k, v := range tt.wantMeta {
				assert.Equal(t, v, req.Header.Get("X-Amz-Meta-"+k))
			}
		})
	}
}

func TestStorageAdapter_SignedURL(t *testing.T) {
	tests := []struct {
		name       string
		pathStyle  bool
		bucket     string
		expiry     time.Duration
		opts       *file.FileOptions
		wantPrefix string
		wantParams []string
		wantErr    error
	}{
		{
			name: "path style host", pathStyle: true, bucket: testBucket, expiry: 15 * time.Minute,
			opts:       &file.FileOptions{ContentDisposition: `attachment; filename="q1.csv"`},
			wantPrefix: "http://s3.example.com/" + testBucket + "/reports/q1.csv",
			wantParams: []string{"X-Amz-Signature=", "X-Amz-Expires=900", "response-content-disposition="},
		},
		{
			name: "virtual host style", pathStyle: false, bucket: testBucket, expiry: time.Hour,
			wantPrefix: "http://" + testBucket + ".s3.example.com/reports/q1.csv",
			wantParams: []string{"X-Amz-Signature=", "X-Amz-Expires=3600"},
		},
		{name: "no bucket", pathStyle: true, bucket: "", expiry: time.Hour, wantErr: errBucketNotConfigured},
		{name: "non-positive expiry", pathStyle: true, bucket: testBucket, expiry: 0, wantErr: errExpiryMustBePositive},
		{
			name: "bad content type", pathStyle: true, bucket: testBucket, expiry: time.Hour,
			opts: &file.FileOptions{ContentType: "bogus"}, wantErr: errInvalidContentType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient("http://s3.example.com", tt.pathStyle)
			a := &storageAdapter{cfg: &Config{BucketName: tt.bucket}, client: c, presigner: s3.NewPresignClient(c)}

			got, err := a.SignedURL(context.Background(), "reports/q1.csv", tt.expiry, tt.opts)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(got, tt.wantPrefix), "got %s", got)

			for _, p := range tt.wantParams {
				assert.Contains(t, got, p)
			}
		})
	}
}

func TestStorageAdapter_SignedURL_Preconditions(t *testing.T) {
	tests := []struct {
		name    string
		adapter *storageAdapter
		ctx     func() context.Context
		wantErr error
	}{
		{
			name: "not connected", adapter: &storageAdapter{cfg: &Config{BucketName: testBucket}},
			ctx: context.Background, wantErr: errS3ClientNotInitialized,
		},
		{
			name: "canceled context", adapter: &storageAdapter{cfg: &Config{BucketName: testBucket}},
			ctx: func() context.Context {
				c, cancel := context.WithCancel(context.Background())
				cancel()

				return c
			},
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.adapter.SignedURL(tt.ctx(), "a", time.Hour, nil)

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func Test_applyClientOptions_Endpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantNil  bool
	}{
		{name: "empty endpoint leaves BaseEndpoint nil so AWS resolves its regional endpoint", endpoint: "", wantNil: true},
		{name: "custom endpoint is set", endpoint: "http://localhost:9000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &s3.Options{}
			applyClientOptions(resolveProfile(&Config{}), tt.endpoint)(o)

			if tt.wantNil {
				assert.Nil(t, o.BaseEndpoint)
				return
			}

			require.NotNil(t, o.BaseEndpoint)
			assert.Equal(t, tt.endpoint, *o.BaseEndpoint)
		})
	}
}
