package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/examples/using-elasticsearch/model"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"

	"github.com/stretchr/testify/assert"
)

type mockStore struct{}

func (m mockStore) Get(_ *gofr.Context, name string) ([]model.Customer, error) {
	if name == "error" {
		return nil, errors.Error("elastic search error")
	} else if name == "multiple" {
		return []model.Customer{{ID: "12", Name: "ee", City: "city"}, {ID: "189"}}, nil
	}

	return nil, nil
}

func (m mockStore) GetByID(_ *gofr.Context, id string) (model.Customer, error) {
	if id == "o978" {
		return model.Customer{}, errors.Error("error")
	}

	return model.Customer{ID: "ipo897", Name: "Marc"}, nil
}

func (m mockStore) Update(_ *gofr.Context, _ model.Customer, id string) (model.Customer, error) {
	if id == "ofjru3343" {
		return model.Customer{}, errors.Error("error")
	}

	return model.Customer{ID: "ipo897", Name: "Henry"}, nil
}

func (m mockStore) Create(_ *gofr.Context, customer model.Customer) (model.Customer, error) {
	if customer.Name == "March" {
		return model.Customer{}, errors.Error("cannot insert")
	}

	return model.Customer{ID: "weop24444", Name: "Mike"}, nil
}

func (m mockStore) Delete(_ *gofr.Context, id string) error {
	if id == "ef444" {
		return errors.Error("error while deleting")
	}

	return nil
}

func TestCustomer_Index(t *testing.T) {
	tests := []struct {
		desc string
		name string
		resp interface{}
		err  error
	}{
		{"Index error case", "error", nil, &errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}},
		{"Index success case", "multiple", []model.Customer{{ID: "12", Name: "ee", City: "city"}, {ID: "189"}}, nil},
	}

	ms := mockStore{}
	h := New(ms)

	for i, tc := range tests {
		app := gofr.New()

		req := httptest.NewRequest(http.MethodGet, "/customer?name="+tc.name, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		resp, err := h.Index(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Read(t *testing.T) {
	tests := []struct {
		desc string
		id   string
		resp interface{}
		err  error
	}{
		{"read with missing id", "", nil, errors.MissingParam{Param: []string{"id"}}},
		{"read fail", "o978", nil, &errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}},
		{"read success", "ity6", model.Customer{ID: "ipo897", Name: "Marc"}, nil},
	}

	ms := mockStore{}
	h := New(ms)

	for i, tc := range tests {
		app := gofr.New()

		req := httptest.NewRequest("GET", "/customer", nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		resp, err := h.Read(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Create(t *testing.T) {
	tests := []struct {
		desc     string
		customer model.Customer
		err      error
		resp     interface{}
	}{
		{"create fail", model.Customer{Name: "March"},
			&errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}, nil},
		{"create success", model.Customer{Name: "Henry", City: "Marc City"}, nil, model.Customer{ID: "weop24444", Name: "Mike"}},
	}

	ms := mockStore{}
	h := New(ms)

	for i, tc := range tests {
		app := gofr.New()
		body, _ := json.Marshal(tc.customer)
		req := httptest.NewRequest("GET", "/customer", bytes.NewBuffer(body))

		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)
		resp, err := h.Create(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Update(t *testing.T) {
	tests := []struct {
		desc     string
		id       string
		customer interface{}
		err      error
		resp     interface{}
	}{
		{"udpate with missing id", "", nil, errors.MissingParam{Param: []string{"id"}}, nil},
		{"udpate fail", "ofjru3343", nil,
			&errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}, nil},
		{"update success", "ity6", &model.Customer{ID: "ipo897", Name: "Henry"}, nil, model.Customer{ID: "ipo897", Name: "Henry"}},
	}

	ms := mockStore{}
	h := New(ms)

	for i, tc := range tests {
		app := gofr.New()

		body, _ := json.Marshal(tc.customer)

		req := httptest.NewRequest(http.MethodGet, "/customer", bytes.NewBuffer(body))
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		resp, err := h.Update(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Delete(t *testing.T) {
	tests := []struct {
		desc string
		id   string
		resp interface{}
		err  error
	}{
		{"delete with missing id", "", nil, errors.MissingParam{Param: []string{"id"}}},
		{"delete success", "12", "Deleted successfully", nil},
		{"delete fail", "ef444", nil, &errors.Response{StatusCode: 500, Reason: "something unexpected happened"}},
	}

	ms := mockStore{}
	h := New(ms)

	for i, tc := range tests {
		app := gofr.New()

		req := httptest.NewRequest(http.MethodDelete, "/customer?"+tc.id, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		resp, err := h.Delete(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestInvalidBody(t *testing.T) {
	app := gofr.New()

	ms := mockStore{}
	h := New(ms)

	req := httptest.NewRequest(http.MethodPost, "/customer", nil)
	r := request.NewHTTPRequest(req)
	ctx := gofr.NewContext(nil, r, app)

	expErr := errors.InvalidParam{Param: []string{"body"}}

	_, err := h.Create(ctx)

	assert.Equal(t, expErr, err)
}

func TestInvalidBodyUpdate(t *testing.T) {
	app := gofr.New()

	ms := mockStore{}
	h := New(ms)

	req := httptest.NewRequest(http.MethodPost, "/customer", nil)
	r := request.NewHTTPRequest(req)
	ctx := gofr.NewContext(nil, r, app)

	ctx.SetPathParams(map[string]string{
		"id": "1",
	})

	expErr := errors.InvalidParam{Param: []string{"body"}}

	_, err := h.Update(ctx)

	assert.Equal(t, expErr, err)
}
