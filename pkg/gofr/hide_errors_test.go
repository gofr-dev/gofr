package gofr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/internal/errhide"
	"gofr.dev/pkg/gofr/testutil"
)

// TestNew_HideInternalErrorsIsProcessWide pins that the switch is process-wide: a later app that
// leaves HIDE_INTERNAL_ERRORS off must not turn hiding off for an earlier app that turned it on.
func TestNew_HideInternalErrorsIsProcessWide(t *testing.T) {
	hideErrors(t, false)
	testutil.NewServerConfigs(t)

	t.Setenv("HIDE_INTERNAL_ERRORS", "true")

	first := New()

	t.Setenv("HIDE_INTERNAL_ERRORS", "false")

	second := New()

	assert.True(t, errhide.Enabled(), "the second app must not reset the switch")

	for name, app := range map[string]*App{"first": first, "second": second} {
		app.GET("/fail", func(*Context) (any, error) { return nil, errDriverDetail })

		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/fail", http.NoBody)

		app.httpServer.router.ServeHTTP(w, r)

		assert.JSONEq(t, `{"error":{"message":"Internal Server Error"}}`, w.Body.String(), name)
	}
}
