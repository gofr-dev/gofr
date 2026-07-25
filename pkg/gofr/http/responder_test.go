package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

var (
	errTest        = errors.New("internal server error")
	errBoom        = errors.New("boom")
	errPartialFail = errors.New("partial fail")
)

func TestResponder(t *testing.T) {
	tests := []struct {
		desc         string
		data         any
		contentType  string
		expectedBody []byte
	}{
		{
			desc: "xml response type default content type",
			data: resTypes.XML{
				Content: []byte(`<Response status="ok"><Message>Hello</Message></Response>`),
			},
			contentType:  "application/xml",
			expectedBody: []byte(`<Response status="ok"><Message>Hello</Message></Response>`),
		},
		{
			desc: "xml response type custom content type",
			data: resTypes.XML{
				Content:     []byte(`<soapenv:Envelope></soapenv:Envelope>`),
				ContentType: "application/soap+xml",
			},
			contentType:  "application/soap+xml",
			expectedBody: []byte(`<soapenv:Envelope></soapenv:Envelope>`),
		},
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

// TestResponder_XMLFileTemplate_ErrorStatusCodes verifies that XML, File, and Template responses
// return appropriate error status codes when errors occur, not always 200 OK.
func TestResponder_XMLFileTemplate_ErrorStatusCodes(t *testing.T) {
	tests := []struct {
		desc         string
		data         any
		err          error
		expectedCode int
	}{
		{
			desc: "XML response with 404 error should return 404",
			data: resTypes.XML{
				Content: []byte(`<Response><Error>Not Found</Error></Response>`),
			},
			err:          ErrorEntityNotFound{Name: "id", Value: "123"},
			expectedCode: http.StatusNotFound,
		},
		{
			desc: "XML response with 500 error should return 500",
			data: resTypes.XML{
				Content: []byte(`<Response><Error>Internal Error</Error></Response>`),
			},
			err:          errTest,
			expectedCode: http.StatusInternalServerError,
		},
		{
			desc: "File response with 404 error should return 404",
			data: resTypes.File{
				ContentType: "image/png",
				Content:     []byte("fake image data"),
			},
			err:          ErrorEntityNotFound{Name: "file", Value: "test.png"},
			expectedCode: http.StatusNotFound,
		},
		{
			desc: "File response with 500 error should return 500",
			data: resTypes.File{
				ContentType: "application/pdf",
				Content:     []byte("fake pdf data"),
			},
			err:          errTest,
			expectedCode: http.StatusInternalServerError,
		},
		{
			desc: "XML response with no error should return 200",
			data: resTypes.XML{
				Content: []byte(`<Response><Status>OK</Status></Response>`),
			},
			err:          nil,
			expectedCode: http.StatusOK,
		},
		{
			desc: "File response with no error should return 200",
			data: resTypes.File{
				ContentType: "text/plain",
				Content:     []byte("file content"),
			},
			err:          nil,
			expectedCode: http.StatusOK,
		},
	}

	for i, tc := range tests {
		recorder := httptest.NewRecorder()
		r := NewResponder(recorder, http.MethodGet)

		r.Respond(tc.data, tc.err)

		assert.Equal(t, tc.expectedCode, recorder.Code, "TEST[%d] Failed: %s", i, tc.desc)
	}
}

func TestResponder_JSONEncodingFailure(t *testing.T) {
	tests := []struct {
		desc string
		data any
	}{
		{"NaN value", math.NaN()},
		{"positive infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
		{"channel type", make(chan int)},
		{"function type", func() {}},
	}

	for i, tc := range tests {
		recorder := httptest.NewRecorder()
		responder := NewResponder(recorder, http.MethodGet)

		responder.Respond(tc.data, nil)

		result := recorder.Result()

		assert.Equal(t, http.StatusInternalServerError, result.StatusCode, "TEST[%d] Failed: %s", i, tc.desc)
		assert.Equal(t, "application/json", result.Header.Get("Content-Type"), "TEST[%d] Failed: %s", i, tc.desc)

		body := new(bytes.Buffer)
		_, err := body.ReadFrom(result.Body)

		require.NoError(t, err, "TEST[%d] Failed: %s", i, tc.desc)

		expectedBody := `{"error":{"message": "failed to encode response as JSON"}}` + "\n"
		assert.Equal(t, expectedBody, body.String(), "TEST[%d] Failed: %s", i, tc.desc)

		require.NoError(t, result.Body.Close())
	}
}

func TestResponder_ValidEncodableData(t *testing.T) {
	tests := []struct {
		desc         string
		data         any
		expectedCode int
	}{
		{"normal float", 42.5, http.StatusOK},
		{"zero float", 0.0, http.StatusOK},
		{"struct with floats", struct{ Temp float64 }{Temp: 98.6}, http.StatusOK},
		{"map with numbers", map[string]float64{"value": 123.45}, http.StatusOK},
	}

	for i, tc := range tests {
		recorder := httptest.NewRecorder()
		responder := NewResponder(recorder, http.MethodGet)

		responder.Respond(tc.data, nil)

		result := recorder.Result()

		t.Cleanup(func() {
			require.NoError(t, result.Body.Close())
		})

		assert.Equal(t, tc.expectedCode, result.StatusCode, "TEST[%d] Failed: %s", i, tc.desc)

		body := new(bytes.Buffer)
		_, err := body.ReadFrom(result.Body)

		require.NoError(t, err, "TEST[%d] Failed: %s", i, tc.desc)

		assert.NotEmpty(t, body.String(), "TEST[%d] Failed: %s", i, tc.desc)
	}
}

// TestResponseEnvelopeSnapshot pins the on-the-wire byte shape of the
// response envelope for the most common handler return types. Any
// later refactor that changes the bytes a client sees will fail this
// test — including ostensibly cosmetic things like field ordering,
// the trailing newline, the Content-Type header, or the status code.
//
// If a change here is intended, update the expectations in this test
// AND mention it in the PR body so the impact on clients is reviewed.
func TestResponseEnvelopeSnapshot(t *testing.T) {
	type sampleStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	cases := []struct {
		name       string
		method     string
		data       any
		err        error
		wantStatus int
		wantCType  string
		wantBody   string
	}{
		{
			name:       "string-get",
			method:     http.MethodGet,
			data:       "hello",
			wantStatus: http.StatusOK,
			wantCType:  "application/json",
			wantBody:   `{"data":"hello"}` + "\n",
		},
		{
			name:       "map-get",
			method:     http.MethodGet,
			data:       map[string]string{"message": "hello"},
			wantStatus: http.StatusOK,
			wantCType:  "application/json",
			wantBody:   `{"data":{"message":"hello"}}` + "\n",
		},
		{
			name:       "struct-get",
			method:     http.MethodGet,
			data:       sampleStruct{ID: 42, Name: "alice"},
			wantStatus: http.StatusOK,
			wantCType:  "application/json",
			wantBody:   `{"data":{"id":42,"name":"alice"}}` + "\n",
		},
		{
			name:       "struct-post-201",
			method:     http.MethodPost,
			data:       sampleStruct{ID: 7, Name: "bob"},
			wantStatus: http.StatusCreated,
			wantCType:  "application/json",
			wantBody:   `{"data":{"id":7,"name":"bob"}}` + "\n",
		},
		{
			name:       "nil-post-202",
			method:     http.MethodPost,
			data:       nil,
			wantStatus: http.StatusAccepted,
			wantCType:  "application/json",
			wantBody:   "{}\n",
		},
		{
			name:       "nil-delete-204",
			method:     http.MethodDelete,
			data:       nil,
			wantStatus: http.StatusNoContent,
			wantCType:  "application/json",
			wantBody:   "{}\n",
		},
		{
			name:       "error-only-500",
			method:     http.MethodGet,
			data:       nil,
			err:        errBoom,
			wantStatus: http.StatusInternalServerError,
			wantCType:  "application/json",
			wantBody:   `{"error":{"message":"boom"}}` + "\n",
		},
		{
			name:       "data-and-error-206",
			method:     http.MethodGet,
			data:       map[string]string{"partial": "ok"},
			err:        errPartialFail,
			wantStatus: http.StatusPartialContent,
			wantCType:  "application/json",
			wantBody:   `{"error":{"message":"partial fail"},"data":{"partial":"ok"}}` + "\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := NewResponder(w, tc.method)

			r.Respond(tc.data, tc.err)

			assert.Equal(t, tc.wantStatus, w.Code, "status code")
			assert.Equal(t, tc.wantCType, w.Header().Get("Content-Type"), "Content-Type")
			assert.Equal(t, tc.wantBody, w.Body.String(), "body bytes")
		})
	}
}

// BenchmarkRespond_String measures the cost of responding with a string
// value. Captures: envelope wrap into `{"data":"..."}`, json.Marshal,
// three separate Write calls (responder.go:65-67).
//
// PR-3 target: a single Write with Content-Length set. Expect lower
// ns/op and B/op after fix.
func BenchmarkRespond_String(b *testing.B) {
	r := NewResponder(&discardingResponseWriter{}, http.MethodGet)
	data := "hello"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Respond(data, nil)
	}
}

// BenchmarkRespond_Map measures a typical JSON response: a small map.
// Closest match to GoFr's /json bench endpoint.
func BenchmarkRespond_Map(b *testing.B) {
	r := NewResponder(&discardingResponseWriter{}, http.MethodGet)
	data := map[string]string{"message": "hello"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Respond(data, nil)
	}
}

// BenchmarkRespond_Struct measures the typed-struct response path
// (uses reflection in json.Marshal). Representative of real API
// handlers that return structs.
func BenchmarkRespond_Struct(b *testing.B) {
	type payload struct {
		Message string `json:"message"`
		ID      int    `json:"id"`
	}

	r := NewResponder(&discardingResponseWriter{}, http.MethodGet)
	data := payload{Message: "hello", ID: 42}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Respond(data, nil)
	}
}

