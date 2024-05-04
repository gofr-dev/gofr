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
	"strconv"
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

	errMock = errors.New("mock error")
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
			expectedQuery: "INSERT INTO `user` (`id`, `name`) VALUES (?, ?)",
			expectedResp:  "user successfully created with id: 1",
			expectedErr:   nil,
		},
		{
			desc:          "success case",
			dialect:       "postgres",
			reqBody:       []byte(`{"id":1,"name":"goFr"}`),
			id:            1,
			mockErr:       nil,
			expectedQuery: `INSERT INTO "user" ("id", "name") VALUES ($1, $2)`,
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

//nolint:funlen // test fails when used 2 loops in test cases approach
func Test_GetAllHandler(t *testing.T) {
	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	type testCase struct {
		dialect       string
		expectedQuery string
		desc          string
		mockResp      *sqlmock.Rows
		mockErr       error
		expectedResp  interface{}
		expectedErr   error
	}

	testCases := []testCase{
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user`",
			desc:          "success case",
			mockResp:      sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John Doe").AddRow(2, "Jane Doe"),
			mockErr:       nil,
			expectedResp:  []interface{}{&user{ID: 1, Name: "John Doe"}, &user{ID: 2, Name: "Jane Doe"}},
			expectedErr:   nil,
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user"`,
			desc:          "success case",
			mockResp:      sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John Doe").AddRow(2, "Jane Doe"),
			mockErr:       nil,
			expectedResp:  []interface{}{&user{ID: 1, Name: "John Doe"}, &user{ID: 2, Name: "Jane Doe"}},
			expectedErr:   nil,
		},
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user`",
			desc:          "error retrieving rows",
			mockResp:      sqlmock.NewRows([]string{"id", "name"}),
			mockErr:       errMock,
			expectedResp:  nil,
			expectedErr:   errMock,
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user"`,
			desc:          "error retrieving rows",
			mockResp:      sqlmock.NewRows([]string{"id", "name"}),
			mockErr:       errMock,
			expectedResp:  nil,
			expectedErr:   errMock,
		},
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user`",
			desc:          "error scanning rows",
			mockResp:      sqlmock.NewRows([]string{"id", "name"}).AddRow("as", ""),
			mockErr:       nil,
			expectedResp:  nil,
			expectedErr:   errSQLScan,
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user"`,
			desc:          "error scanning rows",
			mockResp:      sqlmock.NewRows([]string{"id", "name"}).AddRow("as", ""),
			mockErr:       nil,
			expectedResp:  nil,
			expectedErr:   errSQLScan,
		},
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user`",
			desc:          "error retrieving rows",
			mockResp:      sqlmock.NewRows([]string{"id", "name"}),
			mockErr:       errTest,
			expectedResp:  nil,
			expectedErr:   errTest,
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user"`,
			desc:          "error retrieving rows",
			mockResp:      sqlmock.NewRows([]string{"id", "name"}),
			mockErr:       errTest,
			expectedResp:  nil,
			expectedErr:   errTest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.dialect+" "+tc.desc, func(t *testing.T) {
			c := container.NewContainer(nil)
			db, mock, _ := gofrSql.NewSQLMocksWithConfig(t, &gofrSql.DBConfig{Dialect: tc.dialect})
			c.SQL = db

			defer db.Close()

			ctx := createTestContext(http.MethodGet, "/users", "", nil, c)

			mock.ExpectQuery(tc.expectedQuery).WillReturnRows(tc.mockResp).WillReturnError(tc.mockErr)

			resp, err := e.GetAll(ctx)

			assert.Equal(t, tc.expectedResp, resp, "Failed.\n%s", tc.desc)

			if tc.expectedErr != nil {
				assert.Equal(t, tc.expectedErr.Error(), err.Error(), "Failed.\n%s", tc.desc)
			} else {
				assert.Nil(t, err, "Failed.\n%s", tc.desc)
			}
		})
	}
}

