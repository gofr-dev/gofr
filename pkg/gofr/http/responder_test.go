package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestResponder_getStatusCode(t *testing.T) {
	tests := []struct {
		desc       string
		method     string
		data       interface{}
		err        error
		statusCode int
		errObj     interface{}
	}{
		{"success case", http.MethodGet, "success response", nil, http.StatusOK, nil},
		{"post with response body", http.MethodPost, "entity created", nil, http.StatusCreated, nil},
		{"post with nil response", http.MethodPost, nil, nil, http.StatusAccepted, nil},
		{"success delete", http.MethodDelete, nil, nil, http.StatusNoContent, nil},
		{"invalid route error", http.MethodGet, nil, ErrorInvalidRoute{}, http.StatusNotFound,
			map[string]interface{}{"message": ErrorInvalidRoute{}.Error()}},
		{"internal server error", http.MethodGet, nil, http.ErrHandlerTimeout, http.StatusInternalServerError,
			map[string]interface{}{"message": http.ErrHandlerTimeout.Error()}},
	}

	for i, tc := range tests {
		statusCode, errObj := getStatusCode(tc.method, tc.data, tc.err)

		assert.Equal(t, tc.statusCode, statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.errObj, errObj, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
