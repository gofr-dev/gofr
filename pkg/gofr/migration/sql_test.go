package migration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
		require.NoError(t, err)

		i := 0

		for rows.Next() {
			require.NoError(t, rows.Err())
			err = rows.Scan(&id, &name)
			require.NoError(t, err)
			assert.Equal(t, expectedResult[i].id, id)
			assert.Equal(t, expectedResult[i].name, name)

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
		require.NoError(t, err)

		for rows.Next() {
			require.NoError(t, rows.Err())
			err = rows.Scan(&id, &name)
			require.Error(t, err)
			assert.Equal(t, expectedErr, err)
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
		require.NoError(t, err, "Error while scanning row")
		assert.Equal(t, 1, id, "expected id to be equal to 1")
		assert.Equal(t, "Alex", name, "expected name to be equal to 'Alex'")
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
		require.NoError(t, err, "Error while scanning row")
		assert.Equal(t, 1, id, "expected id to be equal to 1")
		assert.Equal(t, "Alex", name, "expected name to be equal to 'Alex'")
	})
}

func TestExec(t *testing.T) {
	t.Run("successful exec", func(t *testing.T) {
		mockContainer, mocks := container.NewMockContainer(t)

		expectedResult := mocks.SQL.NewResult(10, 1)

		mocks.SQL.ExpectExec("DELETE FROM users WHERE id = ?").WithArgs(1).WillReturnResult(expectedResult)
		sqlDB := mockContainer.SQL

		result, err := sqlDB.Exec("DELETE FROM users WHERE id = ?", 1)
		require.NoError(t, err)

		expectedLastInserted, err := expectedResult.LastInsertId()
		require.NoError(t, err)

		resultLastInserted, err := result.LastInsertId()
		require.NoError(t, err)

		expectedRowsAffected, err := expectedResult.RowsAffected()
		require.NoError(t, err)

		resultRowsAffected, err := result.RowsAffected()
		require.NoError(t, err)

		assert.Equal(t, expectedLastInserted, resultLastInserted)
		assert.Equal(t, expectedRowsAffected, resultRowsAffected)
	})

	t.Run("exec error", func(t *testing.T) {
		mockContainer, mocks := container.NewMockContainer(t)

		expectedErr := sql.ErrNoRows
		mocks.SQL.ExpectExec("UPDATE unknown_table SET name = ?").WithArgs("John").WillReturnError(expectedErr)
		sqlMockDB := mockContainer.SQL

		_, err := sqlMockDB.Exec("UPDATE unknown_table SET name = ?", "John")

		if err == nil {
			t.Errorf("Exec should return an error")
		}

		if !errors.Is(err, expectedErr) {
			t.Errorf("Exec should return the expected error, got: %v", err)
		}
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
		require.NoError(t, err)

		expectedLastInserted, err := expectedResult.LastInsertId()
		require.NoError(t, err)

		resultLastInserted, err := result.LastInsertId()
		require.NoError(t, err)

		expectedRowsAffected, err := expectedResult.RowsAffected()
		require.NoError(t, err)

		resultRowsAffected, err := result.RowsAffected()
		require.NoError(t, err)

		assert.Equal(t, expectedLastInserted, resultLastInserted)
		assert.Equal(t, expectedRowsAffected, resultRowsAffected)
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

	if err != nil {
		t.Errorf("checkAndCreateMigrationTable should return no error, got: %v", err)
	}
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

	if err == nil {
		t.Errorf("checkAndCreateMigrationTable should return an error")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("checkAndCreateMigrationTable should return the expected error, got: %v", err)
	}
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
	assert.NotNil(t, data.SQLTx.Tx)
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

	if data.SQLTx != nil {
		t.Errorf("beginTransaction should not return a transaction on DB error")
	}
}

func TestRollbackNoTransaction(t *testing.T) {
	mockContainer, _ := container.NewMockContainer(t)

	migrator := sqlMigrator{}
	migrator.rollback(mockContainer, transactionData{})
}
