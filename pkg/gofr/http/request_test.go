package http

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParam(t *testing.T) {
	req := NewRequest(httptest.NewRequest("GET", "/abc?a=b", nil))
	if req.Param("a") != "b" {
		t.Error("Can not parse the request params")
	}
}

func TestBind(t *testing.T) {
	req := NewRequest(httptest.NewRequest("POST", "/abc", strings.NewReader(`{"a": "b", "b": 5}`)))

	x := struct {
		A string `json:"a"`
		B int    `json:"b"`
	}{}

	req.Bind(&x)

	if x.A != "b" || x.B != 5 {
		t.Errorf("Bind error. Got: %v", x)
	}
}
