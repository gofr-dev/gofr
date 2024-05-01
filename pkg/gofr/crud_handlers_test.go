package gofr

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

var (
	errSQLScan = errors.New("sql: Scan error on column index 0, name \"id\": converting driver.Value type string " +
		"(\"as\") to a int: invalid syntax")
)

func createTestContext(method, path, id string, body []byte, cont *container.Container) *Context {
	testReq := httptest.NewRequest(method, path+"/"+id, bytes.NewBuffer(body))
	testReq = mux.SetURLVars(testReq, map[string]string{"id": id})
	testReq.Header.Set("Content-Type", "application/json")
	gofrReq := gofrHTTP.NewRequest(testReq)

	return newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), method), gofrReq, cont)
}

func Test_scanEntity(t *testing.T) {
	var invalidObject int

	type user struct {
		ID   int
		Name string
	}

	tests := []struct {
		desc  string
		input interface{}
		resp  *entity
		err   error
	}{
		{"success case", &user{}, &entity{name: "user", entityType: reflect.TypeOf(user{}), primaryKey: "id"}, nil},
		{"invalid object", &invalidObject, nil, errInvalidObject},
	}

	for i, tc := range tests {
		resp, err := scanEntity(tc.input)

		assert.Equal(t, tc.resp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_CreateHandler(t *testing.T) {
	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	tests := []struct {
		desc          string
		dialect       string
		reqBody       []byte
		id            int
		mockErr       error
		expectedQuery string
		expectedResp  interface{}
		expectedErr   error
	}{
		{
			desc:          "success case",
			dialect:       "mysql",
			reqBody:       []byte(`{"id":1,"name":"goFr"}`),
			id:            1,
			mockErr:       nil,
			expectedQuery: "INSERT INTO user (ID, Name) VALUES (?, ?)",
			expectedResp:  "user successfully created with id: 1",
			expectedErr:   nil,
		},
		{
			desc:          "success case",
			dialect:       "postgres",
			reqBody:       []byte(`{"id":1,"name":"goFr"}`),
			id:            1,
			mockErr:       nil,
			expectedQuery: "INSERT INTO user (ID, Name) VALUES ($1, $2)",
			expectedResp:  "user successfully created with id: 1",
			expectedErr:   nil,
		},
		{
			desc:          "bind error",
			dialect:       "any-other-dialect",
			reqBody:       []byte(`{"id":"2"}`),
			id:            2,
			mockErr:       nil,
			expectedQuery: "",
			expectedResp:  nil,
			expectedErr:   &json.UnmarshalTypeError{Value: "string", Offset: 9, Struct: "user", Field: "id"},
		},
	}

	for i, tc := range tests {
		t.Run(tc.dialect+" "+tc.desc, func(t *testing.T) {
			c, mocks := container.NewMockContainer(t)

			ctrl := gomock.NewController(t)
			mockMetrics := gofrSql.NewMockMetrics(ctrl)

			ctx := createTestContext(http.MethodPost, "/users", "", tc.reqBody, c)

			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(), "type", "INSERT").MaxTimes(2)

			if tc.expectedErr == nil {
				mocks.SQL.EXPECT().Dialect().Return(tc.dialect).Times(1)
			}

			if tc.expectedQuery != "" {
				mocks.SQL.EXPECT().ExecContext(ctx, tc.expectedQuery, tc.id, "goFr").Times(1)
			}

			resp, err := e.Create(ctx)

			assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

			assert.IsType(t, tc.expectedErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func Test_GetAllHandler(t *testing.T) {
	c := container.NewContainer(nil)

	db, mock, _ := gofrSql.NewSQLMocks(t)
	defer db.Close()
	c.SQL = db

	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	tests := []struct {
		desc         string
		mockResp     *sqlmock.Rows
		mockErr      error
		expectedResp interface{}
		expectedErr  error
	}{
		{"success case", sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John Doe").AddRow(2, "Jane Doe"),
			nil, []interface{}{&user{ID: 1, Name: "John Doe"}, &user{ID: 2, Name: "Jane Doe"}}, nil},
		{"error retrieving rows", sqlmock.NewRows([]string{"id", "name"}), errTest, nil, errTest},
		{"error scanning rows", sqlmock.NewRows([]string{"id", "name"}).AddRow("as", ""),
			nil, nil, errSQLScan},
	}

	for i, tc := range tests {
		ctx := createTestContext(http.MethodGet, "/users", "", nil, c)

		mock.ExpectQuery("SELECT * FROM user").WillReturnRows(tc.mockResp).WillReturnError(tc.mockErr)

		resp, err := e.GetAll(ctx)

		assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		if tc.expectedErr != nil {
			assert.Equal(t, tc.expectedErr.Error(), err.Error(), "TEST[%d], Failed.\n%s", i, tc.desc)
		} else {
			assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		}
	}
}

func Test_GetHandler(t *testing.T) {
	c := container.NewContainer(nil)

	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	dialectCases := []struct {
		dialect       string
		expectedQuery string
	}{
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM user WHERE id = ?",
		},
		{
			dialect:       "postgres",
			expectedQuery: "SELECT * FROM user WHERE id = $1",
		},
	}

	type testCase struct {
		desc         string
		id           string
		mockRow      *sqlmock.Rows
		expectedResp interface{}
		expectedErr  error
	}

	for _, dc := range dialectCases {
		tests := []testCase{
			{
				desc:         "success case",
				id:           "1",
				mockRow:      sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John Doe"),
				expectedResp: &user{ID: 1, Name: "John Doe"},
				expectedErr:  nil,
			},
			{
				desc:         "no rows found",
				id:           "2",
				mockRow:      sqlmock.NewRows(nil),
				expectedResp: nil,
				expectedErr:  sql.ErrNoRows,
			},
			{
				desc:         "error scanning rows",
				id:           "3",
				mockRow:      sqlmock.NewRows([]string{"id", "name"}).AddRow("as", ""),
				expectedResp: nil,
				expectedErr:  errSQLScan,
			},
		}

		db, mock, mockMetrics := gofrSql.NewSQLMocksWithConfig(t, &gofrSql.DBConfig{Dialect: dc.dialect})
		c.SQL = db

		for i, tc := range tests {
			t.Run(dc.dialect+" "+tc.desc, func(t *testing.T) {
				ctx := createTestContext(http.MethodGet, "/user", tc.id, nil, c)

				mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(), "type", "SELECT")
				mock.ExpectQuery(dc.expectedQuery).WithArgs(tc.id).WillReturnRows(tc.mockRow)

				resp, err := e.Get(ctx)

				assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

				if tc.expectedErr != nil {
					assert.Equal(t, tc.expectedErr.Error(), err.Error(), "TEST[%d], Failed.\n%s", i, tc.desc)
				} else {
					assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				}
			})
		}

		t.Cleanup(func() {
			db.Close()
		})
	}
}

func Test_UpdateHandler(t *testing.T) {
	c := container.NewContainer(nil)

	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	dialectCases := []struct {
		dialect       string
		expectedQuery string
	}{
		{
			dialect:       "mysql",
			expectedQuery: "UPDATE user SET Name=? WHERE id = 1",
		},
		{
			dialect:       "postgres",
			expectedQuery: "UPDATE user SET Name=$1 WHERE id = 1",
		},
	}

	type testCase struct {
		desc         string
		id           string
		reqBody      []byte
		mockErr      error
		expectedResp interface{}
		expectedErr  error
	}

	for _, dc := range dialectCases {
		tests := []testCase{
			{
				desc:         "success case",
				id:           "1",
				reqBody:      []byte(`{"id":1,"name":"goFr"}`),
				mockErr:      nil,
				expectedResp: "user successfully updated with id: 1",
				expectedErr:  nil,
			},
			{
				desc:         "bind error",
				id:           "2",
				reqBody:      []byte(`{"id":"2"}`),
				mockErr:      nil,
				expectedResp: nil,
				expectedErr:  &json.UnmarshalTypeError{Value: "string", Offset: 9, Struct: "user", Field: "id"},
			},
			{
				desc:         "error From DB",
				id:           "3",
				reqBody:      []byte(`{"id":3,"name":"goFr"}`),
				mockErr:      sqlmock.ErrCancelled,
				expectedResp: nil,
				expectedErr:  sqlmock.ErrCancelled,
			},
		}

		db, mock, mockMetrics := gofrSql.NewSQLMocksWithConfig(t, &gofrSql.DBConfig{Dialect: dc.dialect})
		c.SQL = db

		for i, tc := range tests {
			t.Run(dc.dialect+" "+tc.desc, func(t *testing.T) {
				ctx := createTestContext(http.MethodPut, "/user", tc.id, tc.reqBody, c)

				mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(),
					"type", "UPDATE").MaxTimes(2)

				mock.ExpectExec(dc.expectedQuery).WithArgs("goFr").
					WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(nil)

				resp, err := e.Update(ctx)

				assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

				assert.IsType(t, tc.expectedErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			})
		}

		t.Cleanup(func() {
			db.Close()
		})
	}
}

func Test_DeleteHandler(t *testing.T) {
	c, mocks := container.NewMockContainer(t)

	e := entity{
		name:       "user",
		entityType: nil,
		primaryKey: "id",
	}

	dialectCases := []struct {
		dialect       string
		expectedQuery string
	}{
		{
			dialect:       "mysql",
			expectedQuery: "DELETE FROM user WHERE id = ?",
		},
		{
			dialect:       "postgres",
			expectedQuery: "DELETE FROM user WHERE id = $1",
		},
	}

	type testCase struct {
		desc         string
		id           string
		mockResp     driver.Result
		mockErr      error
		expectedErr  error
		expectedResp interface{}
	}

	for _, dc := range dialectCases {
		tests := []testCase{
			{
				desc:         "success case",
				id:           "1",
				mockResp:     sqlmock.NewResult(1, 1),
				mockErr:      nil,
				expectedErr:  nil,
				expectedResp: "user successfully deleted with id: 1",
			},
			{
				desc:         "SQL error case",
				id:           "2",
				mockResp:     nil,
				mockErr:      errTest,
				expectedErr:  errTest,
				expectedResp: nil,
			},
			{
				desc:         "no rows affected",
				id:           "3",
				mockResp:     sqlmock.NewResult(0, 0),
				mockErr:      nil,
				expectedErr:  errEntityNotFound,
				expectedResp: nil,
			},
		}
		for i, tc := range tests {
			t.Run(dc.dialect+" "+tc.desc, func(t *testing.T) {
				ctx := createTestContext(http.MethodDelete, "/user", tc.id, nil, c)

				mocks.SQL.EXPECT().Dialect().Return(dc.dialect)
				mocks.SQL.EXPECT().ExecContext(ctx, dc.expectedQuery, tc.id).Return(tc.mockResp, tc.mockErr)

				resp, err := e.Delete(ctx)

				assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

				assert.Equal(t, tc.expectedErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			})
		}
	}
}
