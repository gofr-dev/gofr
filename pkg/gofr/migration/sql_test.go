package migration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
)

var (
	errCreateTable  = errors.New("create table error")
	errDuplicateKey = errors.New("duplicate key")
	errDB           = errors.New("db error")
	errUpdateFailed = errors.New("update failed")
	errSQLExec      = errors.New("exec error")
	errSQLCommit    = errors.New("commit error")
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
	ctx := t.Context()

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
	ctx := t.Context()

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
	mocks.SQL.ExpectExec(createSQLGoFrMigrationLocksTable).WillReturnResult(mocks.SQL.NewResult(1, 1))

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

func TestApply(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockContainer, _ := container.NewMockContainer(t)

	ds := &sqlDS{SQL: mockContainer.SQL}
	result := ds.apply(mockMigrator)

	sqlMig, ok := result.(sqlMigrator)
	require.True(t, ok, "Result should be an sqlMigrator")
	require.Equal(t, mockContainer.SQL, sqlMig.SQL, "SQL field should match")
	require.Equal(t, mockMigrator, sqlMig.migrator, "Migrator field should match")
}

func TestGetLastMigration_UseMigratorFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockContainer, mocks := container.NewMockContainer(t)

	mocks.SQL.ExpectQuery(getLastSQLGoFrMigration).
		WillReturnRows(mocks.SQL.NewRows([]string{"version"}).AddRow(2))

	mockMigrator.EXPECT().getLastMigration(mockContainer).Return(int64(5), nil)

	migrator := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	last, err := migrator.getLastMigration(mockContainer)
	require.NoError(t, err)
	require.Equal(t, int64(5), last, "Expected getLastMigration to return higher value from embedded migrator")
}

func TestGetLastMigration_MigratorReturnsLesser(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockContainer, mocks := container.NewMockContainer(t)

	mocks.SQL.ExpectQuery(getLastSQLGoFrMigration).
		WillReturnRows(mocks.SQL.NewRows([]string{"version"}).AddRow(7))

	mockMigrator.EXPECT().getLastMigration(mockContainer).Return(int64(5), nil)

	migrator := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	last, err := migrator.getLastMigration(mockContainer)
	require.NoError(t, err)
	require.Equal(t, int64(7), last, "Should return SQL migration value as it's higher")
}

func TestBeginTransaction_ReplaceSQLTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockContainer, mocks := container.NewMockContainer(t)

	mocks.SQL.ExpectBegin() // this returns a usable SQLTx

	mockMigrator.EXPECT().beginTransaction(mockContainer).Return(transactionData{
		MigrationNumber: 123,
	})

	migrator := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	data := migrator.beginTransaction(mockContainer)

	require.NotNil(t, data.SQLTx, "SQLTx should not be nil")
	require.Equal(t, int64(123), data.MigrationNumber, "Expected migration number from embedded migrator")
}

func TestCheckAndCreateMigrationTable_ErrorCreatingTable(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mocks.SQL.ExpectExec(createSQLGoFrMigrationsTable).WillReturnError(errCreateTable)

	m := sqlMigrator{}
	err := m.checkAndCreateMigrationTable(mockContainer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create table error")
}

func TestSQLMigrator_Lock(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	t.Run("LockSuccess", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(mocks.SQL.NewResult(0, 0))
		mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
			WithArgs(lockKey, "1", sqlmock.AnyArg()).
			WillReturnResult(mocks.SQL.NewResult(1, 1))

		mockMigrator.EXPECT().lock(ctx, gomock.Any(), mockContainer, "1").Return(nil)

		err := m.lock(ctx, cancel, mockContainer, "1")
		require.NoError(t, err)
	})
}

func TestSQLMigrator_Unlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	t.Run("UnlockSuccess", func(t *testing.T) {
		mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?").
			WithArgs(lockKey, "1").
			WillReturnResult(mocks.SQL.NewResult(0, 1))

		mockMigrator.EXPECT().unlock(mockContainer, "1").Return(nil)

		err := m.unlock(mockContainer, "1")
		require.NoError(t, err)
	})
}

func TestSQLMigrator_LockRetrySuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// First attempt: cleanup succeeds, but insert fails (lock held)
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(mocks.SQL.NewResult(0, 0))
	mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
		WithArgs(lockKey, "1", sqlmock.AnyArg()).
		WillReturnError(errDuplicateKey)

	// Second attempt succeeds
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(mocks.SQL.NewResult(0, 0))
	mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
		WithArgs(lockKey, "1", sqlmock.AnyArg()).
		WillReturnResult(mocks.SQL.NewResult(1, 1))

	mockMigrator.EXPECT().lock(ctx, gomock.Any(), mockContainer, "1").Return(nil)

	err := m.lock(ctx, cancel, mockContainer, "1")
	require.NoError(t, err)
}

func TestSQLMigrator_LockAcquireError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE expires_at < ?").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(mocks.SQL.NewResult(0, 0))
	mocks.SQL.ExpectExec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)").
		WithArgs(lockKey, "1", sqlmock.AnyArg()).
		WillReturnError(errDB)

	err := m.lock(ctx, cancel, mockContainer, "1")
	require.Error(t, err)
	assert.Equal(t, errLockAcquisitionFailed, err)
}

