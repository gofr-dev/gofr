package http

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
		{"request timeout error", http.MethodGet, nil, ErrorRequestTimeout{},
			http.StatusRequestTimeout,
			map[string]any{"message": ErrorRequestTimeout{}.Error()}},
		{"client closed request error", http.MethodGet, nil, ErrorClientClosedRequest{}, 499,
			map[string]any{"message": ErrorClientClosedRequest{}.Error()}},
		{"server timeout error", http.MethodGet, nil, ErrorRequestTimeout{}, http.StatusRequestTimeout,
			map[string]any{"message": ErrorRequestTimeout{}.Error()}},
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
		{"client closed request - no response", nil, ErrorClientClosedRequest{},
			StatusClientClosedRequest, `{"error":{"message":"client closed request"}}`},
		{"server timeout error", nil, ErrorRequestTimeout{},
			http.StatusRequestTimeout, `{"error":{"message":"request timed out"}}`},
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

func TestResponder_TemplateResponse(t *testing.T) {
	templatePath := "./templates/example.html"
	templateContent := `<html><head><title>{{.Title}}</title></head><body>{{.Body}}</body></html>`

	createTemplateFile(t, templatePath, templateContent)
	defer removeTemplateDir(t)

	recorder := httptest.NewRecorder()
	r := NewResponder(recorder, http.MethodGet)

	templateData := map[string]string{"Title": "Test Title", "Body": "Test Body"}
	expectedBody := "<html><head><title>Test Title</title></head><body>Test Body</body></html>"

	r.Respond(resTypes.Template{Name: "example.html", Data: templateData}, nil)

	contentType := recorder.Header().Get("Content-Type")
	responseBody := recorder.Body.String()

	assert.Equal(t, "text/html", contentType)
	assert.Equal(t, expectedBody, responseBody)
}

func TestResponder_CustomErrorWithResponse(t *testing.T) {
	w := httptest.NewRecorder()
	responder := NewResponder(w, http.MethodGet)

	customErr := &CustomError{
		Code:    http.StatusNotFound,
		Message: "resource not found",
		Title:   "Custom Error",
	}

	responder.Respond(nil, customErr)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	expectedJSON := `{
		"error": {
			"code": 404,
			"title": "Custom Error",
			"message": "resource not found"
		}
	}`

	assert.JSONEq(t, expectedJSON, string(bodyBytes))
}

type CustomError struct {
	Code    int
	Message string
	Title   string
}

func (e *CustomError) Error() string   { return e.Message }
func (e *CustomError) StatusCode() int { return e.Code }
func (e *CustomError) Response() map[string]any {
	return map[string]any{"title": e.Title, "code": e.Code}
}

func TestResponder_ReservedMessageField(t *testing.T) {
	w := httptest.NewRecorder()
	responder := NewResponder(w, http.MethodGet)

	msgErr := &MessageOverrideError{
		Msg: "original message",
	}

	responder.Respond(nil, msgErr)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	expectedJSON := `{
		"error": {
			"message": "original message",
			"info": "additional info"
		}
	}`

	assert.JSONEq(t, expectedJSON, string(bodyBytes))
}

// EmptyError represents an error as an empty struct.
// It implements the error interface.
type emptyError struct{}

// Error implements the error interface.
func (emptyError) Error() string {
	return "error occurred"
}

func TestResponder_EmptyErrorStruct(t *testing.T) {
	recorder := httptest.NewRecorder()
	responder := Responder{w: recorder, method: http.MethodGet}

	statusCode, errObj := responder.determineResponse(nil, emptyError{})

	assert.Equal(t, http.StatusInternalServerError, statusCode)
	assert.Equal(t, map[string]any{"message": "error occurred"}, errObj)
}

func TestIsEmptyStruct(t *testing.T) {
	tests := []struct {
		desc     string
		data     any
		expected bool
	}{
		{"nil value", nil, false},
		{"empty struct", struct{}{}, true},
		{"non-empty struct", struct{ ID int }{ID: 1}, false},
		{"nil pointer to struct", (*struct{})(nil), false},
		{"pointer to non-empty struct", &struct{ ID int }{ID: 1}, false},
		{"non-struct type", 42, false},
	}

	for i, tc := range tests {
		result := isEmptyStruct(tc.data)

		assert.Equal(t, tc.expected, result, "TEST[%d] Failed: %s", i, tc.desc)
	}
}

type MessageOverrideError struct {
	Msg string
}

func (e *MessageOverrideError) Error() string { return e.Msg }
func (*MessageOverrideError) Response() map[string]any {
	return map[string]any{
		"message": "trying to override",
		"info":    "additional info",
	}
}

func createTemplateFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.MkdirAll("./templates", os.ModePerm)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)
}

func removeTemplateDir(t *testing.T) {
	t.Helper()

	err := os.RemoveAll("./templates")

	require.NoError(t, err)
}

func TestResponder_RedirectResponse_Post(t *testing.T) {
	recorder := httptest.NewRecorder()
	r := NewResponder(recorder, http.MethodPost)

	// Set up redirect with specific URL and status code
	redirectURL := "/new-location?from=start"
	statusCode := http.StatusSeeOther // 303

	redirect := resTypes.Redirect{URL: redirectURL}

	r.Respond(redirect, nil)

	assert.Equal(t, statusCode, recorder.Code, "Redirect should set the correct status code")
	assert.Equal(t, redirectURL, recorder.Header().Get("Location"),
		"Redirect should set the Location header")
	assert.Empty(t, recorder.Body.String(), "Redirect response should not have a body")
}

func TestResponder_RedirectResponse_Head(t *testing.T) {
	recorder := httptest.NewRecorder()
	r := NewResponder(recorder, http.MethodHead)

	// Set up redirect with specific URL and status code
	redirectURL := "/new-location?from=start"
	statusCode := http.StatusFound // 302

	redirect := resTypes.Redirect{URL: redirectURL}

	r.Respond(redirect, nil)

	assert.Equal(t, statusCode, recorder.Code, "Redirect should set the correct status code")
	assert.Equal(t, redirectURL, recorder.Header().Get("Location"),
		"Redirect should set the Location header")
	assert.Empty(t, recorder.Body.String(), "Redirect response should not have a body")
}

func TestResponder_ClientClosedRequestHandling(t *testing.T) {
	recorder := httptest.NewRecorder()
	responder := NewResponder(recorder, http.MethodGet)

	// ErrorClientClosedRequest should not send any response
	responder.Respond(nil, ErrorClientClosedRequest{})

	assert.Equal(t, 499, recorder.Code)
	assert.JSONEq(t, `{"error":{"message":"client closed request"}}`, recorder.Body.String())
}

func TestResponder_ContentTypePreservation(t *testing.T) {
	tests := []struct {
		desc              string
		presetContentType string
		expectedType      string
	}{
		{
			desc:              "preset content type should be preserved",
			presetContentType: "text/event-stream",
			expectedType:      "text/event-stream",
		},
		{
			desc:              "no preset content type - defaults to application/json",
			presetContentType: "",
			expectedType:      "application/json",
		},
	}

	for i, tc := range tests {
		recorder := httptest.NewRecorder()

		// Simulate SetCustomHeaders by manually setting Content-Type header before calling Respond
		if tc.presetContentType != "" {
			recorder.Header().Set("Content-Type", tc.presetContentType)
		}

		responder := NewResponder(recorder, http.MethodGet)
		responder.Respond("Test data", nil)

		contentType := recorder.Header().Get("Content-Type")

		assert.Equal(t, tc.expectedType, contentType, "TEST[%d] Failed: %s", i, tc.desc)
	}
}
