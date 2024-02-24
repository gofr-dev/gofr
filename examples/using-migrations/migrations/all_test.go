package migrations

import (
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/migration"
	"testing"
)

func TestAll_ReturnsMapOfMigrations(t *testing.T) {
	result := All()

	// Assert
	assert.NotNil(t, result)
	assert.IsType(t, map[int64]migration.Migrate{}, result)
}
