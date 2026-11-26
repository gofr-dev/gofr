package errhide_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/internal/errhide"
)

var errDriver = errors.New(`pq: relation "users" does not exist`)

type statusError struct {
	msg  string
	code int
}

func (e statusError) Error() string   { return e.msg }
func (e statusError) StatusCode() int { return e.code }

func TestHides(t *testing.T) {
	tests := []struct {
		desc string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"raw error", errDriver, true},
		{"wrapped raw error", fmt.Errorf("load user: %w", errDriver), true},
		{"ErrorDB", datasource.ErrorDB{Err: errDriver, Message: "select failed"}, true},
		{"pointer to ErrorDB", &datasource.ErrorDB{Err: errDriver}, true},
		{"custom 500", statusError{"payment provider is down, retry later", http.StatusInternalServerError}, false},
		{"custom 404 carrying driver text", statusError{errDriver.Error(), http.StatusNotFound}, false},
		{"GoFr entity not found", gofrHTTP.ErrorEntityNotFound{Name: "id", Value: "1"}, false},
		{"wrapped error with a status code", fmt.Errorf("load user: %w", gofrHTTP.ErrorEntityNotFound{Name: "id", Value: "1"}), true},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, errhide.Hides(tc.err), tc.desc)
	}
}

func TestApply(t *testing.T) {
	custom := statusError{"payment provider is down, retry later", http.StatusInternalServerError}

	errhide.Enable(false)
	t.Cleanup(func() { errhide.Enable(false) })

	assert.Equal(t, errDriver.Error(), errhide.Apply(errDriver), "off: raw error text is kept")

	errhide.Enable(true)

	assert.True(t, errhide.Enabled())
	assert.Equal(t, errhide.Message, errhide.Apply(errDriver), "on: raw error text is hidden")
	assert.Equal(t, custom.Error(), errhide.Apply(custom), "on: a custom 500 keeps its text")
}
