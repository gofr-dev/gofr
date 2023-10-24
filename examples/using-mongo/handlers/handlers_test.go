package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-mongo/models"
	"gofr.dev/examples/using-mongo/stores"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func initializeHandlersTest(t *testing.T) (*stores.MockCustomer, handler, *gofr.Gofr) {
	ctrl := gomock.NewController(t)

	store := stores.NewMockCustomer(ctrl)
	h := New(store)
	app := gofr.New()

	return store, h, app
}

func TestHandler_Get(t *testing.T) {
	tests := []struct {
		desc        string
		queryParams string
		name        string
		resp        interface{}
		err         error
	}{
		{"get without params", "", "", []models.Customer{{Name: "Ponting", Age: 24, City: "Sydney"}}, nil},
		{"get with name", "name=Tim", "Tim", []models.Customer{{Name: "Tim", Age: 35, City: "Munich"}}, nil},
		{"get with invalid name", "name=1", "1", nil, errors.InvalidParam{Param: []string{"name"}}},
	}

	store, h, app := initializeHandlersTest(t)

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/customer?"+tc.queryParams, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		store.EXPECT().Get(ctx, tc.name).Return(tc.resp, tc.err)

		resp, err := h.Get(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

type errReader int

func (errReader) Read([]byte) (n int, err error) {
	return 0, errors.Error("test error")
}

func TestHandler_Create_Invalid_Input_Error(t *testing.T) {
	expErr := errors.Error("test error")

	_, h, app := initializeHandlersTest(t)
	req := httptest.NewRequest(http.MethodGet, "/dummy", errReader(0))
	r := request.NewHTTPRequest(req)
	ctx := gofr.NewContext(nil, r, app)

	_, err := h.Create(ctx)

	assert.Equal(t, expErr, err)
}

func TestHandler_Create_Invalid_JSON(t *testing.T) {
	input := `{"name":"Pirlo","age":"42","city":"Turin"}`
	expErr := &json.UnmarshalTypeError{
		Value:  "string",
		Type:   reflect.TypeOf(42),
		Offset: 26,
		Struct: "Customer",
		Field:  "age",
	}

	_, h, app := initializeHandlersTest(t)

	inputReader := strings.NewReader(input)
	req := httptest.NewRequest(http.MethodGet, "/dummy", inputReader)
	r := request.NewHTTPRequest(req)
	ctx := gofr.NewContext(nil, r, app)

	_, err := h.Create(ctx)

	assert.Equal(t, expErr, err)
}

func TestHandler_Create(t *testing.T) {
	customer := `{"name":"Pirlo","age":42,"city":"Turin"}`
	c := models.Customer{Name: "Pirlo", Age: 42, City: "Turin"}
	tests := []struct {
		desc string
		resp string
		err  error
	}{
		{"create success", "New Customer Added!!", nil},
		{"create fail", "", errors.Error("test error")},
	}

	store, h, app := initializeHandlersTest(t)

	for i, tc := range tests {
		input := strings.NewReader(customer)

		req := httptest.NewRequest(http.MethodGet, "/dummy", input)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		store.EXPECT().Create(ctx, c).Return(tc.err)

		_, err := h.Create(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestHandler_Delete(t *testing.T) {
	tests := []struct {
		desc  string
		name  string
		count int
		resp  interface{}
		err   error
	}{
		{"delete invalid entity", "1", 0, nil, errors.InvalidParam{Param: []string{"name"}}},
		{"delete multiple entities", "Tim", 2, "2 Customers Deleted!", nil},
		{"delete single entity", "Thomas", 1, "1 Customers Deleted!", nil},
	}

	store, h, app := initializeHandlersTest(t)

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/customer?name="+tc.name, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		store.EXPECT().Delete(ctx, gomock.Any()).Return(tc.count, tc.err).Times(1)

		resp, err := h.Delete(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
