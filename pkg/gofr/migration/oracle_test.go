package migration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

var (
	errOracleDuplicateKey = errors.New("unique constraint violated")
	errOracleSystemError  = errors.New("ORA-01017: invalid username/password")
	errOracleLockLost     = errors.New("ORA-20001: lock refresh failed: no rows updated")
	errOracleLockStolen   = errors.New("ORA-20002: lock release failed: lock was already released or stolen")
)

func oracleSetup(t *testing.T) (migrator, *container.MockOracleDB, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	mockOracle := mocks.Oracle

	ds := Datasource{Oracle: mockOracle}

	oracleDB := oracleDS{Oracle: mockOracle}
	migrationWithOracle := oracleDB.apply(&ds)

	mockContainer.Oracle = mockOracle

	return migrationWithOracle, mockOracle, mockContainer
}

func Test_OracleCheckAndCreateMigrationTable(t *testing.T) {
	testCases := []struct {
		desc         string
		migTableErr  error
		lockTableErr error
		expectedErr  error
	}{
		{"no error", nil, nil, nil},
		{"migration table creation failed", sql.ErrConnDone, nil, sql.ErrConnDone},
		{"lock table creation failed", nil, sql.ErrConnDone, sql.ErrConnDone},
	}

	for i, tc := range testCases {
		mg, mockOracle, mockContainer := oracleSetup(t)

		mockOracle.EXPECT().Exec(gomock.Any(), checkAndCreateOracleMigrationTable).Return(tc.migTableErr)

		if tc.migTableErr == nil {
			mockOracle.EXPECT().Exec(gomock.Any(), checkAndCreateOracleMigrationLocksTable).Return(tc.lockTableErr)
		}

		err := mg.checkAndCreateMigrationTable(mockContainer)
		assert.Equal(t, tc.expectedErr, err, "TEST[%d]: %s failed", i, tc.desc)
	}
}

func Test_OracleGetLastMigration(t *testing.T) {
	mg, mockOracle, mockContainer := oracleSetup(t)

	testCases := []struct {
		desc string
		err  error
		resp int64
	}{
		{"no error", nil, 0},
		{"connection failed", sql.ErrConnDone, -1},
	}

	for i, tc := range testCases {
		mockOracle.EXPECT().Select(gomock.Any(), gomock.Any(), getLastOracleGoFrMigration).Return(tc.err)

		resp, err := mg.getLastMigration(mockContainer)
		assert.Equal(t, tc.resp, resp, "TEST[%d]: %s failed", i, tc.desc)

		if tc.err != nil {
			assert.ErrorContains(t, err, tc.err.Error(), "TEST[%d]: %s failed", i, tc.desc)
		} else {
			assert.NoError(t, err, "TEST[%d]: %s failed", i, tc.desc)
		}
	}
}

func Test_OracleCommitMigration(t *testing.T) {
	mg, _, mockContainer := oracleSetup(t)
	ctrl := gomock.NewController(t)
	timeNow := time.Now()

	// Success case
	mockTxSuccess := container.NewMockOracleTx(ctrl)
	tdSuccess := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
		OracleTx:        mockTxSuccess,
		UsedDatasources: map[string]bool{dsOracle: true},
	}

	mockTxSuccess.EXPECT().
		ExecContext(gomock.Any(), insertOracleGoFrMigrationRow,
			tdSuccess.MigrationNumber, "UP", tdSuccess.StartTime, gomock.Any()).
		Return(nil)

	mockTxSuccess.EXPECT().Commit().Return(nil)

	err := mg.commitMigration(mockContainer, tdSuccess)
	require.NoError(t, err, "Success case failed")

	// Error case
	mockTxError := container.NewMockOracleTx(ctrl)
	tdError := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
		OracleTx:        mockTxError,
		UsedDatasources: map[string]bool{dsOracle: true},
	}

	mockTxError.EXPECT().
		ExecContext(gomock.Any(), insertOracleGoFrMigrationRow,
			tdError.MigrationNumber, "UP", tdError.StartTime, gomock.Any()).
		Return(sql.ErrConnDone)

	mockTxError.EXPECT().Rollback().Return(nil).AnyTimes()

	err = mg.commitMigration(mockContainer, tdError)
	assert.Equal(t, sql.ErrConnDone, err, "Error case failed")
}

