package migration

import (
	"context"
	"testing"

	red "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/container"
)

func TestNewRedis(t *testing.T) {
	mockCmd := &Mockcommands{}

	r := newRedis(mockCmd)
	if r.commands != mockCmd {
		t.Errorf("Expected newRedis to set commands, but got %v", r.commands)
	}
}

func TestRedis_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := NewMockcommands(ctrl)

	mockCmd.EXPECT().Get(context.Background(), "test_key").Return(&red.StringCmd{})

	r := redis{mockCmd}

	_, err := r.Get(context.Background(), "test_key").Result()
	assert.NoError(t, err)
}

func TestRedisMigrator_GetLastMigration2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c, mocks := container.NewMockContainer(t)
	mockMigrator := NewMockMigrator(ctrl)

	m := redisMigrator{
		commands: mocks.Redis,
		Migrator: mockMigrator,
	}

	testCases := []struct {
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
			redisErr:              red.ErrClosed,
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

	for i, tc := range testCases {
		mocks.Redis.EXPECT().HGetAll(gomock.Any(), "gofr_migrations").Return(
			red.NewMapStringStringResult(tc.mockedData, tc.redisErr))

		mockMigrator.EXPECT().getLastMigration(gomock.Any()).Return(tc.migratorLastMigration).AnyTimes()

		lastMigration := m.getLastMigration(c)

		assert.Equal(t, tc.expectedLastMigration, lastMigration, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
