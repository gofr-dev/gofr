package gofr

import (
	ctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	gofrErrors "gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/responder"
	"gofr.dev/pkg/gofr/types"
)

// CustomError to be used for Err field in errors.Raw
type CustomError struct {
	Message string `json:"message"`
}

func (e CustomError) Error() string {
	return e.Message
}

// routeKeySetter is used to set the routKey in the request context
func routeKeySetter(w http.ResponseWriter, r *http.Request) *http.Request {
	// dummy handler for setting routeKey
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r = req
	})

	muxRouter := mux.NewRouter()
	muxRouter.NewRoute().Path(r.URL.Path).Methods(r.Method).Handler(handler)
	muxRouter.ServeHTTP(w, r)

	return r
}

// TestHandler_ServeHTTP_StatusCode tests the different combination of statusCode and errors.
func TestHandler_ServeHTTP_StatusCode(t *testing.T) {
	testCases := []struct {
		error      error
		statusCode int
		code       string
		data       interface{}
	}{
		{gofrErrors.InvalidParam{Param: []string{"organizationId"}}, http.StatusBadRequest, "Invalid Parameter", nil},
		{gofrErrors.EntityAlreadyExists{}, http.StatusOK, "", "some data"},
		{gofrErrors.EntityNotFound{Entity: "user", ID: "2"}, http.StatusNotFound, "Entity Not Found", nil},
		{errors.New("unexpected response from internal dependency"), http.StatusInternalServerError, "Internal Server Error", nil},
		{nil, http.StatusOK, "", nil},
		{gofrErrors.MissingParam{Param: []string{"organizationId"}}, http.StatusBadRequest, "Missing Parameter", nil},
		{gofrErrors.MissingParam{Param: []string{"organizationId"}}, http.StatusPartialContent, "Missing Parameter",
			map[string]interface{}{"name": "Alice"}},
		{gofrErrors.MissingParam{Param: []string{"organizationId"}}, http.StatusBadRequest, "Missing Parameter", types.Response{}},
		{nil, http.StatusOK, "", &types.Response{Data: map[string]interface{}{"name": "Alice"}}},
		{gofrErrors.DB{}, http.StatusInternalServerError, "DB Error", nil},
		{gofrErrors.Raw{StatusCode: http.StatusInternalServerError, Err: CustomError{Message: "Test Error"}},
			http.StatusInternalServerError, `{"message":"Test Error"}`, nil},
	}

	for i, tc := range testCases {
		tc := tc
		g := New()
		w := newCustomWriter()
		r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
		r = routeKeySetter(w, r)
		req := request.NewHTTPRequest(r)
		resp := responder.NewContextualResponder(w, r)
		*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

		Handler(func(c *Context) (interface{}, error) {
			return tc.data, tc.error
		}).ServeHTTP(w, r)

		if w.Status != tc.statusCode {
			t.Errorf("TestCase[%v]\nIncorrect status code: \nGot\n%v\nExpected\n%v\n", i, w.Status, tc.statusCode)
		}

		if tc.code != "" && !strings.Contains(w.Body, tc.code) {
			t.Errorf("FAILED, Expected %v in response body", tc.code)
		}
	}
}

// TestHandler_ServeHTTP_ErrorFormat
func TestHandler_ServeHTTP_ErrorFormat(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	Handler(func(c *Context) (interface{}, error) {
		return nil, &gofrErrors.Response{StatusCode: 400, Code: "Invalid name", Reason: "The name in the parameter is incorrect"}
	}).ServeHTTP(w, r)

	e := struct {
		Errors []gofrErrors.Response `json:"errors"`
	}{[]gofrErrors.Response{}}

	_ = json.Unmarshal([]byte(w.Body), &e)

	if len(e.Errors) != 1 && e.Errors[0].Code != "Invalid name" &&
		e.Errors[0].Reason != "The name in the parameter is incorrect" && e.Errors[0].Detail != 1 {
		t.Errorf("Error formating failed.")
	}
}

// TestHandler_ServeHTTP_RawErrorFormat to test error format for errors.Raw response
func TestHandler_ServeHTTP_RawErrorFormat(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	testCases := []struct {
		desc   string
		err    error
		expRes string
	}{
		{"Success case", gofrErrors.Raw{StatusCode: http.StatusInternalServerError, Err: CustomError{Message: "Test Error"}},
			`{"message":"Test Error"}`},
		{"No error is set to Raw error", gofrErrors.Raw{StatusCode: http.StatusInternalServerError}, "Unknown Error"},
		{"No error is set to Errors of MutlipleErrors", gofrErrors.MultipleErrors{StatusCode: http.StatusInternalServerError}, ""},
	}

	for i, tc := range testCases {
		Handler(func(c *Context) (interface{}, error) {
			return nil, tc.err
		}).ServeHTTP(w, r)

		assert.Containsf(t, w.Body, tc.expRes, "Test[%d] Failed: %v", i+1, tc.desc)
	}
}

