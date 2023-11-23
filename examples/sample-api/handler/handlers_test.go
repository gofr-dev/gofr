package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/types"
)

func initTest(method, path string, body []byte) *gofr.Context {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	req := request.NewHTTPRequest(r)
	ctx := gofr.NewContext(nil, req, gofr.New())

	return ctx
}
func TestHelloWorldHandler(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, nil)

	resp, err := HelloWorld(ctx)
	if err != nil {
		t.Errorf("FAILED, Expected: %v, Got: %v", nil, err)
	}

	expected := "Hello World!"
	got := fmt.Sprintf("%v", resp)

	if got != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, got)
	}
}

func TestHelloNameHandler(t *testing.T) {
	tests := []struct {
		desc string
		name string
		resp string
	}{
		{"hello without lastname", "SomeName", "Hello SomeName"},
		{"hello with lastname", "Firstname Lastname", "Hello Firstname Lastname"},
	}

	for i, tc := range tests {
		ctx := initTest(http.MethodGet, "http://dummy/hello?name="+url.QueryEscape(tc.name), nil)

		resp, err := HelloName(ctx)

		assert.Equal(t, nil, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestJSONHandler(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, nil)

	res, err := JSONHandler(ctx)
	if err != nil {
		t.Errorf("FAILED, got error: %v", err)
	}

	expected := resp{Name: "Vikash", Company: "gofr.dev"}

	var got resp

	resBytes, _ := json.Marshal(res)

	if err := json.Unmarshal(resBytes, &got); err != nil {
		t.Errorf("FAILED, got error: %v", err)
	}

	assert.Equal(t, expected, got)
}

func TestUserHandler(t *testing.T) {
	tests := []struct {
		desc string
		name string
		resp interface{}
		err  error
	}{
		{"UserHandler success", "Vikash", resp{Name: "Vikash", Company: "gofr.dev"}, nil},
		{"UserHandler fail", "ABC", nil, errors.EntityNotFound{Entity: "user", ID: "ABC"}},
	}

	for i, tc := range tests {
		ctx := initTest(http.MethodGet, "http://dummy", nil)

		ctx.SetPathParams(map[string]string{"name": tc.name})

		resp, err := UserHandler(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestErrorHandler(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, nil)

	res, err := ErrorHandler(ctx)
	if res != nil {
		t.Errorf("FAILED, expected nil, got: %v", res)
	}

	exp := &errors.Response{
		StatusCode: 500,
		Code:       "UNKNOWN_ERROR",
		Reason:     "unknown error occurred",
	}

	assert.Equal(t, exp, err)
}

func TestHelloLogHandler(t *testing.T) {
	ctx := initTest(http.MethodGet, "http://dummy/log", nil)

	res, err := HelloLogHandler(ctx)
	if res != "Logging OK" {
		t.Errorf("Logging Failed due to : %v", err)
	}
}

func TestRawHandler(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, nil)

	resp, err := Raw(ctx)
	if err != nil {
		t.Errorf("FAILED, Expected: %v, Got: %v", nil, err)
	}

	expOut := types.Raw{Data: details{"Mukund"}}

	if resp != expOut {
		t.Errorf("FAILED, Expected: %v, Got: %v", expOut, resp)
	}
}
