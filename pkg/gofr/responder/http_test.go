package responder

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	gofrErrors "gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/template"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

// CustomError to be used for Err field in errors.Raw
type CustomError struct {
	Message string `json:"message"`
}

func (e CustomError) Error() string {
	return e.Message
}

func TestNewContextualResponder(t *testing.T) {
	var (
		w             = httptest.NewRecorder()
		correlationID = "50deb4921d7bc1b441bb992c4874a147"
	)

	path := "/dummy"
	testCases := []struct {
		contentType         string
		correlationIDHeader string
		want                Responder
	}{
		{"", "X-B3-TraceID", &HTTP{w: w, resType: JSON, method: "GET", path: path, correlationID: correlationID}},
		{"text/xml", "X-B3-TraceID", &HTTP{w: w, resType: XML, method: "GET", path: path, correlationID: correlationID}},
		{"application/xml", "X-B3-TraceID", &HTTP{w: w, resType: XML, method: "GET", path: path, correlationID: correlationID}},
		{"text/json", "X-Correlation-ID", &HTTP{w: w, resType: JSON, method: "GET", path: path, correlationID: correlationID}},
		{"application/json", "X-Correlation-ID", &HTTP{w: w, resType: JSON, method: "GET", path: path, correlationID: correlationID}},
		{"text/plain", "X-Correlation-ID", &HTTP{w: w, resType: TEXT, method: "GET", path: path, correlationID: correlationID}},
	}

	for _, tc := range testCases {
		r := httptest.NewRequest("GET", "/dummy", http.NoBody)

		// handler to set the routeKey in request context
		handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			r = req
		})

		muxRouter := mux.NewRouter()
		muxRouter.NewRoute().Path(r.URL.Path).Methods("GET").Handler(handler)
		muxRouter.ServeHTTP(w, r)

		*r = *r.Clone(context.WithValue(r.Context(), middleware.CorrelationIDKey, correlationID))

		r.Header.Set("Content-Type", tc.contentType)
		r.Header.Set(tc.correlationIDHeader, correlationID)

		if got := NewContextualResponder(w, r); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("NewContextualResponder() = %v, want %v", got, tc.want)
		}
	}
}

func TestHTTP_Respond(t *testing.T) {
	createDefaultTemplate()

	defer deleteDefaultTemplate()

	data := struct {
		Title string
		Items []string
	}{
		Title: "Default Gofr Template",
		Items: []string{
			"Welcome to Gofr",
		},
	}

	w := httptest.NewRecorder()

	type args struct {
		statusCode int
		data       interface{}
	}

	testCases := []struct {
		desc    string
		resType responseType
		args    args
		want    string
	}{
		{"Case: Internal Server Error", 999, args{500, template.Template{Directory: "./", File: "default.html",
			Data: "test data", Type: template.HTML}}, ""},
		{"Case: Status OK", 999, args{200, template.Template{Directory: "./", File: "default.html",
			Data: data, Type: template.HTML}}, "text/html"},
		{"when input and output content-type is json", JSON, args{200, `{"name": "gofr"}`}, "application/json"},
		{"when input and output content-type is xml", XML, args{200, `<name>gofr</name>`}, "application/xml"},
		{"when input and output content-type is text", TEXT, args{200, `name: gofr`}, "text/plain"},
		{"file content-type is text/html", TEXT, args{200,
			template.File{Content: []byte(`<html></html>`), ContentType: "text/html"}}, "text/html"},
		{"contentType provided is application/xml", TEXT, args{200,
			types.RawWithOptions{Data: "Hello World", ContentType: "text/xml"}}, "application/xml"},
		{"contentType provided is application/json", TEXT, args{200,
			types.RawWithOptions{Data: "Hello World", ContentType: "application/json"}}, "application/json"},
		{"contentType provided is text/plain", XML, args{200,
			types.RawWithOptions{Data: "Hello World", ContentType: "text/plain"}}, "text/plain"},
		{"contentType provided is text/html", JSON, args{200,
			types.RawWithOptions{Data: "Hello World", ContentType: "text/html"}}, "application/json"},
		{"contentType provided is empty", TEXT, args{200,
			types.RawWithOptions{Data: "Hello World", ContentType: ""}}, "text/plain"},
	}

	for i, tc := range testCases {
		h := HTTP{
			w:       w,
			resType: tc.resType,
		}
		h.Respond(tc.args.data, nil)

		if got := h.w.Header().Get("Content-Type"); got != tc.want {
			t.Errorf("TestCase %d Failed: got %v, want: %v", i, got, tc.want)
		}
	}
}

