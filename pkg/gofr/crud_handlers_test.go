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
	c, mocks := container.NewMockContainer(t)

	ctrl := gomock.NewController(t)
	mockMetrics := gofrSql.NewMockMetrics(ctrl)

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
		reqBody      []byte
		id           int
		mockErr      error
		expectedResp interface{}
		expectedErr  error
	}{
		{"success case", []byte(`{"id":1,"name":"goFr"}`), 1, nil,
			"user successfully created with id: 1", nil},
		{"bind error", []byte(`{"id":"2"}`), 2, nil, nil,
			&json.UnmarshalTypeError{Value: "string", Offset: 9, Struct: "user", Field: "id"}},
	}

	for i, tc := range tests {
		ctx := createTestContext(http.MethodGet, "/users", "", tc.reqBody, c)

		mockMetrics.EXPECT().RecordHistogram(ctx, "app_sql_stats", gomock.Any(), "type", "INSERT").MaxTimes(2)
		mocks.SQL.EXPECT().ExecContext(ctx, "INSERT INTO user (ID, Name) VALUES (?, ?)", tc.id, "goFr").MaxTimes(2)

		resp, err := e.Create(ctx)

		assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.IsType(t, tc.expectedErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
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

	db, mock, mockMetrics := gofrSql.NewSQLMocks(t)
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
		id           string
		mockRow      *sqlmock.Rows
		expectedResp interface{}
		expectedErr  error
	}{
		{"success case", "1", sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John Doe"),
			&user{ID: 1, Name: "John Doe"}, nil},
		{"no rows found", "2", sqlmock.NewRows(nil), nil, sql.ErrNoRows},
		{"error scanning rows", "3", sqlmock.NewRows([]string{"id", "name"}).AddRow("as", ""),
			nil, errSQLScan},
	}

	for i, tc := range tests {
		ctx := createTestContext(http.MethodGet, "/user", tc.id, nil, c)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(), "type", "SELECT")
		mock.ExpectQuery("SELECT * FROM user WHERE id = ?").WithArgs(tc.id).WillReturnRows(tc.mockRow)

		resp, err := e.Get(ctx)

		assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		if tc.expectedErr != nil {
			assert.Equal(t, tc.expectedErr.Error(), err.Error(), "TEST[%d], Failed.\n%s", i, tc.desc)
		} else {
			assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		}
	}
}

func Test_UpdateHandler(t *testing.T) {
	c := container.NewContainer(nil)

	db, mock, mockMetrics := gofrSql.NewSQLMocks(t)
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
		id           string
		reqBody      []byte
		mockErr      error
		expectedResp interface{}
		expectedErr  error
	}{
		{"success case", "1", []byte(`{"id":1,"name":"goFr"}`), nil,
			"user successfully updated with id: 1", nil},
		{"bind error", "2", []byte(`{"id":"2"}`), nil, nil,
			&json.UnmarshalTypeError{Value: "string", Offset: 9, Struct: "user", Field: "id"}},
		{"error From DB", "3", []byte(`{"id":3,"name":"goFr"}`), sqlmock.ErrCancelled,
			nil, sqlmock.ErrCancelled},
	}

	for i, tc := range tests {
		ctx := createTestContext(http.MethodPut, "/user", tc.id, tc.reqBody, c)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(),
			"type", "UPDATE").MaxTimes(2)

		mock.ExpectExec("UPDATE user SET Name=? WHERE id = 1").WithArgs("goFr").
			WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(nil)

		resp, err := e.Update(ctx)

		assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.IsType(t, tc.expectedErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_DeleteHandler(t *testing.T) {
	c, mocks := container.NewMockContainer(t)

	e := entity{
		name:       "user",
		entityType: nil,
		primaryKey: "id",
	}

	tests := []struct {
		desc         string
		id           string
		mockResp     driver.Result
		mockErr      error
		expectedErr  error
		expectedResp interface{}
	}{
		{"success case", "1", sqlmock.NewResult(1, 1), nil,
			nil, "user successfully deleted with id: 1"},
		{"SQL error case", "2", nil, errTest, errTest, nil},
		{"no rows affected", "3", sqlmock.NewResult(0, 0), nil,
			errEntityNotFound, nil},
	}

	for i, tc := range tests {
		ctx := createTestContext(http.MethodDelete, "/user", tc.id, nil, c)

		mocks.SQL.EXPECT().ExecContext(ctx, "DELETE FROM user WHERE id = ?", tc.id).Return(tc.mockResp, tc.mockErr)

		resp, err := e.Delete(ctx)

		assert.Equal(t, tc.expectedResp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.expectedErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