// TestResponder_ByteIdenticalToMarshal proves the pooled-buffer encode produces
// exactly the same bytes as the previous json.Marshal(resp) + "\n" path for a
// variety of payload shapes, including the {data:...} and {error:...} envelopes.
func TestResponder_ByteIdenticalToMarshal(t *testing.T) {
	cases := []struct {
		name string
		data any
	}{
		{"nil", nil},
		{"string", "hello"},
		{"map", map[string]any{"a": 1, "b": "two"}},
		{"slice", []int{1, 2, 3}},
		{"struct", struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{"gofr", 3}},
		{"html-escaped chars", map[string]string{"html": "<b>&</b>"}},
		{"raw", resTypes.Raw{Data: map[string]any{"x": true}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewResponder(rec, http.MethodGet).Respond(tc.data, nil)

			// Reconstruct the exact envelope the responder builds, then encode
			// it the old way (Marshal + newline) as the oracle.
			var resp any

			switch v := tc.data.(type) {
			case resTypes.Raw:
				resp = v.Data
			default:
				resp = response{Data: tc.data}
			}

			want, err := json.Marshal(resp)
			require.NoError(t, err)

			assert.Equal(t, string(want)+"\n", rec.Body.String(),
				"response bytes must match the pre-pooling json.Marshal output exactly")
		})
	}
}

// TestResponder_ConcurrentPoolSafety runs many responders concurrently, each
// with its own recorder and distinct payload, and asserts every response body
// is correct and independent — proof the sync.Pool never leaks one request's
// buffer contents into another.
func TestResponder_ConcurrentPoolSafety(t *testing.T) {
	const n = 500

	var wg sync.WaitGroup

	wg.Add(n)

	errs := make([]error, n)
	bodies := make([]string, n)

	for i := 0; i < n; i++ {
		go func(id int) {
			defer wg.Done()

			rec := httptest.NewRecorder()
			payload := map[string]any{"id": id, "pad": "some padding value to vary length"}
			NewResponder(rec, http.MethodGet).Respond(payload, nil)
			bodies[id] = rec.Body.String()
		}(i)
	}

	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])

		var env struct {
			Data struct {
				ID int `json:"id"`
			} `json:"data"`
		}

		require.NoErrorf(t, json.Unmarshal([]byte(bodies[i]), &env), "body %d not valid JSON: %q", i, bodies[i])
		assert.Equalf(t, i, env.Data.ID, "response %d carried another request's data", i)
	}
}

// discardWriter is a no-op http.ResponseWriter so the benchmark measures only
// the encode/pool/write cost inside Respond, not socket or recorder overhead.
type discardWriter struct{ h http.Header }

func (d *discardWriter) Header() http.Header {
	if d.h == nil {
		d.h = make(http.Header)
	}

	return d.h
}

func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }
func (*discardWriter) WriteHeader(int)             {}