func createDefaultTemplate() {
	rootDir, _ := os.Getwd()
	logger := log.NewLogger()
	f, err := os.Create(rootDir + "/default.html")

	if err != nil {
		logger.Error(err)
	}

	_, err = f.WriteString(`<!DOCTYPE html>
	<html>
	<head>
	<meta charset="UTF-8">
	<title>{{.Title}}</title>
	</head>
	<body>
	{{range .Items}}<div>{{ . }}</div>{{else}}<div><strong>no rows</strong></div>{{end}}
	</body>
	</html>`)

	if err != nil {
		logger.Error(err)
	} else {
		logger.Info("Template created!")
	}

	err = f.Close()
	if err != nil {
		logger.Error(err)
	}
}

func deleteDefaultTemplate() {
	rootDir, _ := os.Getwd()
	logger := log.NewLogger()
	err := os.Remove(rootDir + "/default.html")

	if err != nil {
		logger.Error(err)
	}
}

func TestHTTP_Respond_PartialError(t *testing.T) {
	w := httptest.NewRecorder()

	type args struct {
		statusCode int
		data       interface{}
		err        error
	}

	testCases := []struct {
		resType responseType
		args    args
		want    string
	}{
		{JSON, args{206, map[string]interface{}{"name": "Alice"}, gofrErrors.EntityNotFound{
			Entity: "store",
			ID:     "1",
		}}, "application/json"},
	}

	for _, tc := range testCases {
		h := HTTP{
			w:       w,
			resType: tc.resType,
		}
		h.Respond(tc.args.data, tc.args.err)

		if got := h.w.Header().Get("Content-Type"); got != tc.want {
			t.Errorf("got %v, want: %v", got, tc.want)
		}
	}
}

// Test_getResponse_Raw to test getResponse when MultipleErrors containing errors.Raw is passed in args
func Test_getResponse_Raw(t *testing.T) {
	testErr := gofrErrors.Raw{StatusCode: http.StatusInternalServerError, Err: CustomError{Message: "Test Error"}}
	testErrWithoutErr := gofrErrors.Raw{StatusCode: http.StatusInternalServerError}

	testCases := []struct {
		desc   string
		err    error
		expErr error
	}{
		{"Err field's value of errors. Raw is passed should be returned",
			gofrErrors.MultipleErrors{StatusCode: http.StatusInternalServerError, Errors: []error{testErr}}, CustomError{Message: "Test Error"}},
		{"Raw error with nil Err field is passed",
			gofrErrors.MultipleErrors{StatusCode: http.StatusInternalServerError, Errors: []error{testErrWithoutErr}},
			gofrErrors.Error("Unknown Error")},
		{"Errors field of MultipleErrors is empty", gofrErrors.MultipleErrors{StatusCode: http.StatusInternalServerError}, nil},
	}

	for i, tc := range testCases {
		got := getResponse(&types.Response{}, tc.err)

		assert.Equalf(t, tc.expErr, got, "Test[%d] Failed: %v", i+1, tc.desc)
	}
}

func TestSetHeaders(t *testing.T) {
	testCase := []struct {
		desc      string
		header    map[string]string
		expHeader http.Header
	}{
		{"Header will not Set", map[string]string{"Content-Type": "application/json"}, http.Header(map[string][]string{})},
		{"Header will Set", map[string]string{"Test": "test"}, http.Header(map[string][]string{"Test": {"test"}})},
		{"Header will Set for test not for content-type and x-correlation-id",
			map[string]string{"Content-Type": "text/plain", "Test": "test", "x-correlation-id": "123"},
			http.Header(map[string][]string{"Test": {"test"}})},
	}
	for i, tc := range testCase {
		w := httptest.NewRecorder()
		setHeaders(tc.header, w)
		assert.Equalf(t, tc.expHeader, w.Header(), "TestCase: %d Failed:%v", i, tc.desc)
	}
}

