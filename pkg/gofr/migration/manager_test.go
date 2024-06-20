package migration

import (
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
	"testing"
)

func Test_getMigratorDatastoreNotInitialised(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		container, _ := container.NewMockContainer(t)
		container.SQL = nil
		container.Redis = nil

		mg := manager{}

		mg.Rollback(container, transactionData{})

		assert.Equal(t, int64(0), mg.GetLastMigration(container), "TEST Failed \n Last Migration is not 0")
		assert.Nil(t, mg.CheckAndCreateMigrationTable(container), "TEST Failed")
		assert.Equal(t, transactionData{}, mg.BeginTransaction(container), "TEST Failed")
		assert.Nil(t, mg.CommitMigration(container, transactionData{}), "TEST Failed")
	})

	assert.Contains(t, logs, "Migration 0 ran successfully", "TEST Failed")
}
