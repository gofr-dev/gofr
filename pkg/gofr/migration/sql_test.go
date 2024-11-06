package migration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
)

func TestQuery(t *testing.T) {
	t.Run("successful query", func(t *testing.T) {
		var id int

		var name string

		expectedResult := []struct {
			id   int
			name string
		}{
			{1, "Alex"},
			{2, "John"},
		}

		mockContainer, mocks := container.NewMockContainer(t)

		expectedRows := mocks.SQL.NewRows([]string{"id", "name"}).
			AddRow(expectedResult[0].id, expectedResult[0].name).
			AddRow(expectedResult[1].id, expectedResult[1].name)

		mocks.SQL.ExpectQuery("SELECT * FROM users").WithoutArgs().WillReturnRows(expectedRows)

		rows, err := mockContainer.SQL.Query("SELECT * FROM users")
		require.NoError(t, err, "TestQuery : error executing mock query")

		i := 0

		for rows.Next() {
			require.NoError(t, rows.Err(), "TestQuery: row error")
			err = rows.Scan(&id, &name)
			require.NoError(t, err, "TestQuery: row scan error")
			require.Equal(t, expectedResult[i].id, id, "TestQuery: resultant ID & expected ID are not same")
			require.Equal(t, expectedResult[i].name, name, "TestQuery: resultant name & expected name are not same")

			i++
		}
	})

	t.Run("query error", func(t *testing.T) {
		var id int

		var name string

		mockContainer, mocks := container.NewMockContainer(t)
		expectedErr := sql.ErrNoRows
		expectedRows := mocks.SQL.NewRows([]string{"id", "name"})

		mocks.SQL.ExpectQuery("SELECT * FROM unknown_table").WithoutArgs().WillReturnRows(expectedRows)
		sqlMockDB := mockContainer.SQL

		rows, err := sqlMockDB.Query("SELECT * FROM unknown_table")
		require.NoError(t, err, "TestQuery : error executing mock query")

		for rows.Next() {
			require.NoError(t, rows.Err(), "TestQuery: row error")
			err = rows.Scan(&id, &name)
			require.Error(t, err, "TestQuery: row scan error")
			require.Equal(t, expectedErr, err, "TestQuery: expected error is not equal to resultant error")
		}
	})
}

func TestQueryRow(t *testing.T) {
	t.Run("successful query row", func(t *testing.T) {
		var name string

		var id int

		mockContainer, mocks := container.NewMockContainer(t)

		expectedRows := mocks.SQL.NewRows([]string{"id", "name"}).AddRow(1, "Alex")

		mocks.SQL.ExpectQuery("SELECT * FROM users WHERE id = ?").WithArgs(1).WillReturnRows(expectedRows)
		sqlMockDB := mockContainer.SQL

		err := sqlMockDB.QueryRow("SELECT * FROM users WHERE id = ?", 1).Scan(&id, &name)
		require.NoError(t, err, "TestQueryRow: row scan error")
		require.Equal(t, 1, id, "TestQueryRow: expected id to be equal to 1")
		require.Equal(t, "Alex", name, "TestQueryRow: expected name to be equal to 'Alex'")
	})
}

func TestQueryRowContext(t *testing.T) {
	ctx := context.Background()

	t.Run("successful query row context", func(t *testing.T) {
		var id int

		var name string

		mockContainer, mocks := container.NewMockContainer(t)

		expectedRows := mocks.SQL.NewRows([]string{"id", "name"}).AddRow(1, "Alex")

		mocks.SQL.ExpectQuery("SELECT * FROM users WHERE id = ?").WithArgs(1).WillReturnRows(expectedRows)
		sqlMockDB := mockContainer.SQL

		err := sqlMockDB.QueryRowContext(ctx, "SELECT * FROM users WHERE id = ?", 1).Scan(&id, &name)
		require.NoError(t, err, "TestQueryRowContext: Error while scanning row")
		require.Equal(t, 1, id, "TestQueryRowContext: expected id to be equal to 1")
		require.Equal(t, "Alex", name, "TestQueryRowContext: expected name to be equal to 'Alex'")
	})
}

func TestExec(t *testing.T) {
	t.Run("successful exec", func(t *testing.T) {
		mockContainer, mocks := container.NewMockContainer(t)

		expectedResult := mocks.SQL.NewResult(10, 1)

		mocks.SQL.ExpectExec("DELETE FROM users WHERE id = ?").WithArgs(1).WillReturnResult(expectedResult)
		sqlDB := mockContainer.SQL

		result, err := sqlDB.Exec("DELETE FROM users WHERE id = ?", 1)
		require.NoError(t, err, "TestExec: error while executing mock query")

		expectedLastInserted, err := expectedResult.LastInsertId()
		require.NoError(t, err, "TestExec: error while retrieving last inserted id from expected sqlresult")

		resultLastInserted, err := result.LastInsertId()
		require.NoError(t, err, "TestExec: error while retrieving last inserted id from mock sqlresult")

		expectedRowsAffected, err := expectedResult.RowsAffected()
		require.NoError(t, err, "TestExec: error while retrieving rows affected from expected sqlresult")

		resultRowsAffected, err := result.RowsAffected()
		require.NoError(t, err, "TestExec: error while retrieving rows affected from mock sqlresult")

		require.Equal(t, expectedLastInserted, resultLastInserted, "TestExec: expected last inserted id to be equal to 10")
		require.Equal(t, expectedRowsAffected, resultRowsAffected, "TestExec: expected rows affected to be equal to 1")
	})

	t.Run("exec error", func(t *testing.T) {
		mockContainer, mocks := container.NewMockContainer(t)

		expectedErr := sql.ErrNoRows
		mocks.SQL.ExpectExec("UPDATE unknown_table SET name = ?").WithArgs("John").WillReturnError(expectedErr)
		sqlMockDB := mockContainer.SQL

		_, err := sqlMockDB.Exec("UPDATE unknown_table SET name = ?", "John")
		require.Error(t, err, "TestExec: expected error while executing mock query")
		require.Equal(t, expectedErr, err, "TestExec: Exec should return the expected error, got: %v", err)
	})
}