// benchPayload is a representative JSON response body (a small object slice),
// matching a typical list endpoint.
type benchPayload struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// BenchmarkResponderRespond measures the JSON response hot path, which the
// pooled-buffer change targets (it avoids the fresh []byte json.Marshal returns
// and collapses body + newline into one Write).
func BenchmarkResponderRespond(b *testing.B) {
	data := []benchPayload{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
		{ID: 3, Name: "Carol", Email: "carol@example.com"},
	}

	r := NewResponder(&discardWriter{}, http.MethodGet)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Respond(data, nil)
	}
}

// ---------------------------------------------------------------------------
// Characterization suite.
//
// Everything below pins the CURRENT observable wire contract of Responder:
// exact status code, exact Content-Type, exact body bytes. It is deliberately
// exhaustive and deliberately literal — no Contains, no "not empty". A refactor
// that changes any byte a client sees must fail here.
// ---------------------------------------------------------------------------

var (
	errCharPlain = errors.New("plain failure")
)

// charError is an error carrying an arbitrary status code, used to pin the
// StatusCodeResponder branch without depending on a concrete GoFr error type.
type charError struct {
	msg  string
	code int
}

func (e charError) Error() string   { return e.msg }
func (e charError) StatusCode() int { return e.code }

// charMarshallerError implements both StatusCodeResponder and ResponseMarshaller
// so the merge behavior of createErrorResponse is pinned by value type (the
// existing CustomError is a pointer type).
type charMarshallerError struct{}

func (charMarshallerError) Error() string   { return "validation failed" }
func (charMarshallerError) StatusCode() int { return http.StatusBadRequest }
func (charMarshallerError) Response() map[string]any {
	return map[string]any{"field": "email", "reason": "bad format"}
}

type charStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type charOmit struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// respondCase is one exact-bytes expectation for Respond.
type respondCase struct {
	name       string
	method     string
	data       any
	err        error
	wantStatus int
	wantCType  string
	wantBody   string
}

func runRespondCases(t *testing.T, cases []respondCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			NewResponder(w, tc.method).Respond(tc.data, tc.err)

			assert.Equal(t, tc.wantStatus, w.Code, "status code")
			assert.Equal(t, tc.wantCType, w.Header().Get("Content-Type"), "Content-Type")
			assert.Equal(t, tc.wantBody, w.Body.String(), "body bytes")
		})
	}
}

// TestResponder_Char_SuccessEnvelope pins the success-path envelope for every
// shape of data GoFr allows a handler to return, across every HTTP method whose
// status mapping differs.
func TestResponder_Char_SuccessEnvelope(t *testing.T) {
	runRespondCases(t, []respondCase{
		// --- nil data, per method -------------------------------------------
		{"nil-get", http.MethodGet, nil, nil, http.StatusOK, "application/json", "{}\n"},
		{"nil-post", http.MethodPost, nil, nil, http.StatusAccepted, "application/json", "{}\n"},
		{"nil-put", http.MethodPut, nil, nil, http.StatusOK, "application/json", "{}\n"},
		{"nil-patch", http.MethodPatch, nil, nil, http.StatusOK, "application/json", "{}\n"},
		{"nil-delete", http.MethodDelete, nil, nil, http.StatusNoContent, "application/json", "{}\n"},
		{"nil-head", http.MethodHead, nil, nil, http.StatusOK, "application/json", "{}\n"},
		{"nil-options", http.MethodOptions, nil, nil, http.StatusOK, "application/json", "{}\n"},
		{"nil-empty-method", "", nil, nil, http.StatusOK, "application/json", "{}\n"},

		// --- primitives ------------------------------------------------------
		{"string", http.MethodGet, "hello", nil, http.StatusOK, "application/json", "{\"data\":\"hello\"}\n"},
		// NOTE: `data` carries `omitempty`, but its Go type is `any`, so only a
		// nil INTERFACE is omitted. A zero-valued primitive is still emitted.
		{"empty-string", http.MethodGet, "", nil, http.StatusOK, "application/json", "{\"data\":\"\"}\n"},
		{"int", http.MethodGet, 42, nil, http.StatusOK, "application/json", "{\"data\":42}\n"},
		{"zero-int", http.MethodGet, 0, nil, http.StatusOK, "application/json", "{\"data\":0}\n"},
		{"negative-int", http.MethodGet, -7, nil, http.StatusOK, "application/json", "{\"data\":-7}\n"},
		{"float", http.MethodGet, 3.5, nil, http.StatusOK, "application/json", "{\"data\":3.5}\n"},
		{"bool-true", http.MethodGet, true, nil, http.StatusOK, "application/json", "{\"data\":true}\n"},
		{"bool-false", http.MethodGet, false, nil, http.StatusOK, "application/json", "{\"data\":false}\n"},

		// --- structs ----------------------------------------------------------
		{
			"struct", http.MethodGet, charStruct{ID: 1, Name: "a"}, nil,
			http.StatusOK, "application/json", "{\"data\":{\"id\":1,\"name\":\"a\"}}\n",
		},
		{
			"zero-struct-no-omitempty", http.MethodGet, charStruct{}, nil,
			http.StatusOK, "application/json", "{\"data\":{\"id\":0,\"name\":\"\"}}\n",
		},
		{
			"zero-struct-all-omitempty", http.MethodGet, charOmit{}, nil,
			http.StatusOK, "application/json", "{\"data\":{}}\n",
		},
		{
			"pointer-to-struct", http.MethodGet, &charStruct{ID: 2, Name: "b"}, nil,
			http.StatusOK, "application/json", "{\"data\":{\"id\":2,\"name\":\"b\"}}\n",
		},
		// A typed nil pointer inside the interface: isNil() collapses it to nil
		// for the BODY, but handleSuccess sees a non-nil interface, so a POST
		// still reports 201 Created rather than 202 Accepted.
		{"typed-nil-ptr-get", http.MethodGet, newNilTemp(), nil, http.StatusOK, "application/json", "{}\n"},
		{"typed-nil-ptr-post", http.MethodPost, newNilTemp(), nil, http.StatusCreated, "application/json", "{}\n"},

		// --- slices / arrays / maps -------------------------------------------
		{"slice-of-int", http.MethodGet, []int{1, 2, 3}, nil, http.StatusOK, "application/json", "{\"data\":[1,2,3]}\n"},
		{"empty-slice", http.MethodGet, []int{}, nil, http.StatusOK, "application/json", "{\"data\":[]}\n"},
		// A nil SLICE is a non-nil interface, so it survives omitempty and the
		// client sees an explicit `null` — unlike a nil POINTER, which isNil()
		// collapses to an absent `data` key. Two ways to say "nothing".
		{"nil-slice", http.MethodGet, []int(nil), nil, http.StatusOK, "application/json", "{\"data\":null}\n"},
		{
			"slice-of-struct", http.MethodGet, []charStruct{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}, nil,
			http.StatusOK, "application/json", "{\"data\":[{\"id\":1,\"name\":\"a\"},{\"id\":2,\"name\":\"b\"}]}\n",
		},
		{"array", http.MethodGet, [2]int{9, 8}, nil, http.StatusOK, "application/json", "{\"data\":[9,8]}\n"},
		{
			"map-string-string", http.MethodGet, map[string]string{"k": "v"}, nil,
			http.StatusOK, "application/json", "{\"data\":{\"k\":\"v\"}}\n",
		},
		{"empty-map", http.MethodGet, map[string]string{}, nil, http.StatusOK, "application/json", "{\"data\":{}}\n"},
		{"nil-map", http.MethodGet, map[string]string(nil), nil, http.StatusOK, "application/json", "{\"data\":null}\n"},
		// Map keys are emitted in sorted order by encoding/json, so this is
		// deterministic despite Go's randomized map iteration.
		{
			"map-key-ordering", http.MethodGet, map[string]int{"z": 1, "a": 2, "m": 3}, nil,
			http.StatusOK, "application/json", "{\"data\":{\"a\":2,\"m\":3,\"z\":1}}\n",
		},
	})
}

