package migrations

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/migration"
)

func TestCreateTableEmployee(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	// Create an instance of migration.Datasource using the mock database
	datasource := migration.Datasource{DB: db}

	// Test successful execution
	t.Run("TestSuccessfulExecution", func(t *testing.T) {
		mock.ExpectExec(createTable).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(employee_date).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("alter table employee add dob varchar(11) null;").WillReturnResult(sqlmock.NewResult(0, 1))

		err = createTableEmployee().UP(datasource)
		assert.NoError(t, err)
	})

	// Test error on create table
	t.Run("TestErrorOnCreateTable", func(t *testing.T) {
		mock.ExpectExec(createTable).WillReturnError(fmt.Errorf("create table error"))

		err = createTableEmployee().UP(datasource)
		assert.Error(t, err)
		assert.EqualError(t, err, "create table error")
	})

	// Test error on insert
	t.Run("TestErrorOnInsert", func(t *testing.T) {
		mock.ExpectExec(createTable).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(employee_date).WillReturnError(fmt.Errorf("insert error"))

		err = createTableEmployee().UP(datasource)
		assert.Error(t, err)
		assert.EqualError(t, err, "insert error")
	})

	// Test error on alter table
	t.Run("TestErrorOnAlterTable", func(t *testing.T) {
		mock.ExpectExec(createTable).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(employee_date).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("alter table employee add dob varchar(11) null;").WillReturnError(fmt.Errorf("alter table error"))

		err = createTableEmployee().UP(datasource)
		assert.Error(t, err)
		assert.EqualError(t, err, "alter table error")
	})
}
