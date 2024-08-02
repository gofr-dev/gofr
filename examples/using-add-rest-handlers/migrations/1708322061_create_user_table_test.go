package migrations

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/migration"
)

func TestCreateTableUser(t *testing.T) {
	tests := []struct {
		desc          string
		mockBehaviors func(mock sqlmock.Sqlmock)
		expectedError error
	}{
		{"successful creation", func(mock sqlmock.Sqlmock) {
			mock.ExpectExec(createTable).WillReturnResult(sqlmock.NewResult(0, 1))
		}, nil},
		{"error on create table", func(mock sqlmock.Sqlmock) {
			mock.ExpectExec(createTable).WillReturnError(fmt.Errorf("create table error"))
		}, fmt.Errorf("create table error")},
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
		err = createTableUser().UP(datasource)

		assert.Equal(t, tc.expectedError, err, "TEST[%d] Failed.\n%s", i, tc.desc)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
