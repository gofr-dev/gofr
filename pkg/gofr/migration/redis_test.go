package migration

import (
	"context"
	"errors"
	"testing"
	"time"

	goRedis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
)

var (
	errRefreshFailed = errors.New("refresh failed")
	errRedis         = errors.New("redis error")
	errHSet          = errors.New("hset error")
	errRedisExec     = errors.New("exec error")
	errEval          = errors.New("eval error")
)

func TestRedis_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCmd := NewMockRedis(ctrl)
	mockCmd.EXPECT().Get(t.Context(), "test_key").Return(&goRedis.StringCmd{})

	r := redisDS{mockCmd}
	_, err := r.Get(t.Context(), "test_key").Result()

	require.NoError(t, err, "TEST Failed.\n")
}

func TestRedis_Set(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCmd := NewMockRedis(ctrl)
	mockCmd.EXPECT().Set(t.Context(), "test_key", "test_value", time.Duration(0)).Return(&goRedis.StatusCmd{})

	r := redisDS{mockCmd}
	_, err := r.Set(t.Context(), "test_key", "test_value", 0).Result()

	require.NoError(t, err, "TEST Failed.\n")
}

func TestRedis_Del(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCmd := NewMockRedis(ctrl)
	mockCmd.EXPECT().Del(t.Context(), "test_key").Return(&goRedis.IntCmd{})

	r := redisDS{mockCmd}
	_, err := r.Del(t.Context(), "test_key").Result()

	require.NoError(t, err, "TEST Failed.\n")
}

func TestRedis_Rename(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCmd := NewMockRedis(ctrl)
	mockCmd.EXPECT().Rename(t.Context(), "test_key", "test_new_key").Return(&goRedis.StatusCmd{})

	r := redisDS{mockCmd}
	_, err := r.Rename(t.Context(), "test_key", "test_new_key").Result()

	require.NoError(t, err, "TEST Failed.\n")
}

func TestRedisMigrator_GetLastMigration(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)

	m := redisMigrator{
		Redis:    mocks.Redis,
		migrator: mockMigrator,
	}

	tests := []struct {
		desc                  string
		mockedData            map[string]string
		redisErr              error
		unmarshalErr          error
		migratorLastMigration int64
		expectedLastMigration int64
	}{
		{
			desc: "Successful",
			mockedData: map[string]string{
				"1": `{"method":"UP","startTime":"2024-01-01T00:00:00Z","duration":1000}`,
				"2": `{"method":"UP","startTime":"2024-01-02T00:00:00Z","duration":2000}`,
			},
			migratorLastMigration: 3,
			expectedLastMigration: 3,
		},
		{
			desc:                  "ErrorFromHGetAll",
			redisErr:              goRedis.ErrClosed,
			expectedLastMigration: 0,
		},
		{
			desc: "UnmarshalError",
			mockedData: map[string]string{
				"1": `{"method":"UP","startTime":"2024-01-01T00:00:00Z","duration":1000}`,
				"2": "invalid JSON data",
			},
			expectedLastMigration: 0,
		},
		{
			desc: "lm2IsLessThanLastMigration",
			mockedData: map[string]string{
				"1": `{"method":"UP","startTime":"2024-01-01T00:00:00Z","duration":1000}`,
				"2": `{"method":"UP","startTime":"2024-01-02T00:00:00Z","duration":2000}`,
			},
			migratorLastMigration: 1,
			expectedLastMigration: 3,
		},
	}

	for i, tc := range tests {
		mocks.Redis.EXPECT().HGetAll(gomock.Any(), "gofr_migrations").Return(
			goRedis.NewMapStringStringResult(tc.mockedData, tc.redisErr))

		mockMigrator.EXPECT().getLastMigration(gomock.Any()).Return(tc.migratorLastMigration).MaxTimes(2)

		lastMigration := m.getLastMigration(c)

		assert.Equal(t, tc.expectedLastMigration, lastMigration, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestRedisMigrator_beginTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockPipeliner := NewMockPipeliner(ctrl)

	m := redisMigrator{
		Redis:    mocks.Redis,
		migrator: mockMigrator,
	}

	mocks.Redis.EXPECT().TxPipeline().Return(mockPipeliner)
	mockMigrator.EXPECT().beginTransaction(c).Return(transactionData{})

	data := m.beginTransaction(c)

	assert.Equal(t, mockPipeliner, data.RedisTx)
}

func TestRedisMigrator_StartRefreshSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	ctx, cancel := context.WithCancel(t.Context())

	// The refresh happens every defaultRefresh interval (5 seconds)
	// We expect at least one call within our test window
	mocks.Redis.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{lockKey}, "1", int(defaultLockTTL.Seconds())).
		Return(goRedis.NewCmdResult(int64(1), nil)).MinTimes(1).MaxTimes(2)

	go m.startRefresh(ctx, cancel, c, "1")

	// Wait enough time for at least one refresh cycle
	time.Sleep(defaultRefresh + 100*time.Millisecond)
	cancel()

	// Give goroutine time to exit gracefully
	time.Sleep(50 * time.Millisecond)

	select {
	case <-ctx.Done():
		// Check if it was canceled by us (success) or something else
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Errorf("Unexpected context error: %v", ctx.Err())
		}
	default:
		t.Error("Expected context to be done")
	}
}

