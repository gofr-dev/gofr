package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-mysql/models"
	"gofr.dev/examples/using-mysql/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func initializeHandlerTest(t *testing.T) (*store.MockStore, handler, *gofr.Gofr) {
	ctrl := gomock.NewController(t)

	mockStore := store.NewMockStore(ctrl)
	h := New(mockStore)
	app := gofr.New()

	return mockStore, h, app
}

func TestGet(t *testing.T) {
	tests := []struct {
		desc string
		resp []models.Employee
		err  error
	}{
		{"success case", []models.Employee{{ID: 0, Name: "sample", Email: "email@gmail.com", Phone: 930098800,
			City: "kolkata"}}, nil},
		{"error case", nil, errors.Error("error fetching employee listing")},
	}

	mockStore, h, app := initializeHandlerTest(t)

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/employee", nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		mockStore.EXPECT().Get(ctx).Return(tc.resp, tc.err)

		result, err := h.Get(ctx)

		if tc.err == nil {
			// Assert successful response
			assert.Nil(t, err)
			assert.NotNil(t, result)

			res, ok := result.(response)
			assert.True(t, ok)
			assert.Equal(t, tc.resp, res.Employees)
		} else {
			// Assert error response
			assert.NotNil(t, err)
			assert.Equal(t, tc.err, err)
			assert.Nil(t, result)
		}
	}
}

func TestCreate(t *testing.T) {
	mockStore, h, app := initializeHandlerTest(t)

	input := `{"id":6,"name":"mahak","email":"msjce","phone":928902,"city":"kolkata"}`
	expResp := models.Employee{ID: 6, Name: "mahak", Email: "msjce", Phone: 928902, City: "kolkata"}

	in := strings.NewReader(input)
	req := httptest.NewRequest(http.MethodPost, "/employee", in)
	r := request.NewHTTPRequest(req)
	ctx := gofr.NewContext(nil, r, app)

	var emp models.Employee

	_ = ctx.Bind(&emp)

	mockStore.EXPECT().Get(ctx).Return(nil, nil).MaxTimes(2)
	mockStore.EXPECT().Create(ctx, emp).Return(expResp, nil).MaxTimes(1)

	resp, err := h.Create(ctx)

	assert.Nil(t, err, "TEST,failed :success case")

	assert.Equal(t, expResp, resp, "TEST, failed:success case")
}

func TestCreate_Error(t *testing.T) {
	mockStore, h, app := initializeHandlerTest(t)

	tests := []struct {
		desc    string
		input   string
		expResp interface{}
		err     error
	}{{"create invalid body", `{"id":6,"name":"mahak","email":"msjce","phone":928902}`, models.Employee{},
		errors.InvalidParam{Param: []string{"body"}}},
		{"create invalid body", `}`, models.Employee{}, errors.InvalidParam{Param: []string{"body"}}},
	}

	for i, tc := range tests {
		in := strings.NewReader(tc.input)
		req := httptest.NewRequest(http.MethodPost, "/employee", in)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		var emp models.Employee

		_ = ctx.Bind(&emp)

		mockStore.EXPECT().Create(ctx, emp).Return(tc.expResp.(models.Employee), tc.err).MaxTimes(1)

		resp, err := h.Create(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Nil(t, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
