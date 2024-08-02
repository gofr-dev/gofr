package datasource

import (
	"net/http"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ErrorDB(t *testing.T) {
	wrappedErr := errors.New("underlying error")

	tests := []struct {
		desc        string
		err         ErrorDB
		expectedMsg string
	}{
		{"wrapped error", ErrorDB{Err: wrappedErr, Message: "custom message"}.WithStack(), "custom message: underlying error"},
		{"without wrapped error", ErrorDB{Message: "custom message"}, "custom message"},
		{"no custom error message", ErrorDB{Err: wrappedErr}, "underlying error"},
	}

	for i, tc := range tests {
		require.ErrorContains(t, tc.err, tc.expectedMsg, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestErrorDB_StatusCode(t *testing.T) {
	dbErr := ErrorDB{Message: "custom message"}

	expectedCode := http.StatusInternalServerError

	assert.Equal(t, expectedCode, dbErr.StatusCode(), "TEST Failed.\n")
}
