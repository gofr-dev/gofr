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

type userEntity struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	IsEmployed bool   `json:"isEmployed"`
}

func (u *userEntity) TableName() string {
	return "user"
}

func (u *userEntity) RestPath() string {
	return "users"
}

func Test_scanEntity(t *testing.T) {
	var invalidObject int

	type userTestEntity struct {
		ID   int
		Name string
	}

	tests := []struct {
		desc  string
		input interface{}
		resp  *entity
		err   error
	}{
		{
			desc:  "success case (default)",
			input: &userTestEntity{},
			resp: &entity{
				name:       "userTestEntity",
				entityType: reflect.TypeOf(userTestEntity{}),
				primaryKey: "id",
				tableName:  "user_test_entity",
				restPath:   "userTestEntity",
			},
			err: nil,
		},
		{
			desc:  "success case (custom)",
			input: &userEntity{},
			resp: &entity{
				name:       "userEntity",
				entityType: reflect.TypeOf(userEntity{}),
				primaryKey: "id",
				tableName:  "user",
				restPath:   "users",
			},
			err: nil,
		},
		{
			desc:  "invalid object",
			input: &invalidObject,
			resp:  nil,
			err:   errInvalidObject,
		},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			resp, err := scanEntity(tc.input)

			assert.Equal(t, tc.resp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

			assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

type mockTableName struct {
	tableName string
}

func (m *mockTableName) TableName() string {
	return m.tableName
}

func Test_getTableName(t *testing.T) {
	tests := []struct {
		name       string
		object     interface{}
		structName string
		want       string
	}{
		{
			name:       "Test with TableNameOverrider interface",
			object:     &mockTableName{tableName: "custom_table"},
			structName: "mockTableName",
			want:       "custom_table",
		},
		{
			name:       "Test without TableNameOverrider interface",
			object:     &struct{}{},
			structName: "TestStruct",
			want:       "test_struct",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTableName(tt.object, tt.structName)
			assert.Equal(t, tt.want, got)
		})
	}
}

type mockRestPath struct {
	restPath string
}

func (m *mockRestPath) RestPath() string {
	return m.restPath
}

func Test_getRestPath(t *testing.T) {
	tests := []struct {
		name       string
		object     interface{}
		structName string
		want       string
	}{
		{
			name:       "Test with RestPathOverrider interface",
			object:     &mockRestPath{restPath: "custom_path"},
			structName: "mockRestPath",
			want:       "custom_path",
		},
		{
			name:       "Test without RestPathOverrider interface",
			object:     &struct{}{},
			structName: "TestStruct",
			want:       "TestStruct",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRestPath(tt.object, tt.structName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_CreateHandler(t *testing.T) {
	e := entity{
		name:       "userEntity",
		entityType: reflect.TypeOf(userEntity{}),
		primaryKey: "id",
		tableName:  "user_entity",
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
			reqBody:       []byte(`{"id":1,"name":"goFr","isEmployed":true}`),
			id:            1,
			mockErr:       nil,
			expectedQuery: "INSERT INTO `user_entity` (`id`, `name`, `is_employed`) VALUES (?, ?, ?)",
			expectedResp:  "userEntity successfully created with id: 1",
			expectedErr:   nil,
		},
		{
			desc:          "success case",
			dialect:       "postgres",
			reqBody:       []byte(`{"id":1,"name":"goFr","isEmployed":true}`),
			id:            1,
			mockErr:       nil,
			expectedQuery: `INSERT INTO "user_entity" ("id", "name", "is_employed") VALUES ($1, $2, $3)`,
			expectedResp:  "userEntity successfully created with id: 1",
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
			expectedErr:   &json.UnmarshalTypeError{Value: "string", Offset: 9, Struct: "userEntity", Field: "id"},
		},
	}

	for i, tc := range tests {
		t.Run(tc.dialect+" "+tc.desc, func(t *testing.T) {
			c, mocks := container.NewMockContainer(t)

			ctrl := gomock.NewController(t)
			mockMetrics := gofrSql.NewMockMetrics(ctrl)

			ctx := createTestContext(http.MethodPost, "/users", "", tc.reqBody, c)

			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(),
				"hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT").MaxTimes(2)

			if tc.expectedErr == nil {
				mocks.SQL.EXPECT().Dialect().Return(tc.dialect).Times(1)
			}

			if tc.expectedQuery != "" {
				mocks.SQL.EXPECT().ExecContext(ctx, tc.expectedQuery, tc.id, "goFr", true).Times(1)
			}

			resp, err := e.Create(ctx)

			assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

			assert.IsType(t, tc.expectedErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func Test_GetAllHandler(t *testing.T) {
	e := entity{
		name:       "userEntity",
		entityType: reflect.TypeOf(userEntity{}),
		primaryKey: "id",
		tableName:  "user_entity",
	}

	dialectCases := []struct {
		dialect       string
		expectedQuery string
	}{
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user_entity`",
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user_entity"`,
		},
	}

	type testCase struct {
		desc         string
		mockResp     *sqlmock.Rows
		mockErr      error
		expectedResp interface{}
		expectedErr  error
	}

	for _, dc := range dialectCases {
		tests := []testCase{
			{
				desc:     "success case",
				mockResp: sqlmock.NewRows([]string{"id", "name", "is_employed"}).AddRow(1, "John Doe", true).AddRow(2, "Jane Doe", false),
				mockErr:  nil,
				expectedResp: []interface{}{&userEntity{ID: 1, Name: "John Doe", IsEmployed: true},
					&userEntity{ID: 2, Name: "Jane Doe", IsEmployed: false}},
				expectedErr: nil,
			},
			{
				desc:         "error retrieving rows",
				mockResp:     sqlmock.NewRows([]string{"id", "name", "is_employed"}),
				mockErr:      errMock,
				expectedResp: nil,
				expectedErr:  errMock,
			},
			{
				desc:         "error scanning rows",
				mockResp:     sqlmock.NewRows([]string{"id", "name", "is_employed"}).AddRow("as", "", false),
				mockErr:      nil,
				expectedResp: nil,
				expectedErr:  errSQLScan,
			},
			{
				desc:         "error retrieving rows",
				mockResp:     sqlmock.NewRows([]string{"id", "name", "is_employed"}),
				mockErr:      errTest,
				expectedResp: nil,
				expectedErr:  errTest,
			},
		}
		for i, tc := range tests {
			t.Run(dc.dialect+" "+tc.desc, func(t *testing.T) {
				c := container.NewContainer(nil)
				db, mock, _ := gofrSql.NewSQLMocksWithConfig(t, &gofrSql.DBConfig{Dialect: dc.dialect})
				c.SQL = db

				defer db.Close()

				ctx := createTestContext(http.MethodGet, "/users", "", nil, c)

				mock.ExpectQuery(dc.expectedQuery).WillReturnRows(tc.mockResp).WillReturnError(tc.mockErr)

				resp, err := e.GetAll(ctx)

				assert.Equal(t, tc.expectedResp, resp, "Failed.\n%s", tc.desc)

				if tc.expectedErr != nil {
					assert.Equal(t, tc.expectedErr.Error(), err.Error(), "TEST[%d], Failed.\n%s", i, tc.desc)
				} else {
					assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				}
			})
		}
	}
}

func Test_GetHandler(t *testing.T) {
	e := entity{
		name:       "userEntity",
		entityType: reflect.TypeOf(userEntity{}),
		primaryKey: "id",
		tableName:  "user_entity",
	}

	dialectCases := []struct {
		dialect       string
		expectedQuery string
	}{
		{
			dialect:       "mysql",
			expectedQuery: "SELECT * FROM `user_entity` WHERE `id`=?",
		},
		{
			dialect:       "postgres",
			expectedQuery: `SELECT * FROM "user_entity" WHERE "id"=$1`,
		},
	}

	type testCase struct {
		desc         string
		id           string
		mockRow      *sqlmock.Rows
		mockErr      error
		expectedResp interface{}
		expectedErr  error
	}

	for _, dc := range dialectCases {
		testCases := []testCase{
			{
				desc:         "success case",
				id:           "1",
				mockRow:      sqlmock.NewRows([]string{"id", "name", "is_employed"}).AddRow(1, "John Doe", true),
				mockErr:      nil,
				expectedResp: &userEntity{ID: 1, Name: "John Doe", IsEmployed: true},
				expectedErr:  nil,
			},
			{
				desc:         "no rows found",
				id:           "2",
				mockRow:      sqlmock.NewRows(nil),
				mockErr:      nil,
				expectedResp: nil,
				expectedErr:  sql.ErrNoRows,
			},
			{
				desc:         "error scanning rows",
				id:           "3",
				mockRow:      sqlmock.NewRows([]string{"id", "name", "is_employed"}).AddRow("as", "", false),
				mockErr:      nil,
				expectedResp: nil,
				expectedErr:  errSQLScan,
			},
		}

		for _, tc := range testCases {
			t.Run(dc.dialect+" "+tc.desc, func(t *testing.T) {
				c := container.NewContainer(nil)
				db, mock, _ := gofrSql.NewSQLMocksWithConfig(t, &gofrSql.DBConfig{Dialect: dc.dialect})
				c.SQL = db

				defer db.Close()

				ctx := createTestContext(http.MethodGet, "/user", tc.id, nil, c)

				mock.ExpectQuery(dc.expectedQuery).WithArgs(tc.id).WillReturnRows(tc.mockRow).WillReturnError(tc.mockErr)

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
}

func Test_UpdateHandler(t *testing.T) {
	c := container.NewContainer(nil)

	e := entity{
		name:       "userEntity",
		entityType: reflect.TypeOf(userEntity{}),
		primaryKey: "id",
		tableName:  "user_entity",
	}

	dialectCases := []struct {
		dialect       string
		expectedQuery string
	}{
		{
			dialect:       "mysql",
			expectedQuery: "UPDATE `user_entity` SET `name`=?, `is_employed`=? WHERE `id`=?",
		},
		{
			dialect:       "postgres",
			expectedQuery: `UPDATE "user_entity" SET "name"=$1, "is_employed"=$2 WHERE "id"=$3`,
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
				reqBody:      []byte(`{"id":1,"name":"goFr","isEmployed":true}`),
				mockErr:      nil,
				expectedResp: "userEntity successfully updated with id: 1",
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
				reqBody:      []byte(`{"id":3,"name":"goFr","isEmployed":false}`),
				mockErr:      sqlmock.ErrCancelled,
				expectedResp: nil,
				expectedErr:  sqlmock.ErrCancelled,
			},
		}

		db, mock, _ := gofrSql.NewSQLMocksWithConfig(t, &gofrSql.DBConfig{Dialect: dc.dialect})
		c.SQL = db

		for i, tc := range tests {
			t.Run(dc.dialect+" "+tc.desc, func(t *testing.T) {
				ctx := createTestContext(http.MethodPut, "/user", strconv.Itoa(tc.id), tc.reqBody, c)

				mock.ExpectExec(dc.expectedQuery).WithArgs("goFr", true, tc.id).
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
		name:       "userEntity",
		entityType: nil,
		primaryKey: "id",
		tableName:  "user_entity",
	}

	dialectCases := []struct {
		dialect       string
		expectedQuery string
	}{
		{
			dialect:       "mysql",
			expectedQuery: "DELETE FROM `user_entity` WHERE `id`=?",
		},
		{
			dialect:       "postgres",
			expectedQuery: `DELETE FROM "user_entity" WHERE "id"=$1`,
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
				expectedResp: "userEntity successfully deleted with id: 1",
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