func TestExecContext(t *testing.T) {
	ctx := context.Background()

	t.Run("successful exec context", func(t *testing.T) {
		mockContainer, mocks := container.NewMockContainer(t)
		expectedResult := mocks.SQL.NewResult(10, 1)
		mocks.SQL.ExpectExec("DELETE FROM users WHERE id = ?").WithArgs(1).WillReturnResult(expectedResult)
		sqlMockDB := mockContainer.SQL

		result, err := sqlMockDB.ExecContext(ctx, "DELETE FROM users WHERE id = ?", 1)
		require.NoError(t, err, "TestExecContext: error while executing mock query")

		expectedLastInserted, err := expectedResult.LastInsertId()
		require.NoError(t, err, "TestExecContext: error while retrieving last inserted id from expected sqlresult")

		resultLastInserted, err := result.LastInsertId()
		require.NoError(t, err, "TestExecContext: error while retrieving last inserted id from mock sqlresult")

		expectedRowsAffected, err := expectedResult.RowsAffected()
		require.NoError(t, err, "TestExecContext: error while retrieving rows affected from expected sqlresult")

		resultRowsAffected, err := result.RowsAffected()
		require.NoError(t, err, "TestExecContext: error while retrieving rows affected from mock sqlresult")

		require.Equal(t, expectedLastInserted, resultLastInserted, "TestExecContext: expected last inserted id to be equal to 10")
		require.Equal(t, expectedRowsAffected, resultRowsAffected, "TestExecContext: expected rows affected to be equal to 1")
	})
}

func TestCheckAndCreateMigrationTableSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockContainer, mocks := container.NewMockContainer(t)

	mockMigrator.EXPECT().checkAndCreateMigrationTable(mockContainer)
	mocks.SQL.ExpectExec(createSQLGoFrMigrationsTable).WillReturnResult(mocks.SQL.NewResult(1, 1))

	migrator := sqlMigrator{
		SQL:      mockContainer.SQL,
		migrator: mockMigrator,
	}

	err := migrator.checkAndCreateMigrationTable(mockContainer)
	require.NoError(t, err, "TestCheckAndCreateMigrationTable: error while executing mock query")
}

func TestCheckAndCreateMigrationTableExecError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockContainer, mocks := container.NewMockContainer(t)
	expectedErr := sql.ErrNoRows

	mocks.SQL.ExpectExec(createSQLGoFrMigrationsTable).WillReturnError(expectedErr)

	migrator := sqlMigrator{
		SQL:      mockContainer.SQL,
		migrator: mockMigrator,
	}

	err := migrator.checkAndCreateMigrationTable(mockContainer)
	require.Error(t, err, "TestCheckAndCreateMigrationTable: expected an error while executing mock query")
	require.Equal(t, expectedErr, err, "TestCheckAndCreateMigrationTable: resultant error is not eual to expected error")
}

func TestBeginTransactionSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockContainer, mocks := container.NewMockContainer(t)

	mocks.SQL.ExpectBegin()
	mockMigrator.EXPECT().beginTransaction(mockContainer)

	migrator := sqlMigrator{
		SQL:      mockContainer.SQL,
		migrator: mockMigrator,
	}

	data := migrator.beginTransaction(mockContainer)
	require.NotNil(t, data.SQLTx.Tx, "TestBeginTransaction: SQLTX.tx should not be nil")
}

var (
	errBeginTx = errors.New("failed to begin transaction")
)

func TestBeginTransactionDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockContainer, mocks := container.NewMockContainer(t)

	mocks.SQL.ExpectBegin().WillReturnError(errBeginTx)

	migrator := sqlMigrator{
		SQL:      mockContainer.SQL,
		migrator: mockMigrator,
	}

	data := migrator.beginTransaction(mockContainer)
	require.Nil(t, data.SQLTx, "TestBeginTransaction: beginTransaction should not return a transaction on DB error")
}

func TestRollbackNoTransaction(t *testing.T) {
	mockContainer, _ := container.NewMockContainer(t)

	migrator := sqlMigrator{}
	migrator.rollback(mockContainer, transactionData{})
}