// TestResponder_Char_RawEnvelope pins resTypes.Raw: the envelope is bypassed
// entirely and Data is written as the whole body.
func TestResponder_Char_RawEnvelope(t *testing.T) {
	runRespondCases(t, []respondCase{
		{
			"raw-map", http.MethodGet, resTypes.Raw{Data: map[string]string{"k": "v"}}, nil,
			http.StatusOK, "application/json", "{\"k\":\"v\"}\n",
		},
		{"raw-string", http.MethodGet, resTypes.Raw{Data: "plain"}, nil, http.StatusOK, "application/json", "\"plain\"\n"},
		{
			"raw-slice", http.MethodGet, resTypes.Raw{Data: []int{1, 2}}, nil,
			http.StatusOK, "application/json", "[1,2]\n",
		},
		// Raw{} is a zero struct, so a POST sees non-nil data and reports 201.
		{"raw-nil-data-get", http.MethodGet, resTypes.Raw{}, nil, http.StatusOK, "application/json", "null\n"},
		{"raw-nil-data-post", http.MethodPost, resTypes.Raw{Data: "x"}, nil, http.StatusCreated, "application/json", "\"x\"\n"},

		// LATENT BUG (pinned as-is): when a handler returns a Raw alongside an
		// error, the status becomes 206 Partial Content but the error object is
		// silently DROPPED from the body — the client gets only Raw.Data and no
		// indication of what failed.
		{
			"raw-with-error-drops-error", http.MethodGet, resTypes.Raw{Data: "partial"}, errCharPlain,
			http.StatusPartialContent, "application/json", "\"partial\"\n",
		},
		// LATENT BUG (pinned as-is): Raw{} is an empty struct, so isEmptyStruct
		// fires and the status becomes 500 with errEmptyResponse — but the body
		// is still the raw `null`, not the error envelope.
		{
			"raw-empty-with-error", http.MethodGet, resTypes.Raw{}, errCharPlain,
			http.StatusInternalServerError, "application/json", "null\n",
		},
	})
}

// TestResponder_Char_ResponseEnvelope pins resTypes.Response, including the
// Metadata field and the fixed JSON field ordering (error, metadata, data).
func TestResponder_Char_ResponseEnvelope(t *testing.T) {
	runRespondCases(t, []respondCase{
		{
			"response-data-only", http.MethodGet,
			resTypes.Response{Data: map[string]string{"k": "v"}}, nil,
			http.StatusOK, "application/json", "{\"data\":{\"k\":\"v\"}}\n",
		},
		{
			"response-with-metadata", http.MethodGet,
			resTypes.Response{Data: "d", Metadata: map[string]any{"page": 1}}, nil,
			http.StatusOK, "application/json", "{\"metadata\":{\"page\":1},\"data\":\"d\"}\n",
		},
		// Field order in the envelope is error, then metadata, then data.
		{
			"response-metadata-and-error-ordering", http.MethodGet,
			resTypes.Response{Data: "d", Metadata: map[string]any{"page": 1}}, errCharPlain,
			http.StatusPartialContent, "application/json",
			"{\"error\":{\"message\":\"plain failure\"},\"metadata\":{\"page\":1},\"data\":\"d\"}\n",
		},
		{
			"response-empty-metadata-omitted", http.MethodGet,
			resTypes.Response{Data: "d", Metadata: map[string]any{}}, nil,
			http.StatusOK, "application/json", "{\"data\":\"d\"}\n",
		},
		// Headers are declared `json:"-"` and are applied by the caller
		// (handler.ServeHTTP), never by Respond.
		{
			"response-headers-not-serialized", http.MethodGet,
			resTypes.Response{Data: "d", Headers: map[string]string{"X-A": "b"}}, nil,
			http.StatusOK, "application/json", "{\"data\":\"d\"}\n",
		},
		// LATENT QUIRK (pinned as-is): resTypes.Response{} is a zero struct, so
		// isEmptyStruct fires: the status becomes 500 AND the caller's real
		// error is replaced by the generic "internal server error".
		{
			"response-empty-with-error", http.MethodGet,
			resTypes.Response{}, errCharPlain,
			http.StatusInternalServerError, "application/json", "{\"error\":{\"message\":\"internal server error\"}}\n",
		},
	})
}

// TestResponder_Char_ResponseHeadersNotApplied pins that Respond does NOT apply
// resTypes.Response.Headers to the writer — that is handler.ServeHTTP's job.
func TestResponder_Char_ResponseHeadersNotApplied(t *testing.T) {
	w := httptest.NewRecorder()

	NewResponder(w, http.MethodGet).Respond(resTypes.Response{
		Data:    "d",
		Headers: map[string]string{"X-Custom": "v"},
	}, nil)

	assert.Empty(t, w.Header().Get("X-Custom"), "Respond must not set Response.Headers")
}

