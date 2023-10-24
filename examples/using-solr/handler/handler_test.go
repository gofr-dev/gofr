package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/examples/using-solr/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"

	"github.com/stretchr/testify/assert"
)

const er = "error"

func TestCustomer_List(t *testing.T) {
	testcases := []struct {
		desc  string
		query string
		err   error
	}{
		{"get by id and name", "id=1&name=Henry", nil},
		{"get non existent customer", "id=123&name=Tomato", errors.Error("core error")},
		{"empty query string", "", errors.MissingParam{Param: []string{"id"}}},
	}
	c := New(&mockStore{})
	app := gofr.New()

	for i, tc := range testcases {
		req := httptest.NewRequest(http.MethodGet, "/dummy?"+tc.query, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)
		_, err := c.List(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Create(t *testing.T) {
	//nolint:govet // table tests
	tests := []struct {
		desc string
		body []byte
		err  error
	}{
		{"create success", []byte(`{"id":1,"name":"Ethen"}`), nil},
		{"create failure", []byte(`{"id":1,"name":"error"}`), errors.Error("core error")},
		{"create invalid param", []byte(`{"id":1}`), errors.InvalidParam{[]string{"name"}}},
		{"create invalid body", []byte(`{"id":"1"}`), errors.InvalidParam{[]string{"body"}}},
	}

	c := New(&mockStore{})
	app := gofr.New()

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodPost, "/dummy", bytes.NewBuffer(tc.body))
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)
		_, err := c.Create(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Update(t *testing.T) {
	//nolint:govet // table tests
	tests := []struct {
		desc string
		body []byte
		err  error
	}{
		{"update success", []byte(`{"id":1,"name":"Ethen"}`), nil},
		{"update fail", []byte(`{"id":1,"name":"error"}`), errors.Error("core error")},
		{"update with invalid name", []byte(`{"id":1}`), errors.InvalidParam{Param: []string{"name"}}},
		{"update with invalid body", []byte(`{"id":"1"}`), errors.InvalidParam{[]string{"body"}}},
		{"update with invalid id", []byte(`{"name":"Wen"}`), errors.InvalidParam{[]string{"id"}}},
	}

	c := New(&mockStore{})
	app := gofr.New()

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodPut, "/dummy", bytes.NewBuffer(tc.body))
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)
		_, err := c.Update(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Delete(t *testing.T) {
	tests := []struct {
		desc string
		body []byte
		err  error
	}{
		{"delete success", []byte(`{"id":1,"name":"Ethen"}`), nil},
		{"delete fail", []byte(`{"id":1,"name":"error"}`), errors.Error("core error")},
		{"delete with invalid body", []byte(`{"id":"1"}`), errors.InvalidParam{Param: []string{"body"}}},
		{"delete with invalid id", []byte(`{"name":"Wen"}`), errors.InvalidParam{Param: []string{"id"}}},
	}

	c := New(&mockStore{})
	app := gofr.New()

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodDelete, "/dummy", bytes.NewBuffer(tc.body))
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)
		_, err := c.Delete(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Errors(t *testing.T) {
	c := New(&mockStore{})
	app := gofr.New()
	req := httptest.NewRequest(http.MethodPost, "/dummy", errReader(0))
	r := request.NewHTTPRequest(req)
	ctx := gofr.NewContext(nil, r, app)

	_, err := c.Delete(ctx)
	if err == nil {
		t.Errorf("Expected error but got nil")
	}

	_, err = c.Create(ctx)
	if err == nil {
		t.Errorf("Expected error but got nil")
	}

	_, err = c.Update(ctx)
	if err == nil {
		t.Errorf("Expected error but got nil")
	}
}

type errReader int

func (errReader) Read([]byte) (n int, err error) {
	return 0, errors.Error("test error")
}

type mockStore struct{}

func (m *mockStore) List(_ *gofr.Context, _ string, filter store.Filter) ([]store.Model, error) {
	if filter.ID == "1" {
		return []store.Model{{ID: 1, Name: "Henry", DateOfBirth: "01-01-1987"}}, nil
	}

	return nil, errors.Error("core error")
}

func (m *mockStore) Create(_ *gofr.Context, _ string, customer store.Model) error {
	if customer.Name == er {
		return errors.Error("core error")
	}

	return nil
}

func (m *mockStore) Update(_ *gofr.Context, _ string, customer store.Model) error {
	if customer.Name == "error" {
		return errors.Error("core error")
	}

	return nil
}

func (m *mockStore) Delete(_ *gofr.Context, _ string, customer store.Model) error {
	if customer.Name == "error" {
		return errors.Error("core error")
	}

	return nil
}
