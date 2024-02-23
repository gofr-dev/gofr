package migrations

import (
	"errors"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/migration"
)

func TestAddEmployeeInRedis(t *testing.T) {
	client, mock := redismock.NewClientMock()
	datasource := migration.Datasource{Redis: client}

	t.Run("TestSuccess", func(t *testing.T) {
		// Set expectations for the Set method
		mock.ExpectSet("Umang", "0987654321", 0).SetVal("OK")

		// Call the UP method of the migration
		err := addEmployeeInRedis().UP(datasource)
		assert.NoError(t, err)

		// Ensure all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("TestError", func(t *testing.T) {
		mock.ExpectSet("Umang", "0987654321", 0).SetErr(errors.New("redis error"))

		// Create an instance of migration.Datasource using the mock Redis client
		datasource := migration.Datasource{Redis: client}

		// Call the UP method of the migration
		err := addEmployeeInRedis().UP(datasource)

		if err == nil || err.Error() != "redis error" {
			t.Errorf("unexpected error: %v", err)
		}

		// Ensure all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