// TestHandler_ServeHTTP_Content_Type tests the JSON content type for a response
func TestHandler_ServeHTTP_Content_Type(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	Handler(func(c *Context) (interface{}, error) {
		return "hi", nil
	}).ServeHTTP(w, r)

	contentType := "application/json"
	if w.Headers.Get("Content-Type") != contentType {
		t.Errorf("Response and Request Content Type does not match")
	}
}

// TestHandler_ServeHTTP
func TestHandler_ServeHTTP(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))
	Handler(func(c *Context) (interface{}, error) {
		p := product{Name: "Orange", CategoryID: 1}
		data := struct {
			Product product `json:"product"`
		}{p}

		return data, nil
	}).ServeHTTP(w, r)

	expectedResponse := []byte(`{"data":{"product":{"name":"Orange","categoryId":1}}}`)

	assert.Equal(t, string(expectedResponse), strings.TrimSpace(w.Body), "TEST Failed.\n")
}

type product struct {
	Name       string `json:"name"`
	CategoryID int    `json:"categoryId"`
}

// TestHandler_ServeHTTP_XML tests the XML content type for a response
func TestHandler_ServeHTTP_XML(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r.Header.Add("Content-type", "application/xml")
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))
	expectedError := gofrErrors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected occurred"}

	Handler(func(c *Context) (interface{}, error) {
		return nil, errors.New("something unexpected occurred")
	}).ServeHTTP(w, r)

	if !strings.Contains(w.Body, expectedError.Reason) || !strings.Contains(w.Body, strconv.Itoa(expectedError.StatusCode)) {
		t.Errorf("Error formating failed for xml.")
	}
}

// TestHandler_ServeHTTP_Text tests the TEXT content type for a response
func TestHandler_ServeHTTP_Text(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r.Header.Add("Content-type", "text/plain")
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	Handler(func(c *Context) (interface{}, error) {
		return nil, errors.New("something unexpected occurred")
	}).ServeHTTP(w, r)

	if w.Body != "something unexpected occurred" {
		t.Errorf("Error formating failed")
	}
}

// TestHandler_ServeHTTP_PartialContent
func TestHandler_ServeHTTP_PartialContent(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))
	Handler(func(c *Context) (interface{}, error) {
		p := product{Name: "Orange", CategoryID: 1}
		data := struct {
			Product product `json:"product"`
		}{p}

		return data, &gofrErrors.Response{Reason: "test error", DateTime: gofrErrors.DateTime{Value: "2020-07-01T14:54:41Z", TimeZone: "IST"}}
	}).ServeHTTP(w, r)

	//nolint:lll // response should be of this type
	expectedResponse := []byte(`{"data":{"errors":[{"code":"","reason":"test error","datetime":{"value":"2020-07-01T14:54:41Z","timezone":"IST"}}],"product":{"categoryId":1,"name":"Orange"}}}`)

	assert.Equal(t, string(expectedResponse), strings.TrimSpace(w.Body), "TEST Failed.\n")

	assert.Equal(t, http.StatusPartialContent, w.Status, "TEST Failed.\n")
}

// TestHandler_ServeHTTP_EntityAlreadyExists
func TestHandler_ServeHTTP_EntityAlreadyExists(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest(http.MethodPost, "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	Handler(func(c *Context) (interface{}, error) {
		p := product{Name: "Orange", CategoryID: 1}
		data := struct {
			Product product `json:"product"`
		}{p}

		return data, gofrErrors.EntityAlreadyExists{}
	}).ServeHTTP(w, r)

	expectedResponse := []byte(`{"data":{"product":{"name":"Orange","categoryId":1}}}`)

	assert.Equal(t, string(expectedResponse), strings.TrimSpace(w.Body), "TEST Failed.\n")

	assert.Equal(t, http.StatusOK, w.Status, "TEST Failed.\n")
}

