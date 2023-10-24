package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/universal-example/pgsql/entity"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/responder"
)

type mockStore struct{}

const (
	fetchErr  = constError("error while fetching employee listing")
	createErr = constError("error while adding new employee")
)

type constError string

func (err constError) Error() string {
	return string(err)
}

func (m mockStore) Get(c *gofr.Context) ([]entity.Employee, error) {
	p := c.Param("mock")
	if p == "success" {
		return nil, nil
	}

	return nil, fetchErr
}

func (m mockStore) Create(_ *gofr.Context, customer entity.Employee) error {
	switch customer.Name {
	case "some_employee":
		return nil
	case "mock body error":
		return errors.InvalidParam{Param: []string{"body"}}
	}

	return createErr
}

func TestPgsqlEmployee_Get(t *testing.T) {
	m := New(mockStore{})

	app := gofr.New()

	tests := []struct {
		mockParamStr string
		expectedErr  error
	}{
		{"mock=success", nil},
		{"", fetchErr},
	}

	for i, tc := range tests {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/dummy?"+tc.mockParamStr, nil)
		req := request.NewHTTPRequest(r)
		res := responder.NewContextualResponder(w, r)
		c := gofr.NewContext(res, req, app)

		_, err := m.Get(c)
		assert.Equal(t, tc.expectedErr, err, i)
	}
}

func TestPgsqlEmployee_Create(t *testing.T) {
	m := New(mockStore{})
	app := gofr.New()

	tests := []struct {
		body        []byte
		expectedErr error
	}{
		{[]byte(`{"name":"some_employee"}`), nil},
		{[]byte(`mock body error`), errors.InvalidParam{Param: []string{"body"}}},
		{[]byte(`{"name":"creation error"}`), createErr},
	}

	for i, tc := range tests {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "http://dummy", bytes.NewReader(tc.body))
		req := request.NewHTTPRequest(r)
		res := responder.NewContextualResponder(w, r)
		c := gofr.NewContext(res, req, app)

		_, err := m.Create(c)
		assert.Equal(t, tc.expectedErr, err, i)
	}
}