func TestGetResponseContentType(t *testing.T) {
	defaultType := JSON

	testCase := []struct {
		contentType string
		expRespType responseType
	}{
		{"text/xml", XML},
		{"application/xml", XML},
		{"text/plain", TEXT},
		{"application/json", JSON},
		{"unknown/type", defaultType},
	}

	for _, tc := range testCase {
		got := getResponseContentType(tc.contentType, defaultType)
		if got != tc.expRespType {
			assert.Equalf(t, tc.expRespType, tc.contentType, "Expected %s for content type %s, but got %s", tc.expRespType, tc.contentType, got)
		}
	}
}

func TestGetStatusCode(t *testing.T) {
	testCases := []struct {
		desc      string
		method    string
		data      interface{}
		err       error
		expStatus int
	}{
		{"NoErrorMethodPost", http.MethodPost, nil, nil, http.StatusCreated},
		{"NoErrorMethodDelete", http.MethodDelete, nil, nil, http.StatusNoContent},
		{"PartialContentErrorWithData", http.MethodGet, "some data", gofrErrors.MultipleErrors{}, http.StatusPartialContent},
		{"PartialContentErrorWithoutData", http.MethodGet, nil, gofrErrors.MultipleErrors{}, http.StatusInternalServerError},
		{"CustomError", http.MethodGet, nil, errors.New("some error"), http.StatusOK},
	}

	for i, tc := range testCases {
		statusCode := getStatusCode(tc.method, tc.data, tc.err)

		assert.Equalf(t, tc.expStatus, statusCode, "Test[%d],failed:%v", i, tc.desc)
	}
}

func TestProcessTemplateError(t *testing.T) {
	testCases := []struct {
		desc           string
		err            error
		expectedStatus int
	}{
		{"FileNotFound", gofrErrors.FileNotFound{}, http.StatusNotFound},
		{"UnknownError", errors.New("Unknown error"), http.StatusInternalServerError},
	}

	for i, tc := range testCases {
		var h HTTP

		h.w = httptest.NewRecorder()
		h.processTemplateError(tc.err)

		assert.Equalf(t, tc.expectedStatus, h.w.(*httptest.ResponseRecorder).Code, "Test[%d],failed:%v", i, tc.desc)
	}
}

func TestGetResponse(t *testing.T) {
	testCases := []struct {
		desc          string
		response      *types.Response
		err           error
		expectedValue interface{}
	}{
		{"ResponseWithMultipleErrorstype", &types.Response{}, gofrErrors.MultipleErrors{
			StatusCode: 400, Errors: []error{errors.New("error 1"), errors.New("error 2")}},
			gofrErrors.MultipleErrors{StatusCode: 400, Errors: []error{errors.New("error 1"), errors.New("error 2")}}},
		{"case when response is nil and error is custom error", nil, errors.New("some error"), (*types.Response)(nil)},
		{"case when response and error of MultipleErrors type both are passed", &types.Response{Data: map[string]interface{}{"key": "value"}},
			gofrErrors.MultipleErrors{StatusCode: 400, Errors: []error{errors.New("error 1"), errors.New("error 2")}},
			types.Response{Data: map[string]interface{}{"key": "value", "errors": []error{errors.New("error 1"),
				errors.New("error 2")}}, Meta: nil}},
		{"case when response and error of MultipleErrors type with empty errors field both are passed",
			&types.Response{Data: "some data"},
			gofrErrors.MultipleErrors{StatusCode: 400},
			types.Response{Data: map[string]interface{}{"errors": []error(nil)}, Meta: interface{}(nil)}},
		{"case when error and response both are empty", &types.Response{}, gofrErrors.MultipleErrors{}, nil},
	}

	for i, tc := range testCases {
		result := getResponse(tc.response, tc.err)

		assert.Equalf(t, tc.expectedValue, result, "Test[%d] failed:%v", i, tc.desc)
	}
}