// TestResponder_Char_ErrorEnvelope pins the exact `{"error":{...}}` envelope and
// status code for every error type GoFr maps, plus the generic branches.
func TestResponder_Char_ErrorEnvelope(t *testing.T) {
	runRespondCases(t, []respondCase{
		// --- plain error (no StatusCodeResponder) -> 500 ----------------------
		{
			"plain-error", http.MethodGet, nil, errCharPlain,
			http.StatusInternalServerError, "application/json", "{\"error\":{\"message\":\"plain failure\"}}\n",
		},
		// The method-based success mapping is NOT consulted on the error path:
		// a failed DELETE is 500, not 204.
		{
			"plain-error-delete", http.MethodDelete, nil, errCharPlain,
			http.StatusInternalServerError, "application/json", "{\"error\":{\"message\":\"plain failure\"}}\n",
		},
		{
			"plain-error-post", http.MethodPost, nil, errCharPlain,
			http.StatusInternalServerError, "application/json", "{\"error\":{\"message\":\"plain failure\"}}\n",
		},

		// --- GoFr status-coded errors ----------------------------------------
		{
			"entity-not-found", http.MethodGet, nil, ErrorEntityNotFound{Name: "id", Value: "2"},
			http.StatusNotFound, "application/json", "{\"error\":{\"message\":\"No entity found with id: 2\"}}\n",
		},
		{
			"entity-not-found-zero", http.MethodGet, nil, ErrorEntityNotFound{},
			http.StatusNotFound, "application/json", "{\"error\":{\"message\":\"No entity found with : \"}}\n",
		},
		{
			"entity-already-exists", http.MethodGet, nil, ErrorEntityAlreadyExist{},
			http.StatusConflict, "application/json", "{\"error\":{\"message\":\"entity already exists\"}}\n",
		},
		// NOTE: ErrorInvalidParam has a `json:"param,omitempty"` tag, but it does
		// NOT implement ResponseMarshaller, so the param list never reaches the
		// wire — the client only ever sees the rendered message string.
		{
			"invalid-param", http.MethodGet, nil, ErrorInvalidParam{Params: []string{"a", "b"}},
			http.StatusBadRequest, "application/json", "{\"error\":{\"message\":\"'2' invalid parameter(s): a, b\"}}\n",
		},
		{
			"invalid-param-empty", http.MethodGet, nil, ErrorInvalidParam{},
			http.StatusBadRequest, "application/json", "{\"error\":{\"message\":\"'0' invalid parameter(s): \"}}\n",
		},
		{
			"missing-param", http.MethodGet, nil, ErrorMissingParam{Params: []string{"id"}},
			http.StatusBadRequest, "application/json", "{\"error\":{\"message\":\"'1' missing parameter(s): id\"}}\n",
		},
		{
			"invalid-route", http.MethodGet, nil, ErrorInvalidRoute{},
			http.StatusNotFound, "application/json", "{\"error\":{\"message\":\"route not registered\"}}\n",
		},
		{
			"request-timeout", http.MethodGet, nil, ErrorRequestTimeout{},
			http.StatusRequestTimeout, "application/json", "{\"error\":{\"message\":\"request timed out\"}}\n",
		},
		{
			"client-closed-request", http.MethodGet, nil, ErrorClientClosedRequest{},
			StatusClientClosedRequest, "application/json", "{\"error\":{\"message\":\"client closed request\"}}\n",
		},
		{
			"panic-recovery", http.MethodGet, nil, ErrorPanicRecovery{},
			http.StatusInternalServerError, "application/json", "{\"error\":{\"message\":\"Internal Server Error\"}}\n",
		},
		{
			"too-many-requests", http.MethodGet, nil, ErrorTooManyRequests{},
			http.StatusTooManyRequests, "application/json", "{\"error\":{\"message\":\"rate limit exceeded\"}}\n",
		},
		{
			"service-unavailable-bare", http.MethodGet, nil, ErrorServiceUnavailable{},
			http.StatusServiceUnavailable, "application/json", "{\"error\":{\"message\":\"Service Unavailable\"}}\n",
		},
		{
			"service-unavailable-detailed", http.MethodGet, nil,
			ErrorServiceUnavailable{Dependency: "redis", ErrorMessage: "dial fail"},
			http.StatusServiceUnavailable, "application/json",
			"{\"error\":{\"message\":\"Service unavailable due to error: dial fail from dependency redis\"}}\n",
		},
	})
}

// TestResponder_Char_ErrorEnvelopeEdges pins the remaining error-path branches:
// arbitrary status codes, the ResponseMarshaller merge, the 206 partial-content
// rule and the empty-struct short circuit.
func TestResponder_Char_ErrorEnvelopeEdges(t *testing.T) {
	runRespondCases(t, []respondCase{
		// --- arbitrary status-coded error ------------------------------------
		{
			"custom-status-code", http.MethodGet, nil, charError{msg: "teapot", code: http.StatusTeapot},
			http.StatusTeapot, "application/json", "{\"error\":{\"message\":\"teapot\"}}\n",
		},
		// A StatusCodeResponder reporting 0 is normalized to 500 by
		// determineResponse.
		{
			"zero-status-code-normalized-to-500", http.MethodGet, nil, charError{msg: "zero", code: 0},
			http.StatusInternalServerError, "application/json", "{\"error\":{\"message\":\"zero\"}}\n",
		},

		// --- ResponseMarshaller merge ----------------------------------------
		// Extra fields are merged in alongside "message"; keys are emitted in
		// sorted order because the error object is a map.
		{
			"response-marshaller-merge", http.MethodGet, nil, charMarshallerError{},
			http.StatusBadRequest, "application/json",
			"{\"error\":{\"field\":\"email\",\"message\":\"validation failed\",\"reason\":\"bad format\"}}\n",
		},

		// --- data + error -> 206 Partial Content ------------------------------
		// NOTE: the error's own StatusCode() is ignored entirely when data is
		// non-nil; a 404 alongside partial data is reported as 206.
		{
			"data-plus-statuscoded-error-is-206", http.MethodGet, map[string]string{"k": "v"},
			ErrorEntityNotFound{Name: "id", Value: "9"},
			http.StatusPartialContent, "application/json",
			"{\"error\":{\"message\":\"No entity found with id: 9\"},\"data\":{\"k\":\"v\"}}\n",
		},
		// A typed nil pointer counts as nil here, so the error's status wins.
		{
			"typed-nil-data-plus-error-uses-error-status", http.MethodGet, newNilTemp(), ErrorEntityNotFound{},
			http.StatusNotFound, "application/json", "{\"error\":{\"message\":\"No entity found with : \"}}\n",
		},

		// --- empty-struct short circuit ---------------------------------------
		// LATENT QUIRK (pinned as-is): when data is a zero-valued struct AND an
		// error is present, determineResponse replaces the status with 500 and
		// the error object with the generic "internal server error" — the real
		// error is lost. The zero struct is still serialized into `data`.
		{
			"empty-struct-plus-error", http.MethodGet, charStruct{}, ErrorEntityNotFound{Name: "id", Value: "1"},
			http.StatusInternalServerError, "application/json",
			"{\"error\":{\"message\":\"internal server error\"},\"data\":{\"id\":0,\"name\":\"\"}}\n",
		},
		// ...but a POINTER to a zero struct escapes the short circuit, because
		// isEmptyStruct dereferences for the Kind check yet compares the
		// original pointer against a zero STRUCT. Different behavior for
		// semantically identical data.
		{
			"pointer-to-empty-struct-plus-error", http.MethodGet, &charStruct{}, ErrorEntityNotFound{Name: "id", Value: "1"},
			http.StatusPartialContent, "application/json",
			"{\"error\":{\"message\":\"No entity found with id: 1\"},\"data\":{\"id\":0,\"name\":\"\"}}\n",
		},
		// A zero struct whose fields are all omitempty still serializes as `{}`
		// under `data`, so the client cannot distinguish it from "no data".
		{
			"empty-omitempty-struct-plus-error", http.MethodGet, charOmit{}, errCharPlain,
			http.StatusInternalServerError, "application/json",
			"{\"error\":{\"message\":\"internal server error\"},\"data\":{}}\n",
		},
	})
}

