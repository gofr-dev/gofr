package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/internal/errhide"
)

var errKeyLookup = errors.New(`pq: password authentication failed for user "auth_ro"`)

// lookupAuthProvider fails the way a custom provider that checks keys in a database would.
type lookupAuthProvider struct{ err ErrorHTTP }

func (lookupAuthProvider) GetAuthMethod() AuthMethod { return APIKey }

func (p lookupAuthProvider) ExtractAuthHeader(*http.Request) (any, ErrorHTTP) { return nil, p.err }

// TestAuthMiddleware_HideInternalErrors checks that an error written by middleware, not a handler,
// goes through the same HIDE_INTERNAL_ERRORS rule.
func TestAuthMiddleware_HideInternalErrors(t *testing.T) {
	tests := []struct {
		desc string
		on   bool
		err  ErrorHTTP
		want string
	}{
		{"on: ErrorDB from a key lookup is hidden", true, datasource.ErrorDB{Err: errKeyLookup},
			`{"error":{"message":"Internal Server Error"}}`},
		{"on: GoFr auth error keeps its message", true, ErrorInvalidAuthorizationHeader{key: headerAuthorization},
			`{"error":{"message":"invalid auth header in key 'Authorization'"}}`},
		{"off: ErrorDB from a key lookup is unchanged", false, datasource.ErrorDB{Err: errKeyLookup},
			`{"error":{"message":"pq: password authentication failed for user \"auth_ro\""}}`},
	}

	prev := errhide.Enabled()

	t.Cleanup(func() { errhide.Enable(prev) })

	for _, tc := range tests {
		errhide.Enable(tc.on)

		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/orders", http.NoBody)

		AuthMiddleware(lookupAuthProvider{err: tc.err})(http.NotFoundHandler()).ServeHTTP(w, r)

		assert.Equal(t, tc.err.StatusCode(), w.Code, tc.desc)
		assert.JSONEq(t, tc.want, w.Body.String(), tc.desc)
	}
}

type captureLogger struct{ lines []string }

func (c *captureLogger) Errorf(format string, args ...any) {
	c.lines = append(c.lines, fmt.Sprintf(format, args...))
}

func TestAuthMiddleware_HideInternalErrors_Logged(t *testing.T) {
	logger := &captureLogger{}

	errhide.SetLogger(logger)

	prev := errhide.Enabled()

	t.Cleanup(func() {
		errhide.SetLogger(nil)
		errhide.Enable(prev)
	})

	errhide.Enable(true)

	serve := func(err ErrorHTTP) {
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/orders", http.NoBody)

		AuthMiddleware(lookupAuthProvider{err: err})(http.NotFoundHandler()).ServeHTTP(w, r)
	}

	serve(ErrorInvalidAuthorizationHeader{key: headerAuthorization})
	assert.Empty(t, logger.lines, "an error that is not hidden is not logged")

	serve(datasource.ErrorDB{Err: errKeyLookup})

	if assert.Len(t, logger.lines, 1) {
		assert.Contains(t, logger.lines[0], errKeyLookup.Error())
	}
}
