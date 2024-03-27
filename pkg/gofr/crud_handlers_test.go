package gofr

import (
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/sql"
	gofrHTTP "gofr.dev/pkg/gofr/http"

	"github.com/stretchr/testify/assert"
)

func Test_scanEntity(t *testing.T) {
	var invalidResource int

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
		{"invalid resource", &invalidResource, nil, errInvalidResource},
	}

	for i, tc := range tests {
		resp, err := scanEntity(tc.input)

		assert.Equal(t, tc.resp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_DeleteHandler(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		mockReturn driver.Result
		mockErr    error
		wantErr    error
		wantRes    interface{}
	}{
		{
			name:       "Success Case",
			id:         "1",
			mockReturn: sqlmock.NewResult(1, 1),
			mockErr:    nil,
			wantErr:    nil,
			wantRes:    "user successfully deleted with id: 1",
		},
		{
			name:       "SQL Error Case",
			id:         "2",
			mockReturn: nil,
			mockErr:    errTest,
			wantErr:    errTest,
			wantRes:    nil,
		},
		{
			name:       "No Rows Affected Case",
			id:         "3",
			mockReturn: sqlmock.NewResult(0, 0),
			mockErr:    nil,
			wantErr:    errEntityNotFound,
			wantRes:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReq := httptest.NewRequest(http.MethodDelete, "/user/"+tt.id, http.NoBody)
			testReq = mux.SetURLVars(testReq, map[string]string{"id": tt.id})
			gofrReq := gofrHTTP.NewRequest(testReq)
			cont := container.NewEmptyContainer()

			db, mock, mockMetrics := sql.NewMockSQLDB(t)
			defer db.Close()
			cont.SQL = db

			c := newContext(gofrHTTP.NewResponder(httptest.NewRecorder()), gofrReq, cont)

			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(), "type", "DELETE")

			mock.ExpectExec("DELETE FROM user WHERE id = ?").WithArgs(tt.id).
				WillReturnResult(tt.mockReturn).WillReturnError(tt.mockErr)

			e := entity{
				name:       "user",
				entityType: nil,
				primaryKey: "id",
			}

			res, err := e.Delete(c)

			assert.Equal(t, tt.wantRes, res)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