//nolint:funlen // test fails when used 2 loops in test cases approach
func Test_GetHandler(t *testing.T) {
	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	type testCase struct {
		dialect       string
		expectedQuery string
		desc          string
		id            string
		mockRow       *sqlmock.Rows
		mockErr       error
		expectedResp  interface{}
		expectedErr   error
	}

	testCases := []testCase{
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user` WHERE `id`=?",
			desc:          "success case",
			id:            "1",
			mockRow:       sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John Doe"),
			mockErr:       nil,
			expectedResp:  &user{ID: 1, Name: "John Doe"},
			expectedErr:   nil,
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user" WHERE "id"=$1`,
			desc:          "success case",
			id:            "1",
			mockRow:       sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John Doe"),
			mockErr:       nil,
			expectedResp:  &user{ID: 1, Name: "John Doe"},
			expectedErr:   nil,
		},
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user` WHERE `id`=?",
			desc:          "no rows found",
			id:            "2",
			mockRow:       sqlmock.NewRows(nil),
			mockErr:       nil,
			expectedResp:  nil,
			expectedErr:   sql.ErrNoRows,
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user" WHERE "id"=$1`,
			desc:          "no rows found",
			id:            "2",
			mockRow:       sqlmock.NewRows(nil),
			mockErr:       nil,
			expectedResp:  nil,
			expectedErr:   sql.ErrNoRows,
		},
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user` WHERE `id`=?",
			desc:          "error scanning rows",
			id:            "3",
			mockRow:       sqlmock.NewRows([]string{"id", "name"}).AddRow("as", ""),
			mockErr:       nil,
			expectedResp:  nil,
			expectedErr:   errSQLScan,
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user" WHERE "id"=$1`,
			desc:          "error scanning rows",
			id:            "3",
			mockRow:       sqlmock.NewRows([]string{"id", "name"}).AddRow("as", ""),
			mockErr:       nil,
			expectedResp:  nil,
			expectedErr:   errSQLScan,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.dialect+" "+tc.desc, func(t *testing.T) {
			c := container.NewContainer(nil)
			db, mock, mockMetrics := gofrSql.NewSQLMocksWithConfig(t, &gofrSql.DBConfig{Dialect: tc.dialect})
			c.SQL = db

			defer db.Close()

			ctx := createTestContext(http.MethodGet, "/user", tc.id, nil, c)

			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(), "type", "SELECT")
			mock.ExpectQuery(tc.expectedQuery).WithArgs(tc.id).WillReturnRows(tc.mockRow).WillReturnError(tc.mockErr)

			resp, err := e.Get(ctx)

			assert.Equal(t, tc.expectedResp, resp, "Failed.\n%s", tc.desc)

			if tc.expectedErr != nil {
				assert.Equal(t, tc.expectedErr.Error(), err.Error(), "Failed.\n%s", tc.desc)
			} else {
				assert.Nil(t, err, "Failed.\n%s", tc.desc)
			}
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
			expectedQuery: "UPDATE `user` SET `name`=? WHERE `id`=?",
		},
		{
			dialect:       "postgres",
			expectedQuery: `UPDATE "user" SET "name"=$1 WHERE "id"=$2`,
		},
	}

	type testCase struct {
		desc         string
		id           int
		reqBody      []byte
		mockErr      error
		expectedResp interface{}
		expectedErr  error
	}

	for _, dc := range dialectCases {
		tests := []testCase{
			{
				desc:         "success case",
				id:           1,
				reqBody:      []byte(`{"id":1,"name":"goFr"}`),
				mockErr:      nil,
				expectedResp: "user successfully updated with id: 1",
				expectedErr:  nil,
			},
			{
				desc:         "bind error",
				id:           2,
				reqBody:      []byte(`{"id":"2"}`),
				mockErr:      nil,
				expectedResp: nil,
				expectedErr:  &json.UnmarshalTypeError{Value: "string", Offset: 9, Struct: "user", Field: "id"},
			},
			{
				desc:         "error From DB",
				id:           3,
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
				ctx := createTestContext(http.MethodPut, "/user", strconv.Itoa(tc.id), tc.reqBody, c)

				mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(),
					"type", "UPDATE").MaxTimes(2)

				mock.ExpectExec(dc.expectedQuery).WithArgs("goFr", tc.id).
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
			expectedQuery: "DELETE FROM `user` WHERE `id`=?",
		},
		{
			dialect:       "postgres",
			expectedQuery: `DELETE FROM "user" WHERE "id"=$1`,
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