func TestOracleMigration_RunMigrationSuccess(t *testing.T) {
	mockOracle, mockContainer := initializeOracleRunMocks(t)
	ctrl := gomock.NewController(t)

	mockTx := container.NewMockOracleTx(ctrl)

	migrationMap := map[int64]Migrate{
		1: {UP: func(d Datasource) error {
			return d.Oracle.Exec(context.Background(), "CREATE TABLE test (id INT)")
		}},
	}

	// 1. Create migration table
	mockOracle.EXPECT().Exec(gomock.Any(), checkAndCreateOracleMigrationTable).Return(nil)
	// 2. Create migration lock table
	mockOracle.EXPECT().Exec(gomock.Any(), checkAndCreateOracleMigrationLocksTable).Return(nil)

	// 3. Optimistic pre-check + re-fetch under lock: get last migration
	mockOracle.EXPECT().Select(gomock.Any(), gomock.Any(), getLastOracleGoFrMigration).
		DoAndReturn(func(_ context.Context, dest any, _ string, _ ...any) error {
			results := dest.(*[]map[string]any)
			*results = []map[string]any{
				{"LAST_MIGRATION": int64(0)},
			}
			return nil
		}).Times(2)

	// 4. Acquire lock: clean up expired rows
	mockOracle.EXPECT().Exec(gomock.Any(), deleteExpiredOracleLocks, gomock.Any()).Return(nil)
	// 5. Acquire lock: insert lock row
	mockOracle.EXPECT().Exec(gomock.Any(), insertOracleLock, lockKey, gomock.Any(), gomock.Any()).Return(nil)

	// 6. Begin transaction
	mockOracle.EXPECT().Begin().Return(mockTx, nil)

	// 7. Execute migration via transaction wrapper
	mockTx.EXPECT().ExecContext(gomock.Any(), "CREATE TABLE test (id INT)").Return(nil)

	// 8. Insert migration record
	mockTx.EXPECT().ExecContext(gomock.Any(), insertOracleGoFrMigrationRow,
		int64(1), "UP", gomock.Any(), gomock.Any()).Return(nil)

	// 9. Commit transaction
	mockTx.EXPECT().Commit().Return(nil)

	// 10. Unlock: delete lock row
	mockOracle.EXPECT().Exec(gomock.Any(), deleteOracleLock, lockKey, gomock.Any()).Return(nil)

	Run(migrationMap, mockContainer)
}

func TestOracleMigration_FailCreateMigrationTable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockContainer, mocks := container.NewMockContainer(t)
	mockOracle := mocks.Oracle
	mockContainer.Oracle = mockOracle

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	mg := od.apply(&ds)

	mockOracle.EXPECT().Exec(gomock.Any(), checkAndCreateOracleMigrationTable).Return(sql.ErrConnDone)

	err := mg.checkAndCreateMigrationTable(mockContainer)
	assert.Equal(t, sql.ErrConnDone, err)
}

func TestOracleMigration_GetLastMigration_ReturnsZeroOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockContainer, mocks := container.NewMockContainer(t)
	mockOracle := mocks.Oracle
	mockContainer.Oracle = mockOracle

	ds := Datasource{Oracle: mockOracle}
	od := oracleDS{Oracle: mockOracle}
	mg := od.apply(&ds)

	mockOracle.EXPECT().Select(gomock.Any(), gomock.Any(), getLastOracleGoFrMigration).Return(sql.ErrConnDone)

	lastMigration, err := mg.getLastMigration(mockContainer)
	assert.Equal(t, int64(-1), lastMigration)
	assert.ErrorContains(t, err, sql.ErrConnDone.Error())
}

