package migration

import (
	"context"
	"testing"
	"time"

	goRedis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
)

func TestNewRedis(t *testing.T) {
	mockCmd := &Mockcommands{}

	r := newRedis(mockCmd)
	if r.Redis != mockCmd {
		t.Errorf("Expected newRedis to set Redis, but got %v", r.Redis)
	}
}

func TestRedis_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := NewMockcommands(ctrl)
	mockCmd.EXPECT().Get(context.Background(), "test_key").Return(&goRedis.StringCmd{})

	r := redis{mockCmd}
	_, err := r.Get(context.Background(), "test_key").Result()

	assert.NoError(t, err, "TEST Failed.\n")
}

func TestRedis_Set(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := NewMockcommands(ctrl)
	mockCmd.EXPECT().Set(context.Background(), "test_key", "test_value", time.Duration(0)).Return(&goRedis.StatusCmd{})

	r := redis{mockCmd}
	_, err := r.Set(context.Background(), "test_key", "test_value", 0).Result()

	assert.NoError(t, err, "TEST Failed.\n")
}

func TestRedis_Del(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := NewMockcommands(ctrl)
	mockCmd.EXPECT().Del(context.Background(), "test_key").Return(&goRedis.IntCmd{})

	r := redis{mockCmd}
	_, err := r.Del(context.Background(), "test_key").Result()

	assert.NoError(t, err, "TEST Failed.\n")
}

func TestRedis_Rename(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := NewMockcommands(ctrl)
	mockCmd.EXPECT().Rename(context.Background(), "test_key", "test_new_key").Return(&goRedis.StatusCmd{})

	r := redis{mockCmd}
	_, err := r.Rename(context.Background(), "test_key", "test_new_key").Result()

	assert.NoError(t, err, "TEST Failed.\n")
}

func TestRedisMigrator_GetLastMigration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockMigrator(ctrl)

	m := redisMigrator{
		Redis:   mocks.Redis,
		Manager: mockMigrator,
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
		mocks.Redis.EXPECT().HGetAll(context.Background(), "gofr_migrations").Return(
			goRedis.NewMapStringStringResult(tc.mockedData, tc.redisErr))

		mockMigrator.EXPECT().getLastMigration(gomock.Any()).Return(tc.migratorLastMigration).MaxTimes(2)

		lastMigration := m.GetLastMigration(c)

		assert.Equal(t, tc.expectedLastMigration, lastMigration, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestRedisMigrator_beginTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockMigrator(ctrl)

	m := redisMigrator{
		Redis:   mocks.Redis,
		Manager: mockMigrator,
	}

	mocks.Redis.EXPECT().TxPipeline()
	mockMigrator.EXPECT().beginTransaction(c)

	data := m.BeginTransaction(c)

	assert.Equal(t, transactionData{}, data, "TEST Failed.\n")
}
