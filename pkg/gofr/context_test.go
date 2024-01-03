package gofr

import (
	"bytes"
	ctx "context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"

	"golang.org/x/net/context"

	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/middleware"
	"gofr.dev/pkg/middleware/oauth"
)

func TestContext_Param(t *testing.T) {
	testCases := []struct {
		target   string
		key      string
		expected string
	}{
		{"/test", "a", ""},
		{"/test?a=b", "a", "b"},
		{"/test?name=vikash&id=zop4599", "name", "vikash"},
		{"/test?name=vikash&id=zop4599", "id", "zop4599"},
	}

	for _, tc := range testCases {
		r := httptest.NewRequest(http.MethodGet, tc.target, http.NoBody)
		req := request.NewHTTPRequest(r)

		c := NewContext(nil, req, nil)
		if value := c.Param(tc.key); value != tc.expected {
			t.Errorf("Incorrect value for param `%s` from Context. Expected: %s\tGot %s", tc.key, tc.expected, value)
		}
	}
}

func TestContext_Params(t *testing.T) {
	testCases := []struct {
		query    string
		expected map[string]string
	}{
		{"key1=value1&key2=value2", map[string]string{"key1": "value1", "key2": "value2"}},
		{"key=value1&key=value2", map[string]string{"key": "value1,value2"}},
	}

	for _, tc := range testCases {
		r := httptest.NewRequest(http.MethodGet, "http://dummy?"+tc.query, http.NoBody)
		req := request.NewHTTPRequest(r)

		c := NewContext(nil, req, nil)
		params := c.Params()

		assert.Equal(t, tc.expected, params, "TEST[%d], Failed.\n")
	}
}

func TestContext_Header(t *testing.T) {
	testCases := []struct {
		key   string
		value string
	}{
		{"Content-Type", "application/json"},
	}

	for _, tc := range testCases {
		r := httptest.NewRequest(http.MethodGet, "http://dummy", http.NoBody)
		r.Header.Set(tc.key, tc.value)
		req := request.NewHTTPRequest(r)

		c := NewContext(nil, req, nil)
		if value := c.Header(tc.key); value != tc.value {
			t.Errorf("FAILED, Expected: %v, Got: %v", tc.value, value)
		}
	}
}

func TestContext_Body_Response(t *testing.T) {
	reqBody := `{"id":"1","name":"Bob"}`
	r, _ := http.NewRequest(http.MethodGet, "http://dummy", bytes.NewBuffer([]byte(reqBody)))
	req := request.NewHTTPRequest(r)

	c := NewContext(nil, req, nil)

	resp := make(map[string]interface{})
	err := c.Bind(resp)

	if err != nil {
		t.Errorf("FAILED, expected: %v, got: %v", nil, err)
	}
}

type customWriter struct {
	Body    string
	Headers http.Header
	Status  int
}

func newCustomWriter() *customWriter {
	return &customWriter{
		Body:    "",
		Headers: make(http.Header),
		Status:  0,
	}
}

func (c *customWriter) Write(b []byte) (int, error) {
	c.Body += string(b)
	return len(b), nil
}

func (c *customWriter) WriteHeader(statusCode int) {
	c.Status = statusCode
}

func (c *customWriter) Header() http.Header {
	return c.Headers
}

func TestContext_Request(t *testing.T) {
	expected := httptest.NewRequest(http.MethodGet, "http://dummy", http.NoBody)
	req := request.NewHTTPRequest(expected)

	c := NewContext(nil, req, nil)
	if c.Request() != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, c.Request())
	}
}

func Test_SetPathParams(t *testing.T) {
	key := "id"
	expectedValue := "12345"

	r := httptest.NewRequest(http.MethodGet, "http://dummy", http.NoBody)

	req := request.NewHTTPRequest(r)

	c := NewContext(nil, req, nil)

	c.SetPathParams(map[string]string{
		key: expectedValue,
	})

	if got := c.PathParam(key); got != expectedValue {
		t.Errorf("FAILED, Expected: %v, Got: %v", expectedValue, got)
	}
}

func getAppData(c context.Context) map[string]interface{} {
	appData := make(map[string]interface{})

	if data, ok := c.Value(middleware.LogDataKey("appLogData")).(*sync.Map); ok {
		data.Range(func(key, value interface{}) bool {
			if k, ok := key.(string); ok {
				appData[k] = value
			}

			return true
		})
	}

	return appData
}

func Test_Log(t *testing.T) {
	var (
		r     = httptest.NewRequest(http.MethodGet, "http://dummy", http.NoBody)
		req   = request.NewHTTPRequest(r)
		c     = NewContext(nil, req, New())
		key   = "testKey"
		value = "testValue"
	)

	tests := []struct {
		appData *sync.Map
		key     string
		value   interface{}
		data    map[string]interface{}
	}{
		{nil, key, value, map[string]interface{}{}},
		{&sync.Map{}, key, value, map[string]interface{}{key: value}},
		{&sync.Map{}, key, nil, map[string]interface{}{key: nil}},
		{&sync.Map{}, "", value, map[string]interface{}{"": value}},
		{&sync.Map{}, "", nil, map[string]interface{}{"": nil}},
	}

	for i, tc := range tests {
		if tc.appData != nil {
			// simulating the case where the `contextInjector` middleware sets the `appLogData` to empty map
			*r = *r.Clone(ctx.WithValue(c, appData, &sync.Map{}))
		}

		c.Log(tc.key, tc.value)
		// fetch the appData from request context and generate a map of type map[string]interface{}, if appData is nil
		// then getAppData will return empty map
		data := getAppData(c.req.Request().Context())

		assert.Equal(t, tc.data, data, "TEST[%d], Failed.\n", i)
	}
}