func TestOracleMigrator_Lock(t *testing.T) {
	t.Run("LockSuccess", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		ctx, cancel := context.WithCancel(t.Context())

		// Cleanup expired locks
		mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteExpiredOracleLocks, gomock.Any()).Return(nil)
		// Insert lock succeeds
		mocks.Oracle.EXPECT().Exec(gomock.Any(), insertOracleLock, lockKey, "owner-1", gomock.Any()).Return(nil)
		// Chain to inner migrator
		mockMigrator.EXPECT().lock(ctx, gomock.Any(), mockContainer, "owner-1").Return(nil)

		err := m.lock(ctx, cancel, mockContainer, "owner-1")
		require.NoError(t, err)
	})

	t.Run("LockRetryThenSuccess", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		ctx, cancel := context.WithCancel(t.Context())

		// First attempt: cleanup OK, insert fails with duplicate key
		mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteExpiredOracleLocks, gomock.Any()).Return(nil)
		mocks.Oracle.EXPECT().Exec(gomock.Any(), insertOracleLock, lockKey, "owner-1", gomock.Any()).
			Return(errOracleDuplicateKey)

		// Second attempt: cleanup OK, insert succeeds
		mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteExpiredOracleLocks, gomock.Any()).Return(nil)
		mocks.Oracle.EXPECT().Exec(gomock.Any(), insertOracleLock, lockKey, "owner-1", gomock.Any()).Return(nil)
		mockMigrator.EXPECT().lock(ctx, gomock.Any(), mockContainer, "owner-1").Return(nil)

		err := m.lock(ctx, cancel, mockContainer, "owner-1")
		require.NoError(t, err)
	})

	t.Run("LockAcquireSystemError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteExpiredOracleLocks, gomock.Any()).Return(nil)
		mocks.Oracle.EXPECT().Exec(gomock.Any(), insertOracleLock, lockKey, "owner-1", gomock.Any()).
			Return(errOracleSystemError)

		err := m.lock(ctx, cancel, mockContainer, "owner-1")
		assert.Equal(t, errLockAcquisitionFailed, err)
	})

	t.Run("LockContextCancelledWhileRetrying", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		ctx, cancel := context.WithCancel(t.Context())

		// Lock is held; insert fails with duplicate key
		mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteExpiredOracleLocks, gomock.Any()).Return(nil)
		mocks.Oracle.EXPECT().Exec(gomock.Any(), insertOracleLock, lockKey, "owner-1", gomock.Any()).
			Return(errOracleDuplicateKey)

		// Cancel the context while the lock is retrying
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		err := m.lock(ctx, cancel, mockContainer, "owner-1")
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestOracleMigrator_Unlock(t *testing.T) {
	t.Run("UnlockSuccess", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteOracleLock, lockKey, "owner-1").Return(nil)
		mockMigrator.EXPECT().unlock(mockContainer, "owner-1").Return(nil)

		err := m.unlock(mockContainer, "owner-1")
		require.NoError(t, err)
	})

	t.Run("UnlockError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteOracleLock, lockKey, "owner-1").
			Return(sql.ErrConnDone)

		err := m.unlock(mockContainer, "owner-1")
		assert.Equal(t, errLockReleaseFailed, err)
	})

	t.Run("UnlockLockStolen", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		// PL/SQL raises error when 0 rows deleted (lock was stolen)
		mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteOracleLock, lockKey, "owner-1").
			Return(errOracleLockStolen)

		err := m.unlock(mockContainer, "owner-1")
		assert.Equal(t, errLockReleaseFailed, err)
	})
}

