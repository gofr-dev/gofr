package datasource

import (
	"database/sql"
	"github.com/gocql/gocql"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

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
	testCases := []struct {
		message    string
		err        error
		statusCode int
	}{
		{"custom message", errors.New("some error"), http.StatusInternalServerError},
		{"", nil, http.StatusInternalServerError},
		{"custom message", sql.ErrNoRows, http.StatusNotFound},
		{"custom message", gocql.ErrNotFound, http.StatusNotFound},
		{"custom message", mongo.ErrNoDocuments, http.StatusNotFound},
	}
	for i, testCase := range testCases {
		errorDB := ErrorDB{
			Err:     testCase.err,
			Message: testCase.message,
		}
		assert.Equal(t, testCase.statusCode, errorDB.StatusCode(), "Failed test case #%d", i)
	}
}
