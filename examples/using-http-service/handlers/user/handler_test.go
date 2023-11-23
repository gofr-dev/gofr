package user

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-http-service/models"
	"gofr.dev/examples/using-http-service/services"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/responder"
)

func Test_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockService := services.NewMockUser(ctrl)

	tests := []struct {
		desc     string
		name     string
		resp     interface{}
		mockResp interface{}
		err      error
	}{
		{"get success", "Vikash", models.User{Name: "Vikash", Company: "gofr.dev"}, models.User{Name: "Vikash", Company: "gofr.dev"}, nil},
		{"get non existent entity", "ABC", nil, models.User{}, errors.EntityNotFound{Entity: "User", ID: "ABC"}},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "http://dummy", nil)
		ctx := gofr.NewContext(responder.NewContextualResponder(httptest.NewRecorder(), req), request.NewHTTPRequest(req), nil)
		mockService.EXPECT().Get(ctx, tc.name).Return(tc.mockResp, tc.err)
		h := New(mockService)

		ctx.SetPathParams(map[string]string{"name": tc.name})
		resp, err := h.Get(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_GetMissingParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockService := services.NewMockUser(ctrl)

	expErr := errors.MissingParam{Param: []string{"name"}}
	req := httptest.NewRequest(http.MethodGet, "http://dummy", nil)

	ctx := gofr.NewContext(responder.NewContextualResponder(httptest.NewRecorder(), req), request.NewHTTPRequest(req), nil)
	h := New(mockService)

	ctx.SetPathParams(map[string]string{"name": ""})
	resp, err := h.Get(ctx)

	assert.Equal(t, expErr, err, "TEST failed.")

	assert.Equal(t, nil, resp, "TEST failed.")
}
