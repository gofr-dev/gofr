package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/universal-example/pgsql/entity"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

func initializeMock() (sqlmock.Sqlmock, *gofr.Context) {
	app := gofr.New()

	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, nil
	}

	c := gofr.NewContext(nil, nil, app)
	c.DataStore = datastore.DataStore{ORM: db}
	c.Context = context.Background()

	return mock, c
}

func TestEmployee_Get(t *testing.T) {
	mock, c := initializeMock()
	tests := []struct {
		desc     string
		rows     *sqlmock.Rows
		expError error
		resErr   error
		expResp  []entity.Employee
	}{
		{"get success", mock.NewRows([]string{"id", "name", "phone", "email", "city"}).
			AddRow(1, "abc", "2345-678", "abc@gmail.com", "Bangalore").AddRow(2, "cde", "123-331-345", "cde@zopsmart.com", "Mysore"), nil, nil,
			[]entity.Employee{{ID: 1, Name: "abc", Phone: "2345-678", Email: "abc@gmail.com", City: "Bangalore"},
				{ID: 2, Name: "cde", Phone: "123-331-345", Email: "cde@zopsmart.com", City: "Mysore"}},
		},
		{"get failure", mock.NewRows([]string{"id", "name", "phone", "email", "city"}), errors.DB{}, errors.DB{Err: errors.DB{}},
			nil},
	}

	for i, tc := range tests {
		mock.ExpectQuery("SELECT (.+) FROM employees").WillReturnRows(tc.rows).WillReturnError(tc.expError)

		resp, err := New().Get(c)

		assert.Equal(t, tc.resErr, err, "TEST[%d], failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.expResp, resp)
	}
}

func TestEmployee_ScanError(t *testing.T) {
	mock, c := initializeMock()

	row := mock.NewRows([]string{"id", "name", "phone", "email", "city"}).AddRow("abc", "name", 2345, "abc@gmail.com", "kochi")

	mock.ExpectQuery("SELECT (.+) FROM employees").WillReturnRows(row).WillReturnError(nil)

	resp, err := New().Get(c)

	if err == nil {
		t.Errorf("Expected nil, but got %v", err)
	}

	assert.Nil(t, resp)
}

func TestEmployee_Create(t *testing.T) {
	mock, c := initializeMock()
	tests := []struct {
		desc     string
		input    entity.Employee
		expError error
		resError error
	}{
		{"success", entity.Employee{ID: 9, Name: "Sunita", Phone: "01234", Email: "sunita@zopsmart.com", City: "Darbhanga"}, nil, nil},
		{"success failure", entity.Employee{}, errors.DB{}, errors.DB{Err: errors.DB{}}},
	}

	for i, tc := range tests {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO employees(id, name, phone, email, city) VALUES($1, $2, $3, $4, $5)")).
			WithArgs(tc.input.ID, tc.input.Name, tc.input.Phone, tc.input.Email, tc.input.City).
			WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(tc.expError)

		err := New().Create(c, tc.input)
		assert.Equal(t, tc.resError, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
