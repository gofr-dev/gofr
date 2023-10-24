package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/types"

	"github.com/stretchr/testify/assert"
)

func TestHelloWorldHandler(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, nil)

	resp, err := HelloWorld(ctx)
	if err != nil {
		t.Errorf("FAILED, got error: %v", err)
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
		resp types.Response
	}{
		{"short name", "SomeName", types.Response{Data: "Hello SomeName", Meta: map[string]interface{}{"page": 1, "offset": 0}}},
		{"full name", "Firstname Lastname", types.Response{Data: "Hello Firstname Lastname",
			Meta: map[string]interface{}{"page": 1, "offset": 0}}},
	}

	for i, tc := range tests {
		r := httptest.NewRequest(http.MethodGet, "http://dummy/hello?name="+url.QueryEscape(tc.name), nil)
		req := request.NewHTTPRequest(r)
		ctx := gofr.NewContext(nil, req, nil)

		resp, err := HelloName(ctx)
		if err != nil {
			t.Errorf("FAILED, got error: %v", err)
		}

		result, _ := resp.(types.Response)

		assert.Equal(t, tc.resp, result, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestPostNameHandler(t *testing.T) {
	var jsonStr = []byte(`{"Username":"username"}`)
	r := httptest.NewRequest(http.MethodPost, "/post", bytes.NewBuffer(jsonStr))
	req := request.NewHTTPRequest(r)

	r.Header.Set("Content-Type", "application/json")

	app := gofr.New()
	ctx := gofr.NewContext(nil, req, app)

	resp, err := PostName(ctx)
	if err != nil {
		t.Errorf("FAILED, got error: %v", err)
	}

	var got person

	respBytes, _ := json.Marshal(resp)
	_ = json.Unmarshal(respBytes, &got)

	expected := person{Username: "username"}

	assert.Equal(t, expected, got)
}

func TestPostNameHandlerfail(t *testing.T) {
	// invalid JSON passed
	var jsonStr = []byte(`{"Username":}`)

	r := httptest.NewRequest(http.MethodPost, "/post", bytes.NewBuffer(jsonStr))
	r.Header.Set("Content-Type", "application/json")
	req := request.NewHTTPRequest(r)
	ctx := gofr.NewContext(nil, req, gofr.New())

	resp, err := PostName(ctx)
	if err == nil {
		t.Errorf("FAILED, got error: %v", err)
	}

	if resp != nil {
		t.Errorf("FAILED, Expected: nil, Got: %v", resp)
	}
}

func TestMultipleErrorHandler(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://dummy/multiple-errors", nil)
	req := request.NewHTTPRequest(r)
	ctx := gofr.NewContext(nil, req, nil)

	_, err := MultipleErrorHandler(ctx)
	if err == nil {
		t.Errorf("FAILED, got nil expectedErr")
		return
	}

	expectedErr := `Incorrect value for parameter: EmailAddress
Parameter Address is required for this request`

	got := err.Error()
	if got != expectedErr {
		t.Errorf("FAILED, Expected: %v, Got: %v", expectedErr, got)
	}
}
