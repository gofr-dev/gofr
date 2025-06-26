package migrations

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/migration"
)

func TestCreateTableEmployee(t *testing.T) {
	tests := []struct {
		name          string
		mockBehaviors func(mock sqlmock.Sqlmock)
		expectedError error
	}{
		{
			name: "SuccessfulExecution",
			mockBehaviors: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(createTable).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(employee_date).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("alter table employee add dob varchar(11) null;").WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedError: nil,
		},
		{
			name: "ErrorOnCreateTable",
			mockBehaviors: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(createTable).WillReturnError(fmt.Errorf("create table error"))
			},
			expectedError: fmt.Errorf("create table error"),
		},
		{
			name: "ErrorOnInsert",
			mockBehaviors: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(createTable).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(employee_date).WillReturnError(fmt.Errorf("insert error"))
			},
			expectedError: fmt.Errorf("insert error"),
		},
		{
			name: "ErrorOnAlterTable",
			mockBehaviors: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(createTable).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(employee_date).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("alter table employee add dob varchar(11) null;").WillReturnError(fmt.Errorf("alter table error"))
			},
			expectedError: fmt.Errorf("alter table error"),
		},
	}

	for i, tc := range tests {
		// Create mock database and datasource
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		datasource := migration.Datasource{SQL: db}

		// Set mock expectations
		tc.mockBehaviors(mock)

		// Execute the migration
		err = createTableEmployee().UP(datasource)

		assert.Equal(t, tc.expectedError, err, "TEST[%d] failed! Desc : %v", i, tc.name)

		require.NoError(t, mock.ExpectationsWereMet())
	}
}