// TestResponder_Char_JSONEscaping pins encoding/json's default HTML escaping.
// GoFr uses json.Encoder without SetEscapeHTML(false), so <, > and & become
// \u003c, \u003e and \u0026 on the wire, and U+2028/U+2029 are escaped too.
func TestResponder_Char_JSONEscaping(t *testing.T) {
	runRespondCases(t, []respondCase{
		{
			"html-angle-brackets", http.MethodGet, "<script>alert(1)</script>", nil,
			http.StatusOK, "application/json",
			"{\"data\":\"\\u003cscript\\u003ealert(1)\\u003c/script\\u003e\"}\n",
		},
		{
			"ampersand", http.MethodGet, "a & b", nil,
			http.StatusOK, "application/json", "{\"data\":\"a \\u0026 b\"}\n",
		},
		{
			"double-quote-and-backslash", http.MethodGet, `he said "hi" \ bye`, nil,
			http.StatusOK, "application/json", "{\"data\":\"he said \\\"hi\\\" \\\\ bye\"}\n",
		},
		{
			"control-chars", http.MethodGet, "line1\nline2\ttab", nil,
			http.StatusOK, "application/json", "{\"data\":\"line1\\nline2\\ttab\"}\n",
		},
		// Non-ASCII is emitted as raw UTF-8, not \u escapes.
		{
			"unicode-passthrough", http.MethodGet, "héllo 世界 🚀", nil,
			http.StatusOK, "application/json", "{\"data\":\"héllo 世界 🚀\"}\n",
		},
		// U+2028 LINE SEPARATOR / U+2029 PARAGRAPH SEPARATOR are escaped so the
		// body is safe to embed in a <script> tag.
		{
			"line-separator-escaped", http.MethodGet, "a\u2028b\u2029c", nil,
			http.StatusOK, "application/json", "{\"data\":\"a\\u2028b\\u2029c\"}\n",
		},
		// Escaping applies to map KEYS too.
		{
			"escaped-map-key", http.MethodGet, map[string]string{"<k>": "&v"}, nil,
			http.StatusOK, "application/json", "{\"data\":{\"\\u003ck\\u003e\":\"\\u0026v\"}}\n",
		},
		// ...and to the error message.
		{
			//nolint:err113 // characterizing a caller-supplied ad-hoc error.
			"escaped-error-message", http.MethodGet, nil, errors.New("bad <input> & stuff"),
			http.StatusInternalServerError, "application/json",
			"{\"error\":{\"message\":\"bad \\u003cinput\\u003e \\u0026 stuff\"}}\n",
		},
		// Raw bypasses the envelope but NOT the escaping.
		{
			"raw-is-still-escaped", http.MethodGet, resTypes.Raw{Data: "<b>"}, nil,
			http.StatusOK, "application/json", "\"\\u003cb\\u003e\"\n",
		},
	})
}

// TestResponder_Char_ContentTypeNotOverwritten pins that a Content-Type already
// set on the writer is preserved — Respond only defaults it when unset.
func TestResponder_Char_ContentTypeNotOverwritten(t *testing.T) {
	tests := []struct {
		name    string
		preset  string
		wantCTy string
	}{
		{"unset-defaults-to-json", "", "application/json"},
		{"preset-preserved", "application/vnd.api+json", "application/vnd.api+json"},
		{"preset-text-preserved", "text/plain", "text/plain"},
		{"preset-with-charset-preserved", "application/json; charset=utf-8", "application/json; charset=utf-8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if tc.preset != "" {
				w.Header().Set("Content-Type", tc.preset)
			}

			NewResponder(w, http.MethodGet).Respond("x", nil)

			assert.Equal(t, tc.wantCTy, w.Header().Get("Content-Type"))
			// The body is unaffected by the Content-Type; it is always JSON.
			assert.JSONEq(t, `{"data":"x"}`, w.Body.String())
		})
	}
}

// TestResponder_Char_EncodeFailure pins the fallback written when the payload
// cannot be JSON-encoded. Note the body has a space after "message": and the
// status is written AFTER Content-Type has already been set to application/json.
func TestResponder_Char_EncodeFailure(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{"channel", make(chan int)},
		{"func", func() {}},
		{"nan", math.NaN()},
		{"inf", math.Inf(1)},
		{"map-with-unencodable-value", map[string]any{"c": make(chan int)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			NewResponder(w, http.MethodGet).Respond(tc.data, nil)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			//nolint:testifylint // exact bytes are the contract: note the space after the colon.
			assert.Equal(t, "{\"error\":{\"message\": \"failed to encode response as JSON\"}}\n", w.Body.String())
		})
	}
}

