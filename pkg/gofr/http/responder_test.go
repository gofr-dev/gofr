package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

func TestResponder_Respond(t *testing.T) {
	r := NewResponder(httptest.NewRecorder())

	tests := []struct {
		desc        string
		data        interface{}
		contentType string
	}{
		{"raw response type", resTypes.Raw{}, "application/json"},
		{"file response type", resTypes.File{ContentType: "image/png"}, "image/png"},
		{"map response type", map[string]string{}, "application/json"},
	}

	for i, tc := range tests {
		r.Respond(tc.data, nil)
		contentType := r.w.Header().Get("Content-Type")

		assert.Equal(t, tc.contentType, contentType, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestResponder_HTTPStatusFromError(t *testing.T) {
	r := NewResponder(httptest.NewRecorder())

	tests := []struct {
		desc       string
		input      error
		statusCode int
		errObj     interface{}
	}{
		{"success case", nil, http.StatusOK, nil},
		{"file not found", http.ErrMissingFile, http.StatusNotFound, map[string]interface{}{
			"message": http.ErrMissingFile.Error()}},
		{"internal server error", http.ErrHandlerTimeout, http.StatusInternalServerError,
			map[string]interface{}{"message": http.ErrHandlerTimeout.Error()}},
	}

	for i, tc := range tests {
		statusCode, errObj := r.HTTPStatusFromError(tc.input)

		assert.Equal(t, tc.statusCode, statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.errObj, errObj, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestPostResponder_Respond(t *testing.T) {
	const expStatusCode = "201 Created"

	w := httptest.NewRecorder()
	r := NewPostResponder(w)

	tests := []struct {
		desc        string
		data        interface{}
		contentType string
	}{
		{"raw response type", resTypes.Raw{}, "application/json"},
		{"file response type", resTypes.File{ContentType: "image/png"}, "image/png"},
		{"map response type", map[string]string{}, "application/json"},
	}

	for i, tc := range tests {
		r.Respond(tc.data, nil)

		contentType := w.Header().Get("Content-Type")
		result := w.Result()

		assert.Equal(t, expStatusCode, result.Status, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.contentType, contentType, "TEST[%d], Failed.\n%s", i, tc.desc)

		result.Body.Close()
	}
}

func TestPostResponder_HTTPStatusFromError(t *testing.T) {
	r := NewPostResponder(httptest.NewRecorder())

	tests := []struct {
		desc       string
		input      error
		statusCode int
		errObj     interface{}
	}{
		{"success case", nil, http.StatusCreated, nil},
		{"file not found", http.ErrMissingFile, http.StatusNotFound, map[string]interface{}{
			"message": http.ErrMissingFile.Error()}},
		{"internal server error", http.ErrHandlerTimeout, http.StatusInternalServerError,
			map[string]interface{}{"message": http.ErrHandlerTimeout.Error()}},
	}

	for i, tc := range tests {
		statusCode, errObj := r.HTTPStatusFromError(tc.input)

		assert.Equal(t, tc.statusCode, statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.errObj, errObj, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestDeleteResponder_Respond(t *testing.T) {
	const expStatusCode = "204 No Content"

	w := httptest.NewRecorder()
	r := NewDeleteResponder(w)

	tests := []struct {
		desc        string
		data        interface{}
		contentType string
	}{
		{"raw response type", resTypes.Raw{}, "application/json"},
		{"file response type", resTypes.File{ContentType: "image/png"}, "image/png"},
		{"map response type", map[string]string{}, "application/json"},
	}

	for i, tc := range tests {
		r.Respond(tc.data, nil)

		contentType := w.Header().Get("Content-Type")
		result := w.Result()

		assert.Equal(t, expStatusCode, result.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.contentType, contentType, "TEST[%d], Failed.\n%s", i, tc.desc)

		result.Body.Close()
	}
}

func TestDeleteResponder_HTTPStatusFromError(t *testing.T) {
	r := NewDeleteResponder(httptest.NewRecorder())

	tests := []struct {
		desc       string
		input      error
		statusCode int
		errObj     interface{}
	}{
		{"success case", nil, http.StatusNoContent, nil},
		{"file not found", http.ErrMissingFile, http.StatusNotFound, map[string]interface{}{
			"message": http.ErrMissingFile.Error()}},
		{"internal server error", http.ErrHandlerTimeout, http.StatusInternalServerError,
			map[string]interface{}{"message": http.ErrHandlerTimeout.Error()}},
	}

	for i, tc := range tests {
		statusCode, errObj := r.HTTPStatusFromError(tc.input)

		assert.Equal(t, tc.statusCode, statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.errObj, errObj, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