// Test_HealthInvalidMethod checks the health for method.
func Test_HealthInvalidMethod(t *testing.T) {
	testCases := []struct {
		error      error
		statusCode int
		code       string
		data       interface{}
		method     string
	}{
		{gofrErrors.MethodMissing{}, http.StatusMethodNotAllowed, "", nil, http.MethodPost},
		{nil, http.StatusOK, "", nil, "GET"},
	}

	for _, tc := range testCases {
		tc := tc
		g := New()
		w := newCustomWriter()
		r := httptest.NewRequest(tc.method, "/.well-known/health-check", http.NoBody)
		r = routeKeySetter(w, r)
		req := request.NewHTTPRequest(r)
		resp := responder.NewContextualResponder(w, r)
		*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

		Handler(func(c *Context) (interface{}, error) {
			return tc.data, tc.error
		}).ServeHTTP(w, r)

		if w.Status != tc.statusCode {
			t.Errorf("\nIncorrect status code: \nGot\n%v\nExpected\n%v\n", w.Status, tc.statusCode)
		}
	}
}

func testNil() (*types.Response, error) {
	return nil, gofrErrors.MissingParam{}
}

//nolint:unparam //there is only one case to test
func testEmptyStruct() (*product, error) {
	return nil, gofrErrors.InvalidParam{Param: []string{"filter"}}
}

// TestHTTP_Respond_Nil tests nil and empty struct response
func TestHTTP_Respond_Nil(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))
	{
		// Test for nil types.Response
		expectedError := gofrErrors.Response{StatusCode: http.StatusBadRequest, Code: "Missing Parameter",
			Reason: "This request is missing parameters"}

		Handler(func(c *Context) (interface{}, error) {
			return testNil()
		}).ServeHTTP(w, r)

		if !strings.Contains(w.Body, expectedError.Reason) || !strings.Contains(w.Body, expectedError.Code) {
			t.Errorf("Error formating failed.")
		}

		if w.Status != expectedError.StatusCode {
			t.Errorf("Failed. StatusCode expected: 400, got: %v", w.Status)
		}
	}

	{
		// Test for empty struct, where type is non nil but value is nil
		expectedError := gofrErrors.Response{StatusCode: http.StatusBadRequest, Code: "Invalid Parameter",
			Reason: "Incorrect value for parameter: filter"}

		Handler(func(c *Context) (interface{}, error) {
			return testEmptyStruct()
		}).ServeHTTP(w, r)

		if !strings.Contains(w.Body, expectedError.Reason) || !strings.Contains(w.Body, expectedError.Code) {
			t.Errorf("Error formating failed.")
		}

		if w.Status != expectedError.StatusCode {
			t.Errorf("Failed. StatusCode expected: 400, got: %v", w.Status)
		}
	}
}

// TestHTTP_Respond_Delete tests the DELETE method
func TestHTTP_Respond_Delete(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest(http.MethodDelete, "/delete", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	Handler(func(c *Context) (interface{}, error) {
		return nil, nil
	}).ServeHTTP(w, r)

	expectedResponse := []byte(`{"data":null}`)

	assert.Equal(t, string(expectedResponse), strings.TrimSpace(w.Body), "TEST Failed.\n")

	assert.Equal(t, http.StatusNoContent, w.Status, "TEST Failed.\n")
}

// TestHandler_ServeHTTP_Error tests the different error cases returned from Respond
func TestHandler_ServeHTTP_Error(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/error", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	{
		// Error is present but only status code is set and no body
		expectedError := gofrErrors.Response{StatusCode: http.StatusInternalServerError}
		Handler(func(c *Context) (interface{}, error) {
			return nil, &gofrErrors.Response{StatusCode: http.StatusInternalServerError}
		}).ServeHTTP(w, r)

		if w.Status != expectedError.StatusCode {
			t.Errorf("Failed StatusCode expected: %v, got: %v", expectedError.StatusCode, w.Status)
		}
	}

	{
		// Error is set and returned in the body
		expectedError := gofrErrors.Response{StatusCode: http.StatusInternalServerError, Code: "TEST_ERROR",
			Reason: "test error occurred"}
		Handler(func(c *Context) (interface{}, error) {
			return nil, &gofrErrors.Response{StatusCode: http.StatusInternalServerError,
				Code:   "TEST_ERROR",
				Reason: "test error occurred",
			}
		}).ServeHTTP(w, r)

		if w.Status != expectedError.StatusCode {
			t.Errorf("Failed StatusCode expected: %v, got: %v", expectedError.StatusCode, w.Status)
		}
		if !strings.Contains(w.Body, expectedError.Reason) || !strings.Contains(w.Body, expectedError.Code) {
			t.Errorf("Error formating failed.")
		}
	}
}

