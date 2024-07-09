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
	}{
		{"success case", http.MethodGet, "success response", nil, http.StatusOK},
		{"post with response body", http.MethodPost, "entity created", nil, http.StatusCreated},
		{"post with nil response", http.MethodPost, nil, nil, http.StatusAccepted},
		{"success delete", http.MethodDelete, nil, nil, http.StatusNoContent},
		{"invalid route error", http.MethodGet, nil, ErrorInvalidRoute{}, http.StatusNotFound},
		{"internal server error", http.MethodGet, nil, http.ErrHandlerTimeout, http.StatusInternalServerError},
	}

	for i, tc := range tests {
		statusCode := getStatusCode(tc.method, tc.data, tc.err)

		assert.Equal(t, tc.statusCode, statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestResponder_getErrResponse(t *testing.T) {
	tests := []struct {
		desc    string
		err     error
		reason  []string
		details interface{}
	}{
		{"success case", nil, nil, nil},
		{"invalid param error", ErrorInvalidParam{}, []string{ErrorInvalidParam{}.Error()}, nil},
		{"multiple errors", MultipleErrors{Errors: []error{ErrorMissingParam{}, CustomError{Reason: ErrorEntityAlreadyExist{}.Error()}}},
			[]string{ErrorMissingParam{}.Error(), CustomError{Reason: alreadyExistsMessage}.Error()}, nil},
		{"custom error", CustomError{Reason: ErrorEntityNotFound{}.Error()}, []string{ErrorEntityNotFound{}.Error()}, nil},
	}

	for i, tc := range tests {
		errObj := getErrResponse(tc.err)

		for j, err := range errObj {
			assert.Equal(t, tc.reason[j], err.Reason, "TEST[%d], Failed.\n%s", i, tc.desc)

			assert.Equal(t, tc.details, err.Details, "TEST[%d], Failed.\n%s", i, tc.desc)
		}
	}
}
