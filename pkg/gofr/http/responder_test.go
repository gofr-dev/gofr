package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

func TestResponder(t *testing.T) {
	tests := []struct {
		desc         string
		data         any
		contentType  string
		expectedBody []byte
	}{
		{
			desc:         "raw response type",
			data:         resTypes.Raw{Data: []byte("raw data")},
			contentType:  "application/json",
			expectedBody: []byte(`"cmF3IGRhdGE="`),
		},
		{
			desc: "file response type",
			data: resTypes.File{
				ContentType: "image/png",
			},
			contentType:  "image/png",
			expectedBody: nil,
		},
		{
			desc:         "map response type",
			data:         map[string]string{"key": "value"},
			contentType:  "application/json",
			expectedBody: []byte(`{"data":{"key":"value"}}`),
		},
		{
			desc: "gofr response type with metadata",
			data: resTypes.Response{
				Data: "Hello World from new Server",
				Metadata: map[string]any{
					"environment": "stage",
				},
			},
			contentType:  "application/json",
			expectedBody: []byte(`{"metadata":{"environment":"stage"},"data":"Hello World from new Server"}`),
		},
		{
			desc: "gofr response type without metadata",
			data: resTypes.Response{
				Data: "Hello World from new Server",
			},
			contentType:  "application/json",
			expectedBody: []byte(`{"data":"Hello World from new Server"}`),
		},
	}

	for i, tc := range tests {
		recorder := httptest.NewRecorder()
		recorder.Body.Reset()
		r := NewResponder(recorder, http.MethodGet)

		r.Respond(tc.data, nil)

		contentType := recorder.Header().Get("Content-Type")
		assert.Equal(t, tc.contentType, contentType, "TEST[%d] Failed: %s", i, tc.desc)

		responseBody := recorder.Body.Bytes()

		expected := bytes.TrimSpace(tc.expectedBody)

		actual := bytes.TrimSpace(responseBody)

		assert.Equal(t, expected, actual, "TEST[%d] Failed: %s", i, tc.desc)
	}
}

func TestResponder_getStatusCode(t *testing.T) {
	tests := []struct {
		desc       string
		method     string
		data       any
		err        error
		statusCode int
		errObj     any
	}{
		{"success case", http.MethodGet, "success response", nil, http.StatusOK, nil},
		{"post with response body", http.MethodPost, "entity created", nil, http.StatusCreated, nil},
		{"post with nil response", http.MethodPost, nil, nil, http.StatusAccepted, nil},
		{"success delete", http.MethodDelete, nil, nil, http.StatusNoContent, nil},
		{"invalid route error", http.MethodGet, nil, ErrorInvalidRoute{}, http.StatusNotFound,
			map[string]any{"message": ErrorInvalidRoute{}.Error()}},
		{"internal server error", http.MethodGet, nil, http.ErrHandlerTimeout, http.StatusInternalServerError,
			map[string]any{"message": http.ErrHandlerTimeout.Error()}},
		{"partial content with error", http.MethodGet, "partial response", ErrorInvalidRoute{},
			http.StatusPartialContent, map[string]any{"message": ErrorInvalidRoute{}.Error()}},
	}

	for i, tc := range tests {
		statusCode, errObj := getStatusCode(tc.method, tc.data, tc.err)

		assert.Equal(t, tc.statusCode, statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.errObj, errObj, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

type temp struct {
	ID string `json:"id,omitempty"`
}

// newNilTemp returns a nil pointer of type *temp for testing purposes.
func newNilTemp() *temp {
	return nil
}

func TestRespondWithApplicationJSON(t *testing.T) {
	sampleData := map[string]string{"message": "Hello World"}
	sampleError := ErrorInvalidRoute{}

	tests := []struct {
		desc         string
		data         any
		err          error
		expectedCode int
		expectedBody string
	}{
		{"sample data response", sampleData, nil,
			http.StatusOK, `{"data":{"message":"Hello World"}}`},
		{"error response", nil, sampleError,
			http.StatusNotFound, `{"error":{"message":"route not registered"}}`},
		{"error response contains a nullable type with a nil value", newNilTemp(), sampleError,
			http.StatusNotFound, `{"error":{"message":"route not registered"}}`},
		{"error response with partial response", sampleData, sampleError,
			http.StatusPartialContent,
			`{"error":{"message":"route not registered"},"data":{"message":"Hello World"}}`},
	}

	for i, tc := range tests {
		recorder := httptest.NewRecorder()
		responder := Responder{w: recorder, method: http.MethodGet}

		responder.Respond(tc.data, tc.err)

		result := recorder.Result()

		assert.Equal(t, tc.expectedCode, result.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		body := new(bytes.Buffer)
		_, err := body.ReadFrom(result.Body)

		result.Body.Close()

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		// json Encoder by default terminate each value with a newline
		tc.expectedBody += "\n"

		assert.Equal(t, tc.expectedBody, body.String(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestIsNil(t *testing.T) {
	tests := []struct {
		desc     string
		value    any
		expected bool
	}{
		{"nil value", nil, true},
		{"nullable type with a nil value", newNilTemp(), true},
		{"not nil value", temp{ID: "test"}, false},
		{"chan type", make(chan int), false},
	}

	for i, tc := range tests {
		resp := isNil(tc.value)

		assert.Equal(t, tc.expected, resp, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
