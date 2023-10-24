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

func TestK20230116104833_Up(t *testing.T) {
	app := initializeTests()
	k := K20230116104833{}

	err := k.Up(&app.DataStore, app.Logger)

	assert.Nil(t, err, "TEST, failed.\n%s", "success")
}
