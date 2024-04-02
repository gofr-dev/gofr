// //go:build !migration
package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_MigrationInvalidKeys(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvLoader{})

		Run(map[int64]Migrate{
			1: {UP: nil},
		}, cntnr)
	})

	assert.Contains(t, logs, "UP not defined for the following keys: [1]")
}
