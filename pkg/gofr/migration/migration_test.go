package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMigration_InvalidKeys(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		c, _ := container.NewMockContainer(t)

		Run(map[int64]Migrate{
			1: {UP: nil},
		}, c)
	})

	assert.Contains(t, logs, "migration run failed! UP not defined for the following keys: [1]")
}

func TestMigration_NoDatasource(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		c := container.NewContainer(nil)
		c.Logger = logging.NewLogger(logging.DEBUG)

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				return nil
			}},
		}, c)
	})

	assert.Contains(t, logs, "no migrations are running")
}

func Test_getMigratorDBInitialisation(t *testing.T) {
	cntnr, _ := container.NewMockContainer(t)

	datasource, _, isInitialised := getMigrator(cntnr)

	assert.NotNil(t, datasource.SQL, "TEST Failed \nSQL not initialized, but should have been initialized")
	assert.NotNil(t, datasource.Redis, "TEST Failed \nRedis not initialized, but should have been initialized")
	assert.Equal(t, true, isInitialised, "TEST Failed \nNo datastores are Initialized")
}

func Test_getMigratorDatastoreNotInitialised(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		container, _ := container.NewMockContainer(t)
		container.SQL = nil
		container.Redis = nil

		datasource, _, isInitialised := getMigrator(container)

		datasource.rollback(container, migrationData{})

		assert.Equal(t, int64(0), datasource.getLastMigration(container), "TEST Failed \n Last Migration is not 0")
		assert.Nil(t, datasource.checkAndCreateMigrationTable(container), "TEST Failed")
		assert.Equal(t, migrationData{}, datasource.beginTransaction(container), "TEST Failed")
		assert.Nil(t, datasource.commitMigration(container, migrationData{}), "TEST Failed")

		assert.Equal(t, false, isInitialised, "TEST Failed \nDatastores are Initialized")
	})

	assert.Contains(t, logs, "Migration 0 ran successfully", "TEST Failed")
}
