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

	return newContext(gofrHTTP.NewResponder(httptest.NewRecorder()), gofrReq, cont)
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
	cont := container.NewEmptyContainer()

	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	db, mock, mockMetrics := gofrSql.NewMockSQLDB(t)
	defer db.Close()
	cont.SQL = db

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	tests := []struct {
		desc        string
		reqBody     []byte
		id          int
		mockErr     error
		expectedRes interface{}
		expectedErr error
	}{
		{
			desc:        "Success Case",
			reqBody:     []byte(`{"id":1,"name":"goFr"}`),
			id:          1,
			mockErr:     nil,
			expectedRes: "user successfully created with id: 1",
			expectedErr: nil,
		},
		{
			desc:        "Bing Error",
			reqBody:     []byte(`{"id":"2"}`),
			id:          2,
			mockErr:     nil,
			expectedRes: nil,
			expectedErr: &json.UnmarshalTypeError{Value: "string", Offset: 9, Struct: "user", Field: "id"},
		},
		{
			desc:        "DB Error Case",
			reqBody:     []byte(`{"id":3,"name":"goFr"}`),
			id:          3,
			mockErr:     errTest,
			expectedRes: nil,
			expectedErr: errTest,
		},
	}

	for i, tc := range tests {
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(), "type", "INSERT").
			MaxTimes(2)

		mock.ExpectExec("INSERT INTO user (ID, Name) VALUES (?, ?)").WithArgs(tc.id, "goFr").
			WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(tc.expectedErr)

		c := createTestContext(http.MethodGet, "/users", "", tc.reqBody, cont)

		res, err := e.Create(c)

		assert.Equal(t, tc.expectedRes, res, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.IsType(t, tc.expectedErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_GetAllHandler(t *testing.T) {
	cont := container.NewEmptyContainer()

	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	db, mock, _ := gofrSql.NewMockSQLDB(t)
	defer db.Close()
	cont.SQL = db

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	tests := []struct {
		desc        string
		mockRows    *sqlmock.Rows
		mockErr     error
		expectedRes interface{}
		expectedErr error
	}{
		{
			desc: "Success Case",
			mockRows: sqlmock.NewRows([]string{"id", "name"}).
				AddRow(1, "John Doe").
				AddRow(2, "Jane Doe"),
			expectedRes: []interface{}{
				&user{ID: 1, Name: "John Doe"},
				&user{ID: 2, Name: "Jane Doe"},
			},
			expectedErr: nil,
		},
		{
			desc:        "Error Retrieving Rows",
			mockRows:    sqlmock.NewRows([]string{"id", "name"}),
			mockErr:     errTest,
			expectedRes: nil,
			expectedErr: errTest,
		},
		{
			desc: "Error Scanning Rows",
			mockRows: sqlmock.NewRows([]string{"id", "name"}).
				AddRow("as", ""),
			expectedRes: nil,
			expectedErr: errSQLScan,
		},
	}

	for i, tt := range tests {
		mock.ExpectQuery("SELECT * FROM user").WillReturnRows(tt.mockRows).WillReturnError(tt.mockErr)

		c := createTestContext(http.MethodGet, "/users", "", nil, cont)

		res, err := e.GetAll(c)

		assert.Equal(t, tt.expectedRes, res, "TEST[%d], Failed.\n%s", i, tt.desc)

		if tt.expectedErr != nil {
			assert.Equal(t, tt.expectedErr.Error(), err.Error(), "TEST[%d], Failed.\n%s", i, tt.desc)
		} else {
			assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tt.desc)
		}
	}
}

func Test_GetHandler(t *testing.T) {
	cont := container.NewEmptyContainer()

	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	db, mock, mockMetrics := gofrSql.NewMockSQLDB(t)
	defer db.Close()
	cont.SQL = db

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	tests := []struct {
		desc        string
		id          string
		mockRow     *sqlmock.Rows
		expectedRes interface{}
		expectedErr error
	}{
		{
			desc:        "Success Case",
			id:          "1",
			mockRow:     sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John Doe"), // Mocked row data
			expectedRes: &user{ID: 1, Name: "John Doe"},                                // Expected result
			expectedErr: nil,
		},
		{
			desc:        "No Rows Found",
			id:          "2",
			mockRow:     sqlmock.NewRows(nil), // No rows returned
			expectedRes: nil,
			expectedErr: sql.ErrNoRows,
		},
		{
			desc:        "Error Scanning Rows",
			id:          "3",
			mockRow:     sqlmock.NewRows([]string{"id", "name"}).AddRow("as", ""),
			expectedRes: nil,
			expectedErr: errSQLScan,
		},
	}

	for i, tt := range tests {
		c := createTestContext(http.MethodGet, "/user", tt.id, nil, cont)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(), "type", "SELECT")

		mock.ExpectQuery("SELECT * FROM user WHERE id = ?").WithArgs(tt.id).WillReturnRows(tt.mockRow)

		res, err := e.Get(c)

		assert.Equal(t, tt.expectedRes, res, "TEST[%d], Failed.\n%s", i, tt.desc)

		if tt.expectedErr != nil {
			assert.Equal(t, tt.expectedErr.Error(), err.Error(), "TEST[%d], Failed.\n%s", i, tt.desc)
		} else {
			assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tt.desc)
		}
	}
}

