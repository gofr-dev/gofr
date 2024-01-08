package gofr

import (
	"testing"

	"gofr.dev/pkg/gofr/config"

	"github.com/stretchr/testify/assert"
)

func Test_newContainerSuccessWithLogger(t *testing.T) {
	cfg := config.NewEnvFile("")

	container := newContainer(cfg)

	assert.NotNil(t, container)
}

func Test_newContainerRedisIntialisationFail(t *testing.T) {
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("DB_HOST", "localhost")

	cfg := config.NewEnvFile("")

	container := newContainer(cfg)

	assert.NotNil(t, container)
}
