package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/errors"
	resTypes "gofr.dev/pkg/gofr/http/response"
)

func TestResponder_Respond(t *testing.T) {
	r := NewResponder(httptest.NewRecorder(), http.MethodGet)

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
	r := NewResponder(httptest.NewRecorder(), http.MethodGet)
	errInvalidParam := errors.InvalidParamError{Param: []string{"name"}}

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
		{"invalid parameters error", &errInvalidParam, http.StatusBadRequest,
			map[string]interface{}{"message": errInvalidParam.Error()}},
	}

	for i, tc := range tests {
		statusCode, errObj := r.HTTPStatusFromError(tc.input)

		assert.Equal(t, tc.statusCode, statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.errObj, errObj, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
