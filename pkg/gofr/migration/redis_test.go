package migration

import (
	"errors"
	"testing"
	"time"

	goRedis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
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
			expectedLastMigration: -1,
		},
		{
			desc: "UnmarshalError",
			mockedData: map[string]string{
				"1": `{"method":"UP","startTime":"2024-01-01T00:00:00Z","duration":1000}`,
				"2": "invalid JSON data",
			},
			expectedLastMigration: -1,
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

	m := redisMigrator{
		Redis:    mocks.Redis,
		migrator: mockMigrator,
	}

	mocks.Redis.EXPECT().TxPipeline()
	mockMigrator.EXPECT().beginTransaction(c)

	data := m.beginTransaction(c)

	assert.Equal(t, transactionData{}, data, "TEST Failed.\n")
}

var errRedis = errors.New("redis error")

func TestRedisMigrator_AcquireLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	m := redisMigrator{Redis: mocks.Redis}

	t.Run("Success", func(t *testing.T) {
		mocks.Redis.EXPECT().SetNX(gomock.Any(), lockKey, "1", 60*time.Second).
			Return(goRedis.NewBoolResult(true, nil))

		err := m.AcquireLock(c)
		require.NoError(t, err)
	})

	t.Run("RetrySuccess", func(t *testing.T) {
		// First attempt fails (lock held)
		mocks.Redis.EXPECT().SetNX(gomock.Any(), lockKey, "1", 60*time.Second).
			Return(goRedis.NewBoolResult(false, nil))

		// Second attempt succeeds
		mocks.Redis.EXPECT().SetNX(gomock.Any(), lockKey, "1", 60*time.Second).
			Return(goRedis.NewBoolResult(true, nil))

		err := m.AcquireLock(c)
		require.NoError(t, err)
	})

	t.Run("RedisError", func(t *testing.T) {
		mocks.Redis.EXPECT().SetNX(gomock.Any(), lockKey, "1", 60*time.Second).
			Return(goRedis.NewBoolResult(false, errRedis))

		err := m.AcquireLock(c)
		require.Error(t, err)
		assert.Equal(t, ErrLockAcquisitionFailed, err)
	})
}

func TestRedisMigrator_ReleaseLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	c, mocks := container.NewMockContainer(t)
	m := redisMigrator{Redis: mocks.Redis}

	t.Run("Success", func(t *testing.T) {
		mocks.Redis.EXPECT().Del(gomock.Any(), lockKey).
			Return(goRedis.NewIntResult(1, nil))

		err := m.ReleaseLock(c)
		require.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		mocks.Redis.EXPECT().Del(gomock.Any(), lockKey).
			Return(goRedis.NewIntResult(0, errRedis))

		err := m.ReleaseLock(c)
		require.Error(t, err)
		assert.Equal(t, ErrLockReleaseFailed, err)
	})
}
