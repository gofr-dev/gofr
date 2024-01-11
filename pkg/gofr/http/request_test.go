package http

import (
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

func TestRequest_PathParam(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", http.NoBody)
	req := Request{r, map[string]string{"name": "gofr"}}

	resp := req.PathParam("name")

	assert.Equal(t, "gofr", resp, "TEST Failed.\n")
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

func TestRequest_HostName(t *testing.T) {
	r := NewRequest(httptest.NewRequest("GET", "/test", http.NoBody))

	testCases := []struct {
		protoHeader string
		expected    string
	}{
		{"", "http://example.com"},
		{"https", "https://example.com"},
	}

	for i, tc := range testCases {
		r.req.Header.Set("X-Forwarded-Proto", tc.protoHeader)

		resp := r.HostName()

		assert.Equal(t, tc.expected, resp, "TEST[%d], Failed.\n", i)
	}
}
