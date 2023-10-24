package request

import (
	"bytes"
	ctx "context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/middleware/oauth"
)

func TestNewHTTPRequest(*testing.T) {
	NewHTTPRequest(httptest.NewRequest("GET", "http://dummy", nil))
}

func TestHTTP_String(t *testing.T) {
	var (
		h      HTTP
		method = "GET"
		u      = "http://dummy"
		req    = httptest.NewRequest(method, u, nil)
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

	h.req = httptest.NewRequest(expected, "http://dummy", nil)
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

	h.req = httptest.NewRequest("GET", "http://dummy"+expected, nil)
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

		h.req = httptest.NewRequest("GET", "http://dummy"+tc.queryParams, nil)

		got := h.Param(tc.k)

		if !reflect.DeepEqual(tc.expected, got) {
			t.Errorf("FAILED, %v expected: %v, got: %v", i, tc.expected, got)
		}
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

	for _, tc := range tcs {
		var h HTTP

		h.req = httptest.NewRequest("GET", "http://dummy"+tc.queryParams, nil)

		got := h.ParamNames()

		sort.Slice(got, func(i, j int) bool {
			return len(got[i]) < len(got[j])
		})

		if !reflect.DeepEqual(tc.expected, got) {
			t.Errorf("FAILED, expected: %v, got: %v", tc.expected, got)
		}
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

	for _, tc := range tcs {
		var h HTTP

		h.req = httptest.NewRequest("GET", "http://dummy"+tc.queryParams, nil)

		got := h.Params()

		if !reflect.DeepEqual(tc.expected, got) {
			t.Errorf("FAILED, expected: %v, got: %v", tc.expected, got)
		}
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

	for _, tc := range tcs {
		var h HTTP

		h.req = httptest.NewRequest("GET", "http://dummy", nil)
		h.pathParams = tc.pathParams

		if tc.pathParams == nil {
			h.req = mux.SetURLVars(h.req, map[string]string{
				tc.key: tc.expectedValue,
			})
		}

		got := h.PathParam(tc.key)

		if !reflect.DeepEqual(tc.expectedValue, got) {
			t.Errorf("FAILED, expected: %v, got: %v", tc.expectedValue, got)
		}
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

	for _, tc := range tcs {
		var (
			h   HTTP
			req = httptest.NewRequest(http.MethodPost, "http://dummy", tc.reqBody)
		)

		h.req = req

		response, err := h.Body()

		if reflect.DeepEqual(tc.reqBody, malformedReader{}) && err == nil {
			t.Errorf("FAILED, expected: %v, got: %v", "read error", err)
		}

		if err == nil && string(response) != rb {
			t.Errorf("FAILED, expected: %v, got: %v", rb, string(response))
		}
	}
}

func TestHTTP_Header(t *testing.T) {
	var (
		h        HTTP
		key      = "key123"
		expected = "value123"
		req      = httptest.NewRequest("GET", "http://dummy", nil)
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
	}{
		{
			bytes.NewBuffer([]byte(jsonData)),
			resp{},
			nil,
			false,
		},
		{
			bytes.NewBuffer([]byte(xmlData)),
			resp{},
			nil,
			true,
		},
		{
			malformedReader{},
			nil,
			fmt.Errorf("something unexpected occurred"),
			false,
		},
	}

	for _, tc := range tcs {
		var (
			h   HTTP
			req = httptest.NewRequest(http.MethodPost, "http://dummy", tc.reqBody)
		)

		if tc.isXML {
			req.Header.Set("Content-Type", "text/xml")
		}

		h.req = req

		err := h.Bind(tc.i)

		if err != nil && !reflect.DeepEqual(tc.err, err) {
			t.Errorf("FAILED, expected: %v, got: %v", tc.err, err)
		}
	}
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
			req = httptest.NewRequest(http.MethodPost, "http://dummy", tc.reqBody)
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
		expected = httptest.NewRequest("GET", "http://dummy", nil)
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
		r := httptest.NewRequest("GET", "http://dummy", nil)
		r = r.Clone(ctx.WithValue(r.Context(), tc.ctxKey, tc.ctxVal))
		req := NewHTTPRequest(r)

		out := req.GetClaims()
		assert.Equal(t, tc.expOutput, out, "Test case Failed", i)
	}
}

func TestHTTP_GetClaim(t *testing.T) {
	r := httptest.NewRequest("GET", "http://dummy", nil)
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