func TestOracleMigrator_StartRefresh(t *testing.T) {
	t.Run("RefreshSuccess", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		ctx, cancel := context.WithCancel(t.Context())

		// Expect at least one refresh tick within the test window
		mocks.Oracle.EXPECT().
			Exec(gomock.Any(), updateOracleLock, gomock.Any(), lockKey, "owner-1").
			Return(nil).MinTimes(1)

		go m.startRefresh(ctx, cancel, mockContainer, "owner-1")

		time.Sleep(defaultRefresh + 100*time.Millisecond)
		cancel()

		// Allow the goroutine to exit
		time.Sleep(50 * time.Millisecond)

		select {
		case <-ctx.Done():
			require.ErrorIs(t, ctx.Err(), context.Canceled)
		default:
			t.Error("expected context to be done after cancel")
		}
	})

	t.Run("RefreshError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		ctx, cancel := context.WithCancel(t.Context())

		mocks.Oracle.EXPECT().
			Exec(gomock.Any(), updateOracleLock, gomock.Any(), lockKey, "owner-1").
			Return(sql.ErrConnDone).Times(1)

		go m.startRefresh(ctx, cancel, mockContainer, "owner-1")

		select {
		case <-ctx.Done():
			require.Error(t, ctx.Err())
		case <-time.After(defaultRefresh * 2):
			t.Error("expected context to be canceled after refresh error, but timed out")
		}
	})

	t.Run("RefreshLockLost", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, mocks := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
		mockContainer.Oracle = mocks.Oracle

		ctx, cancel := context.WithCancel(t.Context())

		// PL/SQL raises error when 0 rows updated (lock was stolen)
		mocks.Oracle.EXPECT().
			Exec(gomock.Any(), updateOracleLock, gomock.Any(), lockKey, "owner-1").
			Return(errOracleLockLost).Times(1)

		go m.startRefresh(ctx, cancel, mockContainer, "owner-1")

		select {
		case <-ctx.Done():
			require.Error(t, ctx.Err())
		case <-time.After(defaultRefresh * 2):
			t.Error("expected context to be canceled after lock loss, but timed out")
		}
	})
}

func TestOracleMigrator_Name(t *testing.T) {
	m := oracleMigrator{}
	assert.Equal(t, "Oracle", m.name())
}

func TestOracleMigrator_LockWithCleanupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := oracleMigrator{Oracle: mocks.Oracle, migrator: mockMigrator}
	mockContainer.Oracle = mocks.Oracle

	ctx, cancel := context.WithCancel(t.Context())

	// Cleanup fails but should not block lock acquisition
	mocks.Oracle.EXPECT().Exec(gomock.Any(), deleteExpiredOracleLocks, gomock.Any()).Return(sql.ErrConnDone)
	// Insert succeeds
	mocks.Oracle.EXPECT().Exec(gomock.Any(), insertOracleLock, lockKey, "owner-1", gomock.Any()).Return(nil)
	mockMigrator.EXPECT().lock(ctx, gomock.Any(), mockContainer, "owner-1").Return(nil)

	err := m.lock(ctx, cancel, mockContainer, "owner-1")
	require.NoError(t, err)
}

func TestOracleMigrator_Rollback(t *testing.T) {
	t.Run("NilTransaction", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, _ := container.NewMockContainer(t)
		mockMigrator := NewMockmigrator(ctrl)
		m := oracleMigrator{migrator: mockMigrator}

		data := transactionData{MigrationNumber: 1}

		mockMigrator.EXPECT().rollback(mockContainer, data)

		m.rollback(mockContainer, data)
	})

	t.Run("RollbackSuccess", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, _ := container.NewMockContainer(t)
		mockLogger := container.NewMockLogger(ctrl)
		mockContainer.Logger = mockLogger
		mockMigrator := NewMockmigrator(ctrl)
		mockTx := container.NewMockOracleTx(ctrl)
		m := oracleMigrator{migrator: mockMigrator}

		data := transactionData{MigrationNumber: 1, OracleTx: mockTx}

		mockTx.EXPECT().Rollback().Return(nil)
		mockLogger.EXPECT().Fatalf(gomock.Any())
		mockMigrator.EXPECT().rollback(mockContainer, data)

		m.rollback(mockContainer, data)
	})

	t.Run("RollbackError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockContainer, _ := container.NewMockContainer(t)
		mockLogger := container.NewMockLogger(ctrl)
		mockContainer.Logger = mockLogger
		mockMigrator := NewMockmigrator(ctrl)
		mockTx := container.NewMockOracleTx(ctrl)
		m := oracleMigrator{migrator: mockMigrator}

		data := transactionData{MigrationNumber: 1, OracleTx: mockTx}

		mockTx.EXPECT().Rollback().Return(sql.ErrConnDone)
		mockLogger.EXPECT().Fatalf(gomock.Any(), gomock.Any())
		mockMigrator.EXPECT().rollback(mockContainer, data)

		m.rollback(mockContainer, data)
	})
}

