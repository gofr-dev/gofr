package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParam(t *testing.T) {
	req := NewRequest(httptest.NewRequest("GET", "/abc?a=b", http.NoBody))
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

	_ = req.Bind(&x)

	if x.A != "b" || x.B != 5 {
		t.Errorf("Bind error. Got: %v", x)
	}
}

func Test_GetContext(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "test/hello", http.NoBody)
	r := Request{req: req, pathParams: map[string]string{"key": "hello"}}

	assert.Equal(t, context.Background(), r.Context())
	assert.Equal(t, "http://", r.HostName())
	assert.Equal(t, "hello", r.PathParam("key"))
}