// Test_Head tests if HEAD request for an endpoint returns the correct content length in the response header
func Test_Head(t *testing.T) {
	g := New()
	w := newCustomWriter()
	// making the get request
	r := httptest.NewRequest(http.MethodGet, "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	h := Handler(func(c *Context) (interface{}, error) {
		return "hello", nil
	})

	h.ServeHTTP(w, r)
	// content length of GET response should be equal to HEAD response
	expected := w.Headers.Get("content-length")

	// making the HEAD request for the same endpoint
	r = httptest.NewRequest(http.MethodHead, "/Dummy", http.NoBody)
	r = routeKeySetter(w, r)
	req = request.NewHTTPRequest(r)
	resp = responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	got := w.Header().Get("content-length")
	if got != expected {
		t.Errorf("got %v\n expected %v\n", got, expected)
	}
}

func TestHandler_ServeHTTP_TypeResponse(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r.Header.Add("Content-type", "application/json")
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))
	input := types.Response{
		Data: "Mukund",
	}

	Handler(func(c *Context) (interface{}, error) {
		return input, nil
	}).ServeHTTP(w, r)

	expOutput, _ := json.Marshal(input)

	if !strings.Contains(w.Body, string(expOutput)) {
		t.Errorf("Test failed. expected %v, got %v", string(expOutput), w.Body)
	}
}
func TestHandler_ServeHTTP_TypeRaw(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r.Header.Add("Content-type", "application/json")
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))

	Handler(func(c *Context) (interface{}, error) {
		return types.Raw{
			Data: "Mukund",
		}, nil
	}).ServeHTTP(w, r)

	expOutput := "Mukund"

	if !strings.Contains(w.Body, expOutput) {
		t.Errorf("Test failed. expected %v, got %v", expOutput, w.Body)
	}
}
func TestHandler_ServeHTTP_TypeDefault(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r.Header.Add("Content-type", "application/json")
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))
	expOut := struct {
		Data interface{} `json:"data"`
	}{"Mukund"}

	Handler(func(c *Context) (interface{}, error) {
		return "Mukund", nil
	}).ServeHTTP(w, r)

	expOutput, _ := json.Marshal(expOut)

	if !strings.Contains(w.Body, string(expOutput)) {
		t.Errorf("Test failed. expected %v, got %v", string(expOutput), w.Body)
	}
}

// Test_processErrors_RawError to test process errors for errors.Raw
func Test_processErrors_RawError(t *testing.T) {
	type args struct {
		error          gofrErrors.Raw
		path           string
		method         string
		isPartialError bool
	}

	err := gofrErrors.Raw{StatusCode: http.StatusInternalServerError,
		Err: CustomError{Message: "Test Error"}}

	testCases := []struct {
		desc      string
		arguments args
		expErr    gofrErrors.MultipleErrors
	}{
		{"Success Case - even if method and path are empty then also MultipleErrors should be returned",
			args{err, "", "", false},
			gofrErrors.MultipleErrors{StatusCode: err.StatusCode, Errors: []error{err}}},
		{"Success Case- even if method and path are passed then also MultipleErrors should be returned",
			args{err, "/", http.MethodGet, false},
			gofrErrors.MultipleErrors{StatusCode: err.StatusCode, Errors: []error{err}}},
		{"Success Case- even if isPartialError is true then also MultipleErrors should be returned",
			args{err, "/", http.MethodGet, true},
			gofrErrors.MultipleErrors{StatusCode: err.StatusCode, Errors: []error{err}}},
		{"Success Case- even if one of method and path are empty then also MultipleErrors should be returned",
			args{err, "", http.MethodGet, true},
			gofrErrors.MultipleErrors{StatusCode: err.StatusCode, Errors: []error{err}}},
	}

	for i, tc := range testCases {
		gotErr := processErrors(tc.arguments.error, tc.arguments.path, tc.arguments.method, tc.arguments.isPartialError)

		assert.Equalf(t, tc.expErr, gotErr, "Testcase [%d] Failed: %v", i+1, tc.desc)
	}
}

// TestHandler_ServeHTTP_WithContentType will test the behavior of the ServeHTTP function with different Content-Type headers.
func TestHandler_ServeHTTP_WithContentType(t *testing.T) {
	g := New()
	w := newCustomWriter()
	r := httptest.NewRequest("GET", "/Dummy", http.NoBody)
	r.Header.Add("Content-type", "text/plain")
	r = routeKeySetter(w, r)
	req := request.NewHTTPRequest(r)
	resp := responder.NewContextualResponder(w, r)
	*r = *r.Clone(ctx.WithValue(r.Context(), gofrContextkey, NewContext(resp, req, g)))
	testCases := []struct {
		desc           string
		contentType    string
		expContentType string
	}{
		{"application/json is acceptable", "application/json", "application/json"},
		{"application/xml is acceptable", "application/xml", "application/xml"},
		{"text/plain is acceptable", "text/plain", "text/plain"},
		{"text/html is not acceptable", "text/html", "text/plain"},
		{"application/pdf is not acceptable", "application/pdf", "text/plain"},
		{"text/css is not acceptable", "text/css", "text/plain"},
		{"'empty ContentType' is acceptable", "", "text/plain"},
	}

	for i, tc := range testCases {
		Handler(func(c *Context) (interface{}, error) {
			return types.RawWithOptions{Data: "Hello", ContentType: tc.contentType}, nil
		}).ServeHTTP(w, r)

		resContentType := w.Header().Get("Content-Type")

		assert.Equalf(t, tc.expContentType, resContentType, "Testcase [%d] Failed: %v", i+1, tc.desc)
	}
}