func TestOracleMigrator_ConvertToInt64(t *testing.T) {
	m := oracleMigrator{}

	tests := []struct {
		desc     string
		value    any
		expected int64
	}{
		{"float64", float64(42.7), 42},
		{"int64", int64(99), 99},
		{"int", int(7), 7},
		{"parseable string", "123", 123},
		{"negative string", "-5", -5},
		{"unparsable string", "not-a-number", 0},
		{"empty string", "", 0},
		{"nil (becomes <nil>)", nil, 0},
	}

	for i, tc := range tests {
		result := m.convertToInt64(tc.value)
		assert.Equal(t, tc.expected, result, "TEST[%d]: %s", i, tc.desc)
	}
}

func TestOracleMigrator_ParseStringValue(t *testing.T) {
	m := oracleMigrator{}

	tests := []struct {
		desc     string
		value    any
		expected int64
	}{
		{"valid integer string", "42", 42},
		{"empty string", "", 0},
		{"nil-representation", "<nil>", 0},
		{"nil value", nil, 0},
		{"invalid string", "not-a-number", 0},
		{"negative string", "-10", -10},
	}

	for i, tc := range tests {
		result := m.parseStringValue(tc.value)
		assert.Equal(t, tc.expected, result, "TEST[%d]: %s", i, tc.desc)
	}
}

func TestOracleTransactionWrapper_Select(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockTx := container.NewMockOracleTx(ctrl)
	wrapper := &oracleTransactionWrapper{tx: mockTx}

	t.Run("SelectSuccess", func(t *testing.T) {
		var dest []map[string]any

		mockTx.EXPECT().SelectContext(gomock.Any(), &dest, "SELECT 1", gomock.Any()).Return(nil)

		err := wrapper.Select(t.Context(), &dest, "SELECT 1")
		require.NoError(t, err)
	})

	t.Run("SelectError", func(t *testing.T) {
		var dest []map[string]any

		mockTx.EXPECT().SelectContext(gomock.Any(), &dest, "SELECT 1", gomock.Any()).Return(sql.ErrConnDone)

		err := wrapper.Select(t.Context(), &dest, "SELECT 1")
		assert.Equal(t, sql.ErrConnDone, err)
	})
}

func TestOracleTransactionWrapper_Begin(t *testing.T) {
	wrapper := &oracleTransactionWrapper{}

	tx, err := wrapper.Begin()
	assert.Nil(t, tx)
	assert.Equal(t, errNestedTransactionNotSupported, err)
}

func initializeOracleRunMocks(t *testing.T) (*container.MockOracleDB, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)
	mockOracle := mocks.Oracle

	// Disable all other datasources by setting to nil.
	mockContainer.SQL = nil
	mockContainer.Redis = nil
	mockContainer.Mongo = nil
	mockContainer.Cassandra = nil
	mockContainer.PubSub = nil
	mockContainer.ArangoDB = nil
	mockContainer.SurrealDB = nil
	mockContainer.DGraph = nil
	mockContainer.Elasticsearch = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil
	mockContainer.Clickhouse = nil

	// Initialize Oracle mock and Logger.
	mockContainer.Oracle = mockOracle
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)

	return mockOracle, mockContainer
}

func TestOracleMigrator_CommitMigration_SkipsWhenNotUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockContainer, _ := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockOracle := container.NewMockOracleDB(ctrl)

	m := oracleMigrator{Oracle: mockOracle, migrator: mockMigrator}

	mockTx := container.NewMockOracleTx(ctrl)

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now().UTC(),
		OracleTx:        mockTx,
		UsedDatasources: map[string]bool{},
	}

	// Should NOT expect ExecContext for INSERT.
	// Should expect Commit (empty transaction).
	mockTx.EXPECT().Commit().Return(nil)
	mockMigrator.EXPECT().commitMigration(mockContainer, data).Return(nil)

	err := m.commitMigration(mockContainer, data)
	assert.NoError(t, err)
}
