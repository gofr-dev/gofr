package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
)

func Test_newContainerSuccessWithLogger(t *testing.T) {
	cfg := config.NewEnvFile("")

	container := NewContainer(cfg)

	assert.NotNil(t, container.Logger, "TEST, Failed.\nlogger initialisation")
}

func Test_newContainerDBIntializationFail(t *testing.T) {
	t.Setenv("REDIS_HOST", "invalid")
	t.Setenv("DB_DIALECT", "mysqlgit ")
	t.Setenv("DB_HOST", "invalid")

	cfg := config.NewEnvFile("")

	container := NewContainer(cfg)

	// container is a pointer and we need to see if db are not initialized, comparing the container object
	// will not suffice the purpose of this test
	assert.Nil(t, container.DB.DB, "TEST, Failed.\ninvalid db connections")
	assert.Nil(t, container.Redis.Client, "TEST, Failed.\ninvalid redis connections")
}
