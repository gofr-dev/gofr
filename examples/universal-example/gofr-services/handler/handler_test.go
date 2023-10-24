package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/responder"
)

type mockService struct{}

func TestHandler_GetLog(t *testing.T) {
	tests := []struct {
		filter   string
		response interface{}
		err      error
	}{
		{"service=gofr-hello-api", "warn", nil},
		{"", nil, errors.MissingParam{Param: []string{"service"}}},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "http://dummy?"+tc.filter, nil)
		c := gofr.NewContext(responder.NewContextualResponder(httptest.NewRecorder(), req), request.NewHTTPRequest(req), nil)

		h := New(mockService{})

		resp, err := h.Log(c)
		assert.Equal(t, tc.err, err, i)
		assert.Equal(t, tc.response, resp, i)
	}
}

func TestHandler_GetHello(t *testing.T) {
	tests := []struct {
		filter   string
		response interface{}
		err      error
	}{
		{"name=ZopSmart", "Hello ZopSmart", nil},
		{"", "Hello", nil},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "http://dummy?"+tc.filter, nil)
		c := gofr.NewContext(responder.NewContextualResponder(httptest.NewRecorder(), req), request.NewHTTPRequest(req), nil)

		h := New(mockService{})

		resp, err := h.Hello(c)
		assert.Equal(t, tc.err, err, i)
		assert.Equal(t, tc.response, resp, i)
	}
}

func (m mockService) Log(_ *gofr.Context, serviceName string) (string, error) {
	if serviceName != "" {
		return "warn", nil
	}

	return "", errors.MissingParam{Param: []string{"service"}}
}

func (m mockService) Hello(_ *gofr.Context, name string) (string, error) {
	if name != "" {
		return "Hello " + name, nil
	}

	return "Hello", nil
}
