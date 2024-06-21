package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_getMigratorDatastoreNotInitialised(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		container, _ := container.NewMockContainer(t)
		container.SQL = nil
		container.Redis = nil

		mg := manager{}

		mg.rollback(container, transactionData{})

		assert.Equal(t, int64(0), mg.getLastMigration(container), "TEST Failed \n Last Migration is not 0")
		assert.Nil(t, mg.checkAndCreateMigrationTable(container), "TEST Failed")
		assert.Equal(t, transactionData{}, mg.beginTransaction(container), "TEST Failed")
		assert.Nil(t, mg.commitMigration(container, transactionData{}), "TEST Failed")
	})

	assert.Contains(t, logs, "Migration 0 ran successfully", "TEST Failed")
}