func TestEvaluateTimeAndTimeZone(t *testing.T) {
	expectedFormattedTime := time.Now().UTC().Format(time.RFC3339)
	expectedTimeZone, _ := time.Now().Zone()

	formattedTime, timeZone := evaluateTimeAndTimeZone()

	if formattedTime != expectedFormattedTime {
		t.Errorf("expected formatted time %s but got %s", expectedFormattedTime, formattedTime)
	}

	if timeZone != expectedTimeZone {
		t.Errorf("expected time zone %s but got %s", expectedTimeZone, timeZone)
	}
}

// Test_processErrors to test behavior of processErrors function
func Test_processErrors(t *testing.T) {
	expectedFormattedTime := time.Now().UTC().Format(time.RFC3339)
	expectedTimeZone, _ := time.Now().Zone()

	testCases := []struct {
		desc           string
		err            error // input
		isPartialError bool  // input
		statusCode     int   // it is being used as input and expected response
		ExpErrCode     string
		ExpErrReason   string
	}{
		{"success: errors.Response is passed with StatusCode set",
			&gofrErrors.Response{StatusCode: http.StatusInternalServerError, Code: "Internal Server Error",
				DateTime: gofrErrors.DateTime{Value: expectedFormattedTime, TimeZone: expectedTimeZone}}, false,
			http.StatusInternalServerError, "Internal Server Error", ""},
		{"success : errors.InvalidParam is passed", gofrErrors.InvalidParam{}, false,
			http.StatusBadRequest, "Invalid Parameter", "This request has invalid parameters"},
		{"success : errors.MissingParam is passed", gofrErrors.MissingParam{}, false,
			http.StatusBadRequest, "Missing Parameter", "This request is missing parameters"},
		{"success : errors.EntityNotFound is passed", gofrErrors.EntityNotFound{}, false,
			http.StatusNotFound, "Entity Not Found", "No '' found for Id: ''"},
		{"success : errors.FileNotFound is passed", gofrErrors.FileNotFound{}, false,
			http.StatusNotFound, "File Not Found", "File  not found at location "},
		{"success : errors.MethodMissing is passed", gofrErrors.MethodMissing{}, false,
			http.StatusMethodNotAllowed, "Method not allowed", "Method '' for '' not defined yet"},
		{"success : errors.DB is passed", gofrErrors.DB{}, false,
			http.StatusInternalServerError, "Internal Server Error", "DB Error"},
		{"success : errors.DB is passed", CustomError{Message: "test error"}, false,
			http.StatusInternalServerError, "Internal Server Error", "test error"},
		{"success : errors.DB is passed with partial error true", CustomError{Message: "test error"}, true,
			http.StatusInternalServerError, "Internal Server Error", "test error"},
		{"success: errors.Response is passed with empty fields", &gofrErrors.Response{}, false, 0, "", ""},
		{"success: errors.MultipleErrors is passed", gofrErrors.MultipleErrors{StatusCode: http.StatusBadRequest,
			Errors: []error{gofrErrors.InvalidParam{}}}, false, http.StatusBadRequest, "Invalid Parameter",
			"This request has invalid parameters"},
	}

	for i, tc := range testCases {
		errResp := gofrErrors.Response{StatusCode: tc.statusCode, Code: tc.ExpErrCode, Reason: tc.ExpErrReason,
			DateTime: gofrErrors.DateTime{Value: expectedFormattedTime, TimeZone: expectedTimeZone}}

		expErr := gofrErrors.MultipleErrors{StatusCode: tc.statusCode, Errors: []error{&errResp}}

		err := processErrors(tc.err, http.MethodGet, "/dummy", tc.isPartialError)

		assert.Equalf(t, expErr, err, "Test[%d] Failed: %v", i+1, tc.desc)
		assert.Equalf(t, expErr.Errors, err.Errors, "Test[%d] Failed: %v", i+1, tc.desc)
	}
}
