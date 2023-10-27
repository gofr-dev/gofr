//go:build !skip

package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr"
)

func initializeTests() *gofr.Gofr {
	app := gofr.New()
	return app
}

func Test_CQL_K20230113160630_Up(t *testing.T) {
	app := initializeTests()
	k := K20230113160630{}

	err := k.Up(&app.DataStore, app.Logger)

	assert.Nil(t, err, "TEST, failed.\n%s", "success")
}