// TestResponder_Char_File pins the File special response type.
func TestResponder_Char_File(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		file       resTypes.File
		err        error
		wantStatus int
		wantCType  string
		wantBody   string
	}{
		{
			"png-get", http.MethodGet,
			resTypes.File{Content: []byte{0x89, 'P', 'N', 'G'}, ContentType: "image/png"}, nil,
			http.StatusOK, "image/png", "\x89PNG",
		},
		{
			"post-is-201", http.MethodPost,
			resTypes.File{Content: []byte("x"), ContentType: "text/csv"}, nil,
			http.StatusCreated, "text/csv", "x",
		},
		{
			"delete-is-204", http.MethodDelete,
			resTypes.File{Content: []byte("x"), ContentType: "text/csv"}, nil,
			http.StatusNoContent, "text/csv", "x",
		},
		// Empty ContentType is written verbatim as an empty header value rather
		// than being defaulted or sniffed.
		{
			"empty-content-type", http.MethodGet,
			resTypes.File{Content: []byte("x")}, nil,
			http.StatusOK, "", "x",
		},
		{
			"empty-content", http.MethodGet,
			resTypes.File{ContentType: "text/plain"}, nil,
			http.StatusOK, "text/plain", "",
		},
		// With an error the status comes from the error, NOT 206 — and the body
		// is still the file bytes, so the client gets no error detail at all.
		{
			"with-status-coded-error", http.MethodGet,
			resTypes.File{Content: []byte("x"), ContentType: "text/plain"}, ErrorEntityNotFound{Name: "id", Value: "1"},
			http.StatusNotFound, "text/plain", "x",
		},
		{
			"with-plain-error", http.MethodGet,
			resTypes.File{Content: []byte("x"), ContentType: "text/plain"}, errCharPlain,
			http.StatusInternalServerError, "text/plain", "x",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			NewResponder(w, tc.method).Respond(tc.file, tc.err)

			assert.Equal(t, tc.wantStatus, w.Code, "status code")
			assert.Equal(t, tc.wantCType, w.Header().Get("Content-Type"), "Content-Type")
			assert.Equal(t, tc.wantBody, w.Body.String(), "body bytes")
		})
	}
}

// TestResponder_Char_XML pins the XML special response type, including the
// application/xml default and the "no write for empty content" branch.
func TestResponder_Char_XML(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		xml        resTypes.XML
		err        error
		wantStatus int
		wantCType  string
		wantBody   string
	}{
		{
			"default-content-type", http.MethodGet,
			resTypes.XML{Content: []byte("<a>1</a>")}, nil,
			http.StatusOK, "application/xml", "<a>1</a>",
		},
		{
			"explicit-content-type", http.MethodGet,
			resTypes.XML{Content: []byte("<a/>"), ContentType: "application/rss+xml"}, nil,
			http.StatusOK, "application/rss+xml", "<a/>",
		},
		{
			"empty-content-writes-nothing", http.MethodGet,
			resTypes.XML{}, nil,
			http.StatusOK, "application/xml", "",
		},
		{
			"post-is-201", http.MethodPost,
			resTypes.XML{Content: []byte("<a/>")}, nil,
			http.StatusCreated, "application/xml", "<a/>",
		},
		{
			"delete-is-204", http.MethodDelete,
			resTypes.XML{Content: []byte("<a/>")}, nil,
			http.StatusNoContent, "application/xml", "<a/>",
		},
		// XML content is written verbatim — no escaping, no envelope.
		{
			"content-written-verbatim", http.MethodGet,
			resTypes.XML{Content: []byte("<a>&amp; \"x\" <b/></a>")}, nil,
			http.StatusOK, "application/xml", "<a>&amp; \"x\" <b/></a>",
		},
		{
			"with-status-coded-error", http.MethodGet,
			resTypes.XML{Content: []byte("<a/>")}, ErrorInvalidParam{Params: []string{"p"}},
			http.StatusBadRequest, "application/xml", "<a/>",
		},
		{
			"with-plain-error", http.MethodGet,
			resTypes.XML{Content: []byte("<a/>")}, errCharPlain,
			http.StatusInternalServerError, "application/xml", "<a/>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			NewResponder(w, tc.method).Respond(tc.xml, tc.err)

			assert.Equal(t, tc.wantStatus, w.Code, "status code")
			assert.Equal(t, tc.wantCType, w.Header().Get("Content-Type"), "Content-Type")
			assert.Equal(t, tc.wantBody, w.Body.String(), "body bytes")
		})
	}
}

// TestResponder_Char_Redirect pins the Redirect special response type. The
// status depends ONLY on the HTTP method, never on the error, and no body is
// written.
func TestResponder_Char_Redirect(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		url        string
		err        error
		wantStatus int
	}{
		{"get-is-302", http.MethodGet, "/target", nil, http.StatusFound},
		{"head-is-302", http.MethodHead, "/target", nil, http.StatusFound},
		{"delete-is-302", http.MethodDelete, "/target", nil, http.StatusFound},
		{"options-is-302", http.MethodOptions, "/target", nil, http.StatusFound},
		{"post-is-303", http.MethodPost, "/target", nil, http.StatusSeeOther},
		{"put-is-303", http.MethodPut, "/target", nil, http.StatusSeeOther},
		{"patch-is-303", http.MethodPatch, "/target", nil, http.StatusSeeOther},
		// The error is ignored completely for redirects.
		{"get-with-error-still-302", http.MethodGet, "/target", ErrorEntityNotFound{}, http.StatusFound},
		{"post-with-error-still-303", http.MethodPost, "/target", errCharPlain, http.StatusSeeOther},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			NewResponder(w, tc.method).Respond(resTypes.Redirect{URL: tc.url}, tc.err)

			assert.Equal(t, tc.wantStatus, w.Code, "status code")
			assert.Equal(t, tc.url, w.Header().Get("Location"), "Location")
			assert.Empty(t, w.Body.String(), "redirect must write no body")
			// Content-Type is never set for a redirect.
			assert.Empty(t, w.Header().Get("Content-Type"), "Content-Type")
		})
	}
}

// TestResponder_Char_RedirectURLVerbatim pins that the Location header is the
// URL string exactly as given — no normalization, no escaping, no validation.
func TestResponder_Char_RedirectURLVerbatim(t *testing.T) {
	for _, url := range []string{
		"",
		"https://example.com/a?b=c&d=e#frag",
		"/relative path with spaces",
		"javascript:alert(1)",
	} {
		w := httptest.NewRecorder()

		NewResponder(w, http.MethodGet).Respond(resTypes.Redirect{URL: url}, nil)

		assert.Equal(t, url, w.Header().Get("Location"))
	}
}