func TestSQLMigrator_StartRefreshSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockContainer, mocks := container.NewMockContainer(t)
	m := sqlMigrator{
		SQL:      mockContainer.SQL,
		migrator: NewMockmigrator(ctrl),
	}

	ctx, cancel := context.WithCancel(t.Context())

	// Expect at least one refresh within the defaultRefresh interval
	mocks.SQL.ExpectExec("UPDATE gofr_migration_locks SET expires_at = ? WHERE lock_key = ? AND owner_id = ?").
		WithArgs(sqlmock.AnyArg(), lockKey, "1").
		WillReturnResult(mocks.SQL.NewResult(0, 1))

	go m.startRefresh(ctx, cancel, mockContainer, "1")

	// Wait for at least one refresh cycle
	time.Sleep(defaultRefresh + 100*time.Millisecond)
	cancel()

	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)

	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Errorf("Unexpected context error: %v", ctx.Err())
		}
	default:
		t.Error("Expected context to be done")
	}

	// Verify all expectations were met (at least one refresh happened)
	if err := mocks.SQL.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestSQLMigrator_StartRefreshError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockContainer, mocks := container.NewMockContainer(t)
	m := sqlMigrator{
		SQL:      mockContainer.SQL,
		migrator: NewMockmigrator(ctrl),
	}

	ctx, cancel := context.WithCancel(t.Context())

	mocks.SQL.ExpectExec("UPDATE gofr_migration_locks SET expires_at = ? WHERE lock_key = ? AND owner_id = ?").
		WithArgs(sqlmock.AnyArg(), lockKey, "1").
		WillReturnError(errUpdateFailed)

	go m.startRefresh(ctx, cancel, mockContainer, "1")

	select {
	case <-ctx.Done():
		require.Error(t, ctx.Err())
	case <-time.After(defaultRefresh * 2):
		t.Error("Expected context to be canceled, but timed out")
	}
}

func TestSQLMigrator_CommitMigration(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	mocks.SQL.ExpectDialect().WillReturnString("mysql")

	mocks.SQL.ExpectBegin()
	tx, _ := mockContainer.SQL.Begin()

	data := transactionData{
		SQLTx:           tx,
		MigrationNumber: 1,
		StartTime:       time.Now().UTC(),
	}

	mocks.SQL.ExpectExec("INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);").
		WithArgs(int64(1), "UP", data.StartTime, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mocks.SQL.ExpectCommit()
	mockMigrator.EXPECT().commitMigration(mockContainer, data).Return(nil)

	err := m.commitMigration(mockContainer, data)
	assert.NoError(t, err)
}

func TestSQLMigrator_CommitMigration_Postgres(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	mocks.SQL.ExpectBegin()
	tx, _ := mockContainer.SQL.Begin()

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now().UTC(),
		SQLTx:           tx,
	}

	mocks.SQL.ExpectDialect().WillReturnString("postgres")
	mocks.SQL.ExpectExec("INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES ($1, $2, $3, $4);").
		WithArgs(int64(1), "UP", data.StartTime, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	mocks.SQL.ExpectCommit()
	mockMigrator.EXPECT().commitMigration(mockContainer, data).Return(nil)

	err := m.commitMigration(mockContainer, data)
	assert.NoError(t, err)
}

func TestSQLMigrator_CommitMigration_ExecError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	mocks.SQL.ExpectBegin()
	tx, _ := mockContainer.SQL.Begin()

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now().UTC(),
		SQLTx:           tx,
	}

	testErr := errSQLExec

	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);").
		WillReturnError(testErr)

	err := m.commitMigration(mockContainer, data)
	assert.Equal(t, testErr, err)
}

func TestSQLMigrator_CommitMigration_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	mocks.SQL.ExpectBegin()
	tx, _ := mockContainer.SQL.Begin()

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now().UTC(),
		SQLTx:           tx,
	}

	testErr := errSQLCommit

	mocks.SQL.ExpectDialect().WillReturnString("mysql")
	mocks.SQL.ExpectExec("INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mocks.SQL.ExpectCommit().WillReturnError(testErr)

	err := m.commitMigration(mockContainer, data)
	assert.Equal(t, testErr, err)
}

func TestSQLMigrator_RollbackSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	// Set mock logger to avoid os.Exit(1)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	mocks.SQL.ExpectBegin()
	tx, _ := mockContainer.SQL.Begin()

	data := transactionData{
		SQLTx: tx,
	}

	mocks.SQL.ExpectRollback()
	mockMigrator.EXPECT().rollback(mockContainer, data)

	// Fatalf is expected on rollback
	mockLogger.EXPECT().Fatalf(gomock.Any(), gomock.Any())

	assert.NotPanics(t, func() {
		m.rollback(mockContainer, data)
	})
}

func TestSQLMigrator_UnlockError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	testErr := errDB
	mocks.SQL.ExpectExec("DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?").
		WithArgs("gofr_migrations_lock", "owner-1").WillReturnError(testErr)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(1)

	err := m.unlock(mockContainer, "owner-1")
	assert.Equal(t, errLockReleaseFailed, err)
}

func TestSQLMigrator_CheckAndCreateMigrationTable_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := sqlMigrator{SQL: mockContainer.SQL, migrator: mockMigrator}

	createMigrations := `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`
	createLocks := `CREATE TABLE IF NOT EXISTS gofr_migration_locks (
    lock_key VARCHAR(64) PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL
);`

	testErr := errCreateTable
	mocks.SQL.ExpectExec(createMigrations).WillReturnError(testErr)

	err := m.checkAndCreateMigrationTable(mockContainer)
	assert.Equal(t, testErr, err)

	mocks.SQL.ExpectExec(createMigrations).WillReturnResult(sqlmock.NewResult(0, 0))
	mocks.SQL.ExpectExec(createLocks).WillReturnError(testErr)

	err = m.checkAndCreateMigrationTable(mockContainer)
	assert.Equal(t, testErr, err)
}

func TestSQLMigrator_Name(t *testing.T) {
	m := sqlMigrator{}
	assert.Equal(t, "SQL", m.name())
}