func TestRedisMigrator_StartRefreshError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	ctx, cancel := context.WithCancel(t.Context())

	mocks.Redis.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{lockKey}, "1", int(defaultLockTTL.Seconds())).
		Return(goRedis.NewCmdResult(int64(0), errRefreshFailed)).Times(1)

	go m.startRefresh(ctx, cancel, c, "1")

	select {
	case <-ctx.Done():
		// In this version, cancel() doesn't pass the error, but it does cancel the context.
		require.Error(t, ctx.Err())
	case <-time.After(defaultRefresh * 2):
		t.Error("Expected context to be canceled, but timed out")
	}
}

func TestRedisMigrator_StartRefreshLockLost(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	ctx, cancel := context.WithCancel(t.Context())

	// Lock returns 0, indicating lock was lost
	mocks.Redis.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{lockKey}, "1", int(defaultLockTTL.Seconds())).
		Return(goRedis.NewCmdResult(int64(0), nil)).Times(1)

	go m.startRefresh(ctx, cancel, c, "1")

	select {
	case <-ctx.Done():
		require.Error(t, ctx.Err())
	case <-time.After(defaultRefresh * 2):
		t.Error("Expected context to be canceled, but timed out")
	}
}

func TestRedisMigrator_Lock(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	ctx, cancel := context.WithCancel(t.Context())

	// Test Success
	mocks.Redis.EXPECT().SetNX(gomock.Any(), lockKey, "owner-1", defaultLockTTL).Return(goRedis.NewBoolResult(true, nil))
	mockMigrator.EXPECT().lock(ctx, gomock.Any(), gomock.Any(), "owner-1").Return(nil)

	err := m.lock(ctx, cancel, c, "owner-1")

	require.NoError(t, err)

	// Test Error
	mocks.Redis.EXPECT().SetNX(gomock.Any(), lockKey, "owner-1", defaultLockTTL).Return(goRedis.NewBoolResult(false, errRedis))

	err = m.lock(ctx, cancel, c, "owner-1")
	assert.Equal(t, errLockAcquisitionFailed, err)
}

func TestRedisMigrator_Unlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	mocks.Redis.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{lockKey}, "owner-1").Return(goRedis.NewCmdResult(int64(1), nil))
	mockMigrator.EXPECT().unlock(gomock.Any(), "owner-1").Return(nil)

	err := m.unlock(c, "owner-1")
	assert.NoError(t, err)
}

func TestRedisMigrator_CommitMigration(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockPipeliner := NewMockPipeliner(ctrl)

	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now().Add(-1 * time.Second),
		RedisTx:         mockPipeliner,
	}

	// Mock HSet and Exec
	mockPipeliner.EXPECT().HSet(gomock.Any(), "gofr_migrations", gomock.Any()).Return(goRedis.NewIntResult(1, nil))
	mockPipeliner.EXPECT().Exec(gomock.Any()).Return(nil, nil)
	mockMigrator.EXPECT().commitMigration(c, data).Return(nil)

	err := m.commitMigration(c, data)
	assert.NoError(t, err)
}

func TestRedisMigrator_CommitMigration_HSetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockPipeliner := NewMockPipeliner(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	c.Logger = mockLogger

	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
		RedisTx:         mockPipeliner,
	}

	testErr := errHSet
	mockPipeliner.EXPECT().HSet(gomock.Any(), "gofr_migrations", gomock.Any()).Return(goRedis.NewIntResult(0, testErr))
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	err := m.commitMigration(c, data)
	assert.Equal(t, testErr, err)
}

func TestRedisMigrator_CommitMigration_ExecError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockPipeliner := NewMockPipeliner(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	c.Logger = mockLogger

	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
		RedisTx:         mockPipeliner,
	}

	testErr := errRedisExec

	mockPipeliner.EXPECT().HSet(gomock.Any(), "gofr_migrations", gomock.Any()).Return(goRedis.NewIntResult(1, nil))
	mockPipeliner.EXPECT().Exec(gomock.Any()).Return(nil, testErr)
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	err := m.commitMigration(c, data)
	assert.Equal(t, testErr, err)
}

func TestRedisMigrator_Rollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockPipeliner := NewMockPipeliner(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	c.Logger = mockLogger

	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	data := transactionData{
		MigrationNumber: 1,
		RedisTx:         mockPipeliner,
	}

	mockPipeliner.EXPECT().Discard()
	mockMigrator.EXPECT().rollback(c, data)
	mockLogger.EXPECT().Fatalf(gomock.Any(), gomock.Any())

	m.rollback(c, data)
}

func TestRedisMigrator_UnlockError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	c.Logger = mockLogger

	m := redisMigrator{Redis: mocks.Redis, migrator: mockMigrator}

	testErr := errEval
	mocks.Redis.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{lockKey}, "owner-1").Return(goRedis.NewCmdResult(nil, testErr))
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	err := m.unlock(c, "owner-1")
	assert.Equal(t, errLockReleaseFailed, err)
}

func TestRedisMigrator_Name(t *testing.T) {
	m := redisMigrator{}
	assert.Equal(t, "Redis", m.name())
}
