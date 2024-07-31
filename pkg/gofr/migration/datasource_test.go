package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_getMigratorDatastoreNotInitialised(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		mockContainer, _ := container.NewMockContainer(t)
		mockContainer.SQL = nil
		mockContainer.Redis = nil

		mg := Datasource{}

		mg.rollback(mockContainer, transactionData{})

		assert.Equal(t, int64(0), mg.getLastMigration(mockContainer), "TEST Failed \n Last Migration is not 0")
		require.NoError(t, mg.checkAndCreateMigrationTable(mockContainer), "TEST Failed")
		assert.Equal(t, transactionData{}, mg.beginTransaction(mockContainer), "TEST Failed")
		require.NoError(t, mg.commitMigration(mockContainer, transactionData{}), "TEST Failed")
	})

	assert.Contains(t, logs, "Migration 0 ran successfully", "TEST Failed")
}
