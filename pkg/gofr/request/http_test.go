package request

import (
	"bytes"
	ctx "context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/middleware/oauth"
)

const urlHTTPDummy = "http://dummy"

func TestNewHTTPRequest(*testing.T) {
	NewHTTPRequest(httptest.NewRequest("GET", urlHTTPDummy, http.NoBody))
}

func TestHTTP_String(t *testing.T) {
	var (
		h      HTTP
		method = "GET"
		u      = urlHTTPDummy
		req    = httptest.NewRequest(method, u, http.NoBody)
	)

	expected := fmt.Sprintf("%s %s", method, u)

	h.req = req
	got := h.String()

	if expected != got {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}

func TestHTTP_Method(t *testing.T) {
	var (
		h        HTTP
		expected = "GET"
	)

	h.req = httptest.NewRequest(expected, urlHTTPDummy, http.NoBody)
	got := h.Method()

	if expected != got {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}

func TestHTTP_URI(t *testing.T) {
	var (
		h        HTTP
		expected = "/xyz"
	)

	h.req = httptest.NewRequest("GET", urlHTTPDummy+expected, http.NoBody)
	got := h.URI()

	if expected != got {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}

func TestHTTP_Param(t *testing.T) {
	tcs := []struct {
		queryParams string
		expected    string
		k           string
	}{
		{"?key=value&key1=value1", "value", "key"},

		{"?key=value&key=value1&keys=value2", "value,value1", "key"},

		{"", "", "key"},

		{"?key=value", "value", "key"},

		{"?key=value&key=vla&kez=values", "value,vla", "key"},

		{"?key=value&keys=value1", "value1", "keys"},

		{"?keys=value1&keys=value2", "value1,value2", "keys"},
	}
	for i, tc := range tcs {
		var h HTTP

		h.req = httptest.NewRequest("GET", urlHTTPDummy+tc.queryParams, http.NoBody)

		got := h.Param(tc.k)

		assert.Equal(t, tc.expected, got, "TEST[%d], Failed.\n", i)
	}
}

func TestHTTP_ParamNames(t *testing.T) {
	tcs := []struct {
		queryParams string
		expected    []string
	}{
		{
			"?key=value&key1=value1",
			[]string{"key", "key1"},
		},
		{
			"",
			nil,
		},
	}

	for i, tc := range tcs {
		var h HTTP

		h.req = httptest.NewRequest("GET", urlHTTPDummy+tc.queryParams, http.NoBody)

		got := h.ParamNames()

		sort.Slice(got, func(i, j int) bool {
			return len(got[i]) < len(got[j])
		})

		assert.Equal(t, tc.expected, got, "TEST[%d], Failed.\n", i)
	}
}

func TestHTTP_Params(t *testing.T) {
	tcs := []struct {
		queryParams string
		expected    map[string]string
	}{
		{
			"?key=value&key1=value1",
			map[string]string{
				"key":  "value",
				"key1": "value1",
			},
		},
		{
			"?key=value&key=value1&key=value2",
			map[string]string{
				"key": "value,value1,value2",
			},
		},
		{
			"",
			map[string]string{},
		},
	}

	for i, tc := range tcs {
		var h HTTP

		h.req = httptest.NewRequest("GET", urlHTTPDummy+tc.queryParams, http.NoBody)

		got := h.Params()

		assert.Equal(t, tc.expected, got, "TEST[%d], Failed.\n", i)
	}
}

func TestHTTP_PathParam(t *testing.T) {
	tcs := []struct {
		pathParams    map[string]string
		key           string
		expectedValue string
	}{
		{
			map[string]string{
				"key":        "value",
				"anotherKey": "anotherValue",
			},
			"key",
			"value",
		},
		{
			nil,
			"key",
			"value",
		},
	}

	for i, tc := range tcs {
		var h HTTP

		h.req = httptest.NewRequest("GET", urlHTTPDummy, http.NoBody)
		h.pathParams = tc.pathParams

		if tc.pathParams == nil {
			h.req = mux.SetURLVars(h.req, map[string]string{
				tc.key: tc.expectedValue,
			})
		}

		got := h.PathParam(tc.key)

		assert.Equal(t, tc.expectedValue, got, "TEST[%d], Failed.\n", i)
	}
}

type malformedReader struct{}

func (r malformedReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("something unexpected occurred")
}

func TestHTTP_Body(t *testing.T) {
	rb := `{"id":"1","name":"Bob"}`

	tcs := []struct {
		reqBody io.Reader
	}{
		{
			bytes.NewBuffer([]byte(rb)),
		},
		{
			malformedReader{},
		},
	}

	for i, tc := range tcs {
		var (
			h   HTTP
			req = httptest.NewRequest(http.MethodPost, urlHTTPDummy, tc.reqBody)
		)

		h.req = req

		response, err := h.Body()

		if err == nil {
			assert.NotEqual(t, tc.reqBody, malformedReader{}, "TEST[%d], Failed.\n", i)

			assert.Equal(t, rb, string(response), "TEST[%d], Failed.\n", i)
		}
	}
}

func TestHTTP_Header(t *testing.T) {
	var (
		h        HTTP
		key      = "key123"
		expected = "value123"
		req      = httptest.NewRequest("GET", urlHTTPDummy, http.NoBody)
	)

	req.Header.Set(key, expected)
	h.req = req

	if v := h.Header(key); v != expected {
		t.Errorf("FAILED, expected: %v, got: %v", expected, v)
	}
}

func TestHTTP_Bind(t *testing.T) {
	type resp struct {
		ID   string `json:"id" xml:"id"`
		Name string `json:"name" xml:"name"`
	}

	jsonData := `{"id":"1","name":"Bob"}`
	xmlData := `<root><id>1</id><name>Bob</name></root>`

	tcs := []struct {
		reqBody io.Reader
		i       interface{}
		err     error
		isXML   bool
		isJSON  bool
	}{
		{
			bytes.NewBuffer([]byte(jsonData)),
			resp{},
			nil,
			false,
			true,
		},
		{
			bytes.NewBuffer([]byte(xmlData)),
			resp{},
			nil,
			true,
			false,
		},
		{
			malformedReader{},
			nil,
			fmt.Errorf("something unexpected occurred"),
			false,
			false,
		},
	}

	for i, tc := range tcs {
		var (
			h   HTTP
			req = httptest.NewRequest(http.MethodPost, urlHTTPDummy, tc.reqBody)
		)

		if tc.isXML {
			req.Header.Set("Content-Type", "text/xml")
		}

		h.req = req

		err := h.Bind(tc.i)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n", i)
	}
}

func TestHTTP_BindMultipartFormData(t *testing.T) {
	var h HTTP

	// Create a sample multipart form data request body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add form fields
	_ = writer.WriteField("username", "testuser")
	_ = writer.WriteField("password", "testpass")

	// Close the writer to add the boundary
	writer.Close()

	// Create a new request with the multipart form data
	req := httptest.NewRequest(http.MethodPost, urlHTTPDummy, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	h.req = req

	// Create a struct to decode the form data into
	type FormData struct {
		Username string `form:"username"`
		Password string `form:"password"`
	}

	var formData FormData

	// Call the Bind method
	err := h.Bind(&formData)

	// Assert that there is no error and the form data is correctly decoded
	assert.NoError(t, err)
	assert.Equal(t, "testuser", formData.Username)
	assert.Equal(t, "testpass", formData.Password)

	t.Run("MalformedData", func(t *testing.T) {
		var hMalformed HTTP
		reqMalformed := httptest.NewRequest(http.MethodPost, urlHTTPDummy, strings.NewReader("malformed data"))
		reqMalformed.Header.Set("Content-Type", "multipart/form-data")

		hMalformed.req = reqMalformed
		errMalformed := hMalformed.Bind(&formData)

		assert.Error(t, errMalformed)
		assert.Contains(t, errMalformed.Error(), "no multipart boundary param")
	})
}

func TestHTTP_BindStrict(t *testing.T) {
	type resp struct {
		ID   string
		Name string
	}

	xmlData := `<root><id>1</id><name>Bob</name></root>`
	resp1 := resp{}
	tcs := []struct {
		reqBody     io.Reader
		i           interface{}
		expectedErr bool
		isXML       bool
	}{

		{bytes.NewBuffer([]byte(`{"id":"1","name":"Bob"}`)), &resp{}, false, false},
		{bytes.NewBuffer([]byte(`{"id1":"1","name":"Bob" , "age" : 23 }`)), &resp1, true, false},
		{bytes.NewBuffer([]byte(xmlData)), resp{}, false, true},
		{malformedReader{}, nil, true, false},
	}

	for _, tc := range tcs {
		var (
			h   HTTP
			req = httptest.NewRequest(http.MethodPost, urlHTTPDummy, tc.reqBody)
		)

		if tc.isXML {
			req.Header.Set("Content-Type", "text/xml")
		}

		h.req = req

		err := h.BindStrict(tc.i)
		if (err != nil && !tc.expectedErr) || (err == nil && tc.expectedErr) {
			t.Errorf("FAILED expected: %v,got :%v  ", tc.expectedErr, err)
		}
	}
}

func TestHTTP_Request(t *testing.T) {
	var (
		h        HTTP
		expected = httptest.NewRequest("GET", urlHTTPDummy, http.NoBody)
	)

	h.req = expected
	if h.Request() != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, h.Request())
	}
}

func TestHTTP_GetClaims(t *testing.T) {
	expOut := map[string]interface{}{
		"sub": "trial-sub",
	}
	testcases := []struct {
		ctxKey    interface{}
		ctxVal    interface{}
		expOutput map[string]interface{}
	}{
		{oauth.JWTContextKey("claims"), jwt.MapClaims(map[string]interface{}{"sub": "trial-sub"}), expOut},
		{"x-api-key", "123", map[string]interface{}(nil)},
	}

	for i, tc := range testcases {
		r := httptest.NewRequest("GET", urlHTTPDummy, http.NoBody)
		r = r.Clone(ctx.WithValue(r.Context(), tc.ctxKey, tc.ctxVal))
		req := NewHTTPRequest(r)

		out := req.GetClaims()
		assert.Equal(t, tc.expOutput, out, "Test case Failed", i)
	}
}

func TestHTTP_GetClaim(t *testing.T) {
	r := httptest.NewRequest("GET", urlHTTPDummy, http.NoBody)
	r = r.Clone(ctx.WithValue(r.Context(), oauth.JWTContextKey("claims"),
		jwt.MapClaims(map[string]interface{}{"sub": "trial-sub"})))
	req := NewHTTPRequest(r)

	testcases := []struct {
		key       string
		expOutput interface{}
	}{
		{"sub", "trial-sub"},
		{"abc", nil},
	}
	for i, tc := range testcases {
		out := req.GetClaim(tc.key)
		assert.Equal(t, tc.expOutput, out, "Test case Failed", i)
	}
}