func Test_UpdateHandler(t *testing.T) {
	cont := container.NewEmptyContainer()

	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	db, mock, mockMetrics := gofrSql.NewMockSQLDB(t)
	defer db.Close()
	cont.SQL = db

	e := entity{
		name:       "user",
		entityType: reflect.TypeOf(user{}),
		primaryKey: "id",
	}

	tests := []struct {
		desc           string
		id             string
		reqBody        []byte
		mockErr        error
		expectedResult interface{}
		expectedErr    error
	}{
		{
			desc:           "Success Case",
			id:             "1",
			reqBody:        []byte(`{"id":1,"name":"goFr"}`),
			mockErr:        nil,
			expectedResult: "user successfully updated with id: 1",
			expectedErr:    nil,
		},
		{
			desc:           "Bind Error",
			id:             "2",
			reqBody:        []byte(`{"id":"2"}`),
			mockErr:        nil,
			expectedResult: nil,
			expectedErr:    &json.UnmarshalTypeError{Value: "string", Offset: 9, Struct: "user", Field: "id"},
		},
		{
			desc:           "Error From DB",
			id:             "3",
			reqBody:        []byte(`{"id":3,"name":"goFr"}`),
			mockErr:        sqlmock.ErrCancelled,
			expectedResult: nil,
			expectedErr:    sqlmock.ErrCancelled,
		},
	}

	for i, tt := range tests {
		c := createTestContext(http.MethodPut, "/user", tt.id, tt.reqBody, cont)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(),
			"type", "UPDATE").MaxTimes(2)

		mock.ExpectExec("UPDATE user SET Name=? WHERE id = 1").WithArgs("goFr").
			WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(nil)

		res, err := e.Update(c)

		assert.Equal(t, tt.expectedResult, res, "TEST[%d], Failed.\n%s", i, tt.desc)
		assert.IsType(t, tt.expectedErr, err, "TEST[%d], Failed.\n%s", i, tt.desc)
	}
}

func Test_DeleteHandler(t *testing.T) {
	cont := container.NewEmptyContainer()

	db, mock, mockMetrics := gofrSql.NewMockSQLDB(t)
	defer db.Close()
	cont.SQL = db

	e := entity{
		name:       "user",
		entityType: nil,
		primaryKey: "id",
	}

	tests := []struct {
		desc       string
		id         string
		mockReturn driver.Result
		mockErr    error
		wantErr    error
		wantRes    interface{}
	}{
		{
			desc:       "Success Case",
			id:         "1",
			mockReturn: sqlmock.NewResult(1, 1),
			mockErr:    nil,
			wantErr:    nil,
			wantRes:    "user successfully deleted with id: 1",
		},
		{
			desc:       "SQL Error Case",
			id:         "2",
			mockReturn: nil,
			mockErr:    errTest,
			wantErr:    errTest,
			wantRes:    nil,
		},
		{
			desc:       "No Rows Affected Case",
			id:         "3",
			mockReturn: sqlmock.NewResult(0, 0),
			mockErr:    nil,
			wantErr:    errEntityNotFound,
			wantRes:    nil,
		},
	}

	for i, tt := range tests {
		c := createTestContext(http.MethodDelete, "/user", tt.id, nil, cont)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(), "type", "DELETE")

		mock.ExpectExec("DELETE FROM user WHERE id = ?").WithArgs(tt.id).
			WillReturnResult(tt.mockReturn).WillReturnError(tt.mockErr)

		res, err := e.Delete(c)

		assert.Equal(t, tt.wantRes, res, "TEST[%d], Failed.\n%s", i, tt.desc)
		assert.Equal(t, tt.wantErr, err, "TEST[%d], Failed.\n%s", i, tt.desc)
	}
}
