package migrations

import (
	"errors"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/migration"
)

func TestAddEmployeeInRedis(t *testing.T) {
	client, mock := redismock.NewClientMock()
	datasource := migration.Datasource{Redis: client}

	// Set expectations for the Set method
	mock.ExpectSet("Umang", "0987654321", 0).SetVal("OK")

	// Call the UP method of the migration
	err := addEmployeeInRedis().UP(datasource)
	require.NoError(t, err)
}

func TestAddEmployeeInRedis_Error(t *testing.T) {
	client, mock := redismock.NewClientMock()
	datasource := migration.Datasource{Redis: client}

	mock.ExpectSet("Umang", "0987654321", 0).SetErr(errors.New("redis error"))

	// Call the UP method of the migration
	err := addEmployeeInRedis().UP(datasource)

	if err == nil || err.Error() != "redis error" {
		t.Errorf("TestAddEmployeeInRedis Error failed! unexpected error: %v", err)
	}
}
