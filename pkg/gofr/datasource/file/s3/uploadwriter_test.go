package s3

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource/file"
)

const testPartSize = 4

// multipartFake answers the multipart calls and tells which one each request was.
func multipartFake(t *testing.T, failPart int, failComplete bool) *fakeS3 {
	t.Helper()

	return newFakeS3(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		switch {
		case r.Method == http.MethodPost && q.Has("uploads"):
			_, _ = fmt.Fprint(w, `<InitiateMultipartUploadResult><UploadId>u1</UploadId></InitiateMultipartUploadResult>`)
		case r.Method == http.MethodPut && q.Has("partNumber"):
			answerPart(w, q.Get("partNumber"), failPart)
		case r.Method == http.MethodPost && q.Has("uploadId"):
			answerComplete(w, failComplete)
		case r.Method == http.MethodDelete && q.Has("uploadId"):
			w.WriteHeader(http.StatusNoContent)
		}
	})
}

func answerComplete(w http.ResponseWriter, fail bool) {
	if fail {
		writeS3Error(w, http.StatusForbidden, "AccessDenied")
		return
	}

	_, _ = fmt.Fprint(w, `<CompleteMultipartUploadResult><ETag>"done"</ETag></CompleteMultipartUploadResult>`)
}

func answerPart(w http.ResponseWriter, number string, failPart int) {
	n, _ := strconv.Atoi(number)
	if n == failPart {
		writeS3Error(w, http.StatusForbidden, "AccessDenied")
		return
	}

	w.Header().Set("ETag", fmt.Sprintf(`"etag-%d"`, n))
}

// sequence labels each recorded request: PUT, CREATE, PART n, COMPLETE or ABORT.
func sequence(reqs []recordedRequest) []string {
	out := make([]string, 0, len(reqs))

	for _, r := range reqs {
		switch {
		case r.Method == http.MethodPost && r.Query.Has("uploads"):
			out = append(out, "CREATE")
		case r.Method == http.MethodPut && r.Query.Has("partNumber"):
			out = append(out, "PART "+r.Query.Get("partNumber"))
		case r.Method == http.MethodPost:
			out = append(out, "COMPLETE")
		case r.Method == http.MethodDelete:
			out = append(out, "ABORT")
		default:
			out = append(out, r.Method)
		}
	}

	return out
}

func TestUploadWriter(t *testing.T) {
	tests := []struct {
		name         string
		payload      string
		failPart     int
		failComplete bool
		wantSeq      []string
		wantParts    string // part bodies joined by "|"
		wantWriteErr bool
		wantCloseErr bool
	}{
		{name: "small write is one PutObject", payload: "abc", wantSeq: []string{"PUT"}},
		{name: "empty file is one PutObject", payload: "", wantSeq: []string{"PUT"}},
		{
			name: "2.5 parts", payload: "0123456789",
			wantSeq:   []string{"CREATE", "PART 1", "PART 2", "PART 3", "COMPLETE"},
			wantParts: "0123|4567|89",
		},
		{
			name: "exact multiple sends no empty trailing part", payload: "01234567",
			wantSeq:   []string{"CREATE", "PART 1", "PART 2", "COMPLETE"},
			wantParts: "0123|4567",
		},
		{
			name: "part failure aborts and surfaces from Write", payload: "0123456789", failPart: 2,
			wantSeq:      []string{"CREATE", "PART 1", "PART 2", "ABORT"},
			wantWriteErr: true, wantCloseErr: true,
		},
		{
			name: "last part failure aborts and surfaces from Close", payload: "012345", failPart: 2,
			wantSeq:      []string{"CREATE", "PART 1", "PART 2", "ABORT"},
			wantCloseErr: true,
		},
		{
			name: "complete failure aborts", payload: "01234567", failComplete: true,
			wantSeq:      []string{"CREATE", "PART 1", "PART 2", "COMPLETE", "ABORT"},
			wantCloseErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := multipartFake(t, tt.failPart, tt.failComplete)
			a := newTestAdapter(srv)
			a.partSize = testPartSize

			w := a.NewWriterWithOptions(context.Background(), "dir/a.txt",
				&file.FileOptions{ContentType: "text/csv", Metadata: map[string]string{"k": "v"}})

			_, writeErr := w.Write([]byte(tt.payload))
			closeErr := w.Close()

			assert.Equal(t, tt.wantWriteErr, writeErr != nil)
			assert.Equal(t, tt.wantCloseErr, closeErr != nil)
			assert.Equal(t, tt.wantSeq, sequence(srv.requests()))

			if tt.wantWriteErr {
				_, again := w.Write([]byte("x"))
				require.ErrorIs(t, again, os.ErrClosed)
			}

			if tt.wantParts == "" {
				return
			}

			var bodies, etags []string

			for _, r := range srv.requests() {
				switch {
				case r.Method == http.MethodPost && r.Query.Has("uploads"):
					assert.Equal(t, "text/csv", r.Header.Get("Content-Type"))
					assert.Equal(t, "v", r.Header.Get("X-Amz-Meta-K"))
				case r.Method == http.MethodPut:
					assert.Equal(t, "u1", r.Query.Get("uploadId"))
					bodies = append(bodies, string(r.Body))
				case r.Method == http.MethodPost:
					etags = append(etags, r.Header.Get("Content-Type"))
					assert.Less(t, bytes.Index(r.Body, []byte("etag-1")), bytes.Index(r.Body, []byte("etag-2")))
				}
			}

			assert.Equal(t, tt.wantParts, strings.Join(bodies, "|"))
			assert.Equal(t, tt.payload, strings.ReplaceAll(strings.Join(bodies, "|"), "|", ""))
			assert.Len(t, etags, 1)
		})
	}
}

func TestUploadWriter_Close(t *testing.T) {
	tests := []struct {
		name string
		opts *file.FileOptions
	}{
		{name: "nil options"},
		{name: "with options", opts: &file.FileOptions{ContentType: "text/csv"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := multipartFake(t, 0, false)
			a := newTestAdapter(srv)

			w := a.NewWriterWithOptions(context.Background(), "a.csv", tt.opts)

			_, err := w.Write([]byte("abc"))
			require.NoError(t, err)

			require.NoError(t, w.Close())
			require.NoError(t, w.Close(), "a second Close returns nil after a successful first")

			_, err = w.Write([]byte("x"))
			require.ErrorIs(t, err, os.ErrClosed)

			assert.Equal(t, []string{"PUT"}, sequence(srv.requests()))
		})
	}
}
