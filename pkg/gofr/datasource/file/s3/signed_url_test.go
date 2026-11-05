package s3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/datasource/file"
)

func Test_validateSignedURLInput(t *testing.T) {
	tests := []struct {
		name    string
		object  string
		expiry  time.Duration
		opts    *file.FileOptions
		wantErr error
	}{
		{name: "valid without opts", object: "a/b.txt", expiry: time.Hour, wantErr: nil},
		{name: "valid with content type", object: "a/b.txt", expiry: time.Hour,
			opts: &file.FileOptions{ContentType: "text/csv"}, wantErr: nil},
		{name: "empty name", object: "", expiry: time.Hour, wantErr: errEmptyObjectName},
		{name: "zero expiry", object: "a.txt", expiry: 0, wantErr: errExpiryMustBePositive},
		{name: "negative expiry", object: "a.txt", expiry: -time.Second, wantErr: errExpiryMustBePositive},
		{name: "invalid content type - no slash", object: "a.txt", expiry: time.Hour,
			opts: &file.FileOptions{ContentType: "textcsv"}, wantErr: errInvalidContentType},
		{name: "invalid content type - empty subtype", object: "a.txt", expiry: time.Hour,
			opts: &file.FileOptions{ContentType: "text/"}, wantErr: errInvalidContentType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSignedURLInput(tt.object, tt.expiry, tt.opts)

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func Test_sanitizeContentDisposition(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "clean value unchanged", input: `attachment; filename="report.csv"`, want: `attachment; filename="report.csv"`},
		{name: "strips CRLF injection", input: "attachment\r\nX-Evil: 1", want: "attachmentX-Evil: 1"},
		{name: "strips CR", input: "attachment\rfoo", want: "attachmentfoo"},
		{name: "strips LF", input: "attachment\nfoo", want: "attachmentfoo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeContentDisposition(tt.input))
		})
	}
}

func Test_buildPresignGetObjectInput(t *testing.T) {
	t.Run("nil opts sets only bucket and key", func(t *testing.T) {
		in := buildPresignGetObjectInput("bucket", "dir/obj.txt", nil)

		assert.Equal(t, "bucket", *in.Bucket)
		assert.Equal(t, "dir/obj.txt", *in.Key)
		assert.Nil(t, in.ResponseContentType)
		assert.Nil(t, in.ResponseContentDisposition)
	})

	t.Run("opts map to response overrides and sanitize disposition", func(t *testing.T) {
		in := buildPresignGetObjectInput("bucket", "obj.txt", &file.FileOptions{
			ContentType:        "text/csv",
			ContentDisposition: "attachment\r\n",
		})

		require.NotNil(t, in.ResponseContentType)
		require.NotNil(t, in.ResponseContentDisposition)
		assert.Equal(t, "text/csv", *in.ResponseContentType)
		assert.Equal(t, "attachment", *in.ResponseContentDisposition)
	})
}
