package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/service"
)

type mockService struct{ id int }

func TestGetLog(t *testing.T) {
	tests := []struct {
		testID      int
		service     string
		response    string
		expectedErr error
	}{
		{0, "gofr-hello-api", "warn", nil},
		{0, "", "", errors.MissingParam{Param: []string{"service"}}},
		{1, "health-Check", "", &errors.Response{Reason: "unmarshal error"}},
	}

	for i, tc := range tests {
		ctx := gofr.Context{}
		resp, err := New(mockService{id: tc.testID}).Log(&ctx, tc.service)
		assert.Equal(t, tc.expectedErr, err, i)
		assert.Equal(t, tc.response, resp, i)
	}
}

func TestGetHello(t *testing.T) {
	tests := []struct {
		testID      int
		name        string
		response    string
		expectedErr error
	}{
		{0, "gofr.dev", "Hello gofr.dev", nil},
		{0, "", "Hello", nil},
		{1, "", "", &errors.Response{Reason: "unmarshal error"}},
		{2, "", "", service.ErrServiceDown{URL: "http//HelloDown"}},
	}

	for i, tc := range tests {
		ctx := gofr.Context{}
		resp, err := New(mockService{id: tc.testID}).Hello(&ctx, tc.name)
		assert.Equal(t, tc.expectedErr, err, i)
		assert.Equal(t, tc.response, resp, i)
	}
}

//nolint:gocognit // breaking down the function will reduce the readability
func (m mockService) Get(_ context.Context, api string, params map[string]interface{}) (*service.Response, error) {
	if api == "level" && params["service"] != "" {
		return &service.Response{Body: []byte(`{"data": "warn"}`), StatusCode: http.StatusOK}, nil
	}

	if api == "level" {
		return nil, errors.MissingParam{Param: []string{"service"}}
	}

	if api == "hello" && params["name"] != "" {
		return &service.Response{Body: []byte(`{"data": "Hello gofr.dev"}`), StatusCode: http.StatusOK}, nil
	}

	if api == "hello" && m.id == 0 || m.id == 1 {
		return &service.Response{Body: []byte(`{"data": "Hello"}`), StatusCode: http.StatusOK}, nil
	}

	return nil, service.ErrServiceDown{URL: "http//HelloDown"}
}

func (m mockService) Bind(resp []byte, i interface{}) error {
	if m.id == 0 {
		return json.Unmarshal(resp, &i)
	}

	return &errors.Response{Reason: "unmarshal error"}
}
