package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
	resTypes "gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/internal/errhide"
)

var errDriverText = errors.New(`dial tcp 10.0.3.7:5432: connect: connection refused`)

// hideErrors turns HIDE_INTERNAL_ERRORS on or off for one test and restores the previous value.
// The switch is process-wide, so tests that use it must not run in parallel.
func hideErrors(t *testing.T, on bool) {
	t.Helper()

	prev := errhide.Enabled()

	errhide.Enable(on)
	t.Cleanup(func() { errhide.Enable(prev) })
}

func TestResponder_HideInternalErrors(t *testing.T) {
	custom500 := &CustomError{Code: http.StatusInternalServerError, Message: "payment provider is down, retry later", Title: "Upstream"}

	tests := []struct {
		desc       string
		data       any
		err        error
		wantStatus int
		wantBody   string
	}{
		{"raw error is hidden", nil, errDriverText, http.StatusInternalServerError,
			`{"error":{"message":"Internal Server Error"}}`},
		{"wrapped raw error is hidden", nil, fmt.Errorf("load user: %w", errDriverText), http.StatusInternalServerError,
			`{"error":{"message":"Internal Server Error"}}`},
		{"ErrorDB is hidden", nil, datasource.ErrorDB{Err: errDriverText, Message: "select failed"}, http.StatusInternalServerError,
			`{"error":{"message":"Internal Server Error"}}`},
		{"custom 500 keeps its message and fields", nil, custom500, http.StatusInternalServerError,
			`{"error":{"message":"payment provider is down, retry later","title":"Upstream","code":500}}`},
		{"custom 404 with driver text is kept", nil, &CustomError{Code: http.StatusNotFound, Message: errDriverText.Error(), Title: "Missing"},
			http.StatusNotFound,
			`{"error":{"message":"dial tcp 10.0.3.7:5432: connect: connection refused","title":"Missing","code":404}}`},
		{"206 with data hides the message", map[string]string{"id": "1"}, errDriverText, http.StatusPartialContent,
			`{"error":{"message":"Internal Server Error"},"data":{"id":"1"}}`},
	}

	hideErrors(t, true)

	for _, tc := range tests {
		w := httptest.NewRecorder()

		NewResponder(w, http.MethodGet).Respond(tc.data, tc.err)

		assert.Equal(t, tc.wantStatus, w.Code, tc.desc)
		assert.JSONEq(t, tc.wantBody, w.Body.String(), tc.desc)
	}
}

func TestResponder_HideInternalErrors_Off(t *testing.T) {
	hideErrors(t, false)

	w := httptest.NewRecorder()

	NewResponder(w, http.MethodGet).Respond(nil, errDriverText)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":{"message":"dial tcp 10.0.3.7:5432: connect: connection refused"}}`, w.Body.String())
}

func TestStream_HideInternalErrors(t *testing.T) {
	tests := []struct {
		desc   string
		on     bool
		format resTypes.Format
		err    error
		want   string
	}{
		{"on: SSE source error is hidden", true, resTypes.SSE, errDriverText,
			"data: \"a\"\n\nevent: error\ndata: {\"error\":\"Internal Server Error\"}\n\n"},
		{"on: NDJSON source error is hidden", true, resTypes.NDJSON, errDriverText,
			"\"a\"\n{\"error\":\"Internal Server Error\"}\n"},
		{"on: error with a status code is kept", true, resTypes.SSE, ErrorEntityNotFound{Name: "id", Value: "1"},
			"data: \"a\"\n\nevent: error\ndata: {\"error\":\"No entity found with id: 1\"}\n\n"},
		{"off: source error is unchanged", false, resTypes.SSE, errDriverText,
			"data: \"a\"\n\nevent: error\ndata: {\"error\":\"dial tcp 10.0.3.7:5432: connect: connection refused\"}\n\n"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			hideErrors(t, tc.on)

			rec := httptest.NewRecorder()
			src := &sliceSource{items: []any{"a"}, err: tc.err}

			NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: src, Format: tc.format}, nil)

			assert.Equal(t, tc.want, rec.Body.String())
		})
	}
}

func TestStream_HideInternalErrors_Panic(t *testing.T) {
	hideErrors(t, true)

	rec := httptest.NewRecorder()

	NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: &panicSource{}}, nil)

	assert.Contains(t, rec.Body.String(), `{"error":"Internal Server Error"}`)
	assert.NotContains(t, rec.Body.String(), "boom")
}
