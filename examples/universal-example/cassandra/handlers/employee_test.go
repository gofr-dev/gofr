package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/universal-example/cassandra/entity"
	"gofr.dev/examples/universal-example/cassandra/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func initializeHandlerTest(t *testing.T) (*store.MockEmployee, employee, *gofr.Gofr) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	employeeStore := store.NewMockEmployee(ctrl)
	employee := New(employeeStore)
	app := gofr.New()

	return employeeStore, employee, app
}

func TestCassandraEmployee_Get(t *testing.T) {
	tests := []struct {
		queryParams  string
		expectedResp []entity.Employee
		mockErr      error
	}{
		{"id=1", []entity.Employee{{ID: 1, Name: "Rohan", Phone: "01222", Email: "rohan@zopsmart.com", City: "Berlin"}}, nil},
		{"id=1&name=Rohan&phone=01222&email=rohan@zopsmart.com&city=Berlin",
			[]entity.Employee{{ID: 1, Name: "Rohan", Phone: "01222", Email: "rohan@zopsmart.com", City: "Berlin"}}, nil},
		{"", []entity.Employee{{ID: 1, Name: "Rohan", Phone: "01222", Email: "rohan@zopsmart.com", City: "Berlin"},
			{ID: 2, Name: "Aman", Phone: "22234", Email: "aman@zopsmart.com", City: "florida"}}, nil},
		{"id=7&name=Sunita", nil, nil},
	}

	employeeStore, employee, app := initializeHandlerTest(t)

	for i, tc := range tests {
		r := httptest.NewRequest(http.MethodGet, "/employees?"+tc.queryParams, nil)
		req := request.NewHTTPRequest(r)
		context := gofr.NewContext(nil, req, app)

		params := context.Params()

		emp := entity.Employee{Name: params["name"], Phone: params["phone"], Email: params["email"], City: params["city"]}

		emp.ID, _ = strconv.Atoi(params["id"])
		employeeStore.EXPECT().Get(context, emp).Return(tc.expectedResp)

		resp, err := employee.Get(context)
		assert.Equal(t, tc.mockErr, err, i)
		assert.Equal(t, tc.expectedResp, resp, i)
	}
}

func TestCassandraEmployee_Create(t *testing.T) {
	tests := []struct {
		query        string
		expectedResp interface{}
		mockErr      error
	}{
		{`{"id": 3, "name":"Shasank", "phone": "01567", "email":"shasank@zopsmart.com", "city":"Banglore"}`,
			[]entity.Employee{{ID: 3, Name: "Shasank", Phone: "01567", Email: "shasank@zopsmart.com", City: "Banglore"}}, nil},
		{`{"id": 4, "name":"Jay", "phone": "01933", "email":"jay@mahindra.com", "city":"Gujrat"}`,
			[]entity.Employee{{ID: 4, Name: "Jay", Phone: "01933", Email: "jay@mahindra.com", City: "Gujrat"}}, nil},
	}

	employeeStore, employee, app := initializeHandlerTest(t)

	for i, tc := range tests {
		input := strings.NewReader(tc.query)
		r := httptest.NewRequest(http.MethodPost, "/dummy", input)
		req := request.NewHTTPRequest(r)
		context := gofr.NewContext(nil, req, app)

		var emp entity.Employee

		_ = context.Bind(&emp)

		employeeStore.EXPECT().Get(context, entity.Employee{ID: emp.ID}).Return(nil)
		employeeStore.EXPECT().Create(context, emp).Return(tc.expectedResp, tc.mockErr)

		resp, err := employee.Create(context)
		assert.Equal(t, tc.mockErr, err, i)
		assert.Equal(t, tc.expectedResp, resp, i)
	}
}

func TestCassandraEmployee_Create_InvalidInput_JsonError(t *testing.T) {
	tests := []struct {
		query         string
		expectedResp  interface{}
		mockGetOutput []entity.Employee
		mockErr       error
	}{
		// Invalid Input
		{`{"id": 2, "name": "Aman", "phone": "22234", "email": "aman@zopsmart.com", "city": "Florida"}`,
			nil, []entity.Employee{{ID: 2, Name: "Aman", Phone: "22234", Email: "aman@zopsmart.com", City: "Florida"}},
			errors.EntityAlreadyExists{}},
		// JSON Error
		{`{"id":    "2", "name":   "Aman", "phone": "22234", "email": "aman@zopsmart.com", "city": "Florida"}`, nil, nil,
			&json.UnmarshalTypeError{Value: "string", Type: reflect.TypeOf(2), Offset: 13, Struct: "Employee", Field: "id"}},
	}

	employeeStore, employee, app := initializeHandlerTest(t)

	for i, tc := range tests {
		input := strings.NewReader(tc.query)
		r := httptest.NewRequest(http.MethodPost, "/dummy", input)
		req := request.NewHTTPRequest(r)
		context := gofr.NewContext(nil, req, app)

		var emp entity.Employee

		_ = context.Bind(&emp)

		employeeStore.EXPECT().Get(context, entity.Employee{ID: emp.ID}).Return(tc.mockGetOutput)

		resp, err := employee.Create(context)
		assert.Equal(t, tc.mockErr, err, i)
		assert.Equal(t, tc.expectedResp, resp, i)
	}
}