func TestLog_CorrelationModify(t *testing.T) {
	var (
		r     = httptest.NewRequest("GET", "http://dummy", http.NoBody)
		req   = request.NewHTTPRequest(r)
		c     = NewContext(nil, req, nil)
		key   = "correlationID"
		value = "7"
	)

	*r = *r.Clone(ctx.WithValue(c, appData, map[string]interface{}{key: value}))

	// Trying to change the correlationID
	c.Log(key, "incorrect")

	// correlationID has to be remain unchanged
	expected := map[string]interface{}{
		key: value,
	}

	got, _ := c.req.Request().Context().Value(appData).(map[string]interface{})

	assert.Equal(t, expected, got, "TEST Failed.\n")
}

func TestContext_ValidateClaimSubPFCX(t *testing.T) {
	r := httptest.NewRequest("GET", "http://dummy", http.NoBody)

	claims := jwt.MapClaims{}
	claims["sub"] = "trial-sub" //nolint
	claims["pfcx"] = "trial-pfcx"
	r = r.Clone(ctx.WithValue(r.Context(), oauth.JWTContextKey("claims"), claims))
	req := request.NewHTTPRequest(r)
	c := NewContext(nil, req, nil)
	c.Context = req.Request().Context()

	// case to check valid sub
	if !c.ValidateClaimSub("trial-sub") {
		t.Error("Got invalid subject. Expected valid subject")
	}

	// case to check invalid sub
	if c.ValidateClaimSub("trial-sub-invalid") {
		t.Error("Got invalid subject. Expected valid subject")
	}

	// case to check valid pfcx
	if !c.ValidateClaimsPFCX("trial-pfcx") {
		t.Error("Got invalid pfcx. Expected valid pfcx")
	}

	// case to check invalid pfcx
	if c.ValidateClaimsPFCX("trial-pfcx-invalid") {
		t.Error("Got invalid pfcx. Expected valid pfcx")
	}
}

func TestContext_ValidateClaimSubScope(t *testing.T) {
	r := httptest.NewRequest("GET", "http://dummy", http.NoBody)

	claims := jwt.MapClaims{}
	claims["scope"] = "trial-scope1 trial-scope2"
	r = r.Clone(ctx.WithValue(r.Context(), oauth.JWTContextKey("claims"), claims))
	req := request.NewHTTPRequest(r)
	c := NewContext(nil, req, nil)
	c.Context = req.Request().Context()

	// case to check valid sub
	if !c.ValidateClaimsScope("trial-scope1") {
		t.Error("Got invalid scope. Expected valid subject")
	}

	// case to check invalid sub
	if c.ValidateClaimsScope("trial-scope-invalid") {
		t.Error("Got invalid scope. Expected valid scope")
	}

	// case to check valid pfcx
	if !c.ValidateClaimsScope("trial-scope2") {
		t.Error("Got invalid scope. Expected valid scope")
	}
}

type company struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

func TestContext_BindStrict(t *testing.T) {
	reqBody := `{"name":"gofr.dev","location":"Bangalore"}`
	r, _ := http.NewRequest("GET", "http://dummy", bytes.NewBuffer([]byte(reqBody)))
	req := request.NewHTTPRequest(r)

	var com company

	c := NewContext(nil, req, nil)
	if err := c.BindStrict(&com); err != nil {
		t.Errorf("FAILED, expected: %v, got: %v", nil, err)
	}
}

func Test_GetClaim(t *testing.T) {
	r := httptest.NewRequest("GET", "http://dummy", http.NoBody)

	r = r.Clone(ctx.WithValue(r.Context(), oauth.JWTContextKey("claims"), jwt.MapClaims(map[string]interface{}{"sub": "trial-sub"})))
	c := NewContext(nil, request.NewHTTPRequest(r), nil)

	testcases := []struct {
		claimKey  string
		expOutput interface{}
	}{
		{"sub", "trial-sub"},
		{"abc", nil},
	}
	for i, tc := range testcases {
		out := c.GetClaim(tc.claimKey)
		assert.Equal(t, tc.expOutput, out, "Test case failed", i)
	}
}

func Test_GetClaims(t *testing.T) {
	expOut := map[string]interface{}{
		"sub": "trial-sub",
		"aud": "test",
	}

	testcases := []struct {
		ctxKey    interface{}
		ctxValue  interface{}
		expOutput interface{}
	}{
		{oauth.JWTContextKey("claims"), jwt.MapClaims(map[string]interface{}{"sub": "trial-sub", "aud": "test"}), expOut},
		{"api-key", "123", map[string]interface{}(nil)},
	}
	for i, tc := range testcases {
		req := httptest.NewRequest("GET", "http://dummy", http.NoBody)
		req = req.Clone(ctx.WithValue(req.Context(), tc.ctxKey, tc.ctxValue))
		gofrCtx := NewContext(nil, request.NewHTTPRequest(req), nil)

		out := gofrCtx.GetClaims()
		assert.Equal(t, tc.expOutput, out, "Test case failed", i)
	}
}