// TestResponder_Char_Template pins the Template special response type: the
// Content-Type is always text/html (never charset-qualified) and the rendered
// bytes are html/template output with its own contextual escaping.
func TestResponder_Char_Template(t *testing.T) {
	createTemplateFile(t, "./templates/char.html", "<h1>{{.Title}}</h1>")

	defer removeTemplateDir(t)

	tests := []struct {
		name       string
		method     string
		data       any
		err        error
		wantStatus int
		wantBody   string
	}{
		{"get-is-200", http.MethodGet, map[string]string{"Title": "Hi"}, nil, http.StatusOK, "<h1>Hi</h1>"},
		{"post-is-201", http.MethodPost, map[string]string{"Title": "Hi"}, nil, http.StatusCreated, "<h1>Hi</h1>"},
		// LATENT BUG (pinned as-is): a Template returned from a DELETE handler
		// gets status 204, and the HTTP layer rejects a body on a 204. Because
		// html/template writes incrementally and Render discards Execute's
		// error, the client receives a TRUNCATED page (only the first text
		// node) rather than either the full page or an error.
		{"delete-is-204", http.MethodDelete, map[string]string{"Title": "Hi"}, nil, http.StatusNoContent, "<h1>"},
		{
			"with-status-coded-error", http.MethodGet, map[string]string{"Title": "Hi"},
			ErrorEntityNotFound{}, http.StatusNotFound, "<h1>Hi</h1>",
		},
		{
			"with-plain-error", http.MethodGet, map[string]string{"Title": "Hi"},
			errCharPlain, http.StatusInternalServerError, "<h1>Hi</h1>",
		},
		// html/template applies contextual escaping of its own — this is NOT the
		// JSON \u003c escaping used by the envelope path.
		{
			"html-escaped-by-html-template", http.MethodGet, map[string]string{"Title": "<b>&x</b>"},
			nil, http.StatusOK, "<h1>&lt;b&gt;&amp;x&lt;/b&gt;</h1>",
		},
		// A missing field renders as the literal Go zero-value marker.
		{"missing-field", http.MethodGet, map[string]string{}, nil, http.StatusOK, "<h1></h1>"},
		{"nil-data", http.MethodGet, nil, nil, http.StatusOK, "<h1></h1>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			NewResponder(w, tc.method).Respond(resTypes.Template{Name: "char.html", Data: tc.data}, tc.err)

			assert.Equal(t, tc.wantStatus, w.Code, "status code")
			assert.Equal(t, "text/html", w.Header().Get("Content-Type"), "Content-Type")
			assert.Equal(t, tc.wantBody, w.Body.String(), "body bytes")
		})
	}
}

// TestResponder_Char_TemplatePointerIsNotSpecial pins a sharp edge: the type
// switch matches resTypes.Template by VALUE, so returning *resTypes.Template
// falls through to the JSON envelope instead of rendering. Reported, not fixed.
func TestResponder_Char_TemplatePointerIsNotSpecial(t *testing.T) {
	w := httptest.NewRecorder()

	NewResponder(w, http.MethodGet).Respond(&resTypes.Template{Name: "char.html", Data: "x"}, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"data\":{\"Data\":\"x\",\"Name\":\"char.html\"}}\n", w.Body.String())
}

// TestResponder_Char_SpecialTypePointersFallThrough pins that every special
// response type is matched by value only; a pointer to one is JSON-encoded.
func TestResponder_Char_SpecialTypePointersFallThrough(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		wantBody string
	}{
		{"file-ptr", &resTypes.File{Content: []byte("ab"), ContentType: "text/plain"},
			"{\"data\":{\"Content\":\"YWI=\",\"ContentType\":\"text/plain\"}}\n"},
		{"xml-ptr", &resTypes.XML{Content: []byte("<a/>")},
			"{\"data\":{\"Content\":\"PGEvPg==\",\"ContentType\":\"\"}}\n"},
		{"redirect-ptr", &resTypes.Redirect{URL: "/x"}, "{\"data\":{\"URL\":\"/x\"}}\n"},
		{"raw-ptr", &resTypes.Raw{Data: "x"}, "{\"data\":{\"Data\":\"x\"}}\n"},
		{"response-ptr", &resTypes.Response{Data: "x"}, "{\"data\":{\"data\":\"x\"}}\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			NewResponder(w, http.MethodGet).Respond(tc.data, nil)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.Equal(t, tc.wantBody, w.Body.String())
		})
	}
}

// TestResponder_Char_StreamHeaders pins the full header set and status of a
// Stream response, and that an error returned alongside a Stream is ignored.
func TestResponder_Char_StreamHeaders(t *testing.T) {
	t.Run("sse", func(t *testing.T) {
		w := httptest.NewRecorder()

		NewResponder(w, http.MethodPost).
			Respond(resTypes.Stream{Source: &sliceSource{items: []any{"a"}}}, ErrorEntityNotFound{})

		// Always 200, even for POST and even with an error present.
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
		assert.Equal(t, "data: \"a\"\n\ndata: [DONE]\n\n", w.Body.String())
	})

	t.Run("ndjson", func(t *testing.T) {
		w := httptest.NewRecorder()

		NewResponder(w, http.MethodGet).
			Respond(resTypes.Stream{Source: &sliceSource{items: []any{"a"}}, Format: resTypes.NDJSON}, nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/x-ndjson", w.Header().Get("Content-Type"))
		// NDJSON does NOT set Cache-Control / Connection.
		assert.Empty(t, w.Header().Get("Cache-Control"))
		assert.Empty(t, w.Header().Get("Connection"))
		// NDJSON has no [DONE] terminator.
		assert.Equal(t, "\"a\"\n", w.Body.String())
	})

	t.Run("nil-source", func(t *testing.T) {
		w := httptest.NewRecorder()

		NewResponder(w, http.MethodGet).Respond(resTypes.Stream{}, nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		// No Content-Type is set on the nil-source path.
		assert.Empty(t, w.Header().Get("Content-Type"))
		//nolint:testifylint // exact bytes are the contract.
		assert.Equal(t, "{\"error\":{\"message\":\"stream source is nil\"}}\n", w.Body.String())
	})
}

// TestResponder_Char_NoBodyForDeleteIsStillWritten pins that GoFr writes a JSON
// body even with 204 No Content, which is a protocol violation net/http will
// suppress on a real connection but which the responder itself does emit.
func TestResponder_Char_NoBodyForDeleteIsStillWritten(t *testing.T) {
	w := httptest.NewRecorder()

	NewResponder(w, http.MethodDelete).Respond(map[string]string{"k": "v"}, nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"data\":{\"k\":\"v\"}}\n", w.Body.String())
}

// TestResponder_Char_MethodCaseSensitivity pins that method matching is exact —
// a lowercase "post" does not get the 201 mapping.
func TestResponder_Char_MethodCaseSensitivity(t *testing.T) {
	w := httptest.NewRecorder()

	NewResponder(w, "post").Respond("x", nil)

	assert.Equal(t, http.StatusOK, w.Code)
}
