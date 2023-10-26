package dbmigration

import (
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

func insertCassandraMigration(t *testing.T, c *Cassandra, mig *gofrMigration) {
	c.newMigrations = append(c.newMigrations, *mig)

	for i, l := range c.newMigrations {
		err := c.session.Query("INSERT INTO gofr_migrations(app, version, method, start_time, end_time) "+
			"VALUES (?, ?, ?, ?, ?)", l.App, l.Version, l.Method, l.StartTime, l.EndTime).Exec()
		if err != nil {
			t.Errorf("Failed insertion of %d gofr_migrations table :%v", i, err)
		}
	}
}

func initCassandraTests() *Cassandra {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")

	var configs datastore.CassandraCfg

	cassandraPort, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))

	configs = datastore.CassandraCfg{
		Hosts:    c.Get("CASS_DB_HOST"),
		Port:     cassandraPort,
		Username: c.Get("CASS_DB_USER"),
		Password: c.Get("CASS_DB_PASS"),
		Keyspace: "test",
	}

	db, _ := datastore.GetNewCassandra(logger, &configs)
	cass := NewCassandra(&db)

	return cass
}

func Test_CQL_Run(t *testing.T) {
	c := initCassandraTests()
	createCassandraTable(c, t)

	logger := log.NewMockLogger(io.Discard)

	defer func() {
		err := c.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	tests := []struct {
		desc   string
		method string
		cass   *Cassandra
		err    error
	}{
		{"success case", "UP", c, nil},
		{"failure case", "DOWN", c, &errors.Response{Reason: "error encountered in running the migration", Detail: errors.Error("test error")}},
		{"db not initialized", "UP", &Cassandra{}, errors.DataStoreNotInitialized{DBName: datastore.CassandraStore}},
	}

	for i, tc := range tests {
		err := tc.cass.Run(K20180324120906{}, "testApp", "20180324120906", tc.method, logger)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_CQL_LastRunVersion(t *testing.T) {
	cass := initCassandraTests()

	createCassandraTable(cass, t)

	defer func() {
		err := cass.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	testcases := []struct {
		desc            string
		cass            *Cassandra
		expectedVersion int
	}{
		{"success case", cass, 0},
		{"db not initialized", &Cassandra{}, -1},
	}

	for i, tc := range testcases {
		lastVersion := tc.cass.LastRunVersion("sample-api-v2", "UP")

		assert.Equal(t, tc.expectedVersion, lastVersion, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}

func Test_CQL_GetAllMigrations(t *testing.T) {
	c := initCassandraTests()
	now := time.Now()

	createCassandraTable(c, t)

	defer func() {
		err := c.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	insertCassandraMigration(t, c, &gofrMigration{
		App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "UP",
	})
	insertCassandraMigration(t, c, &gofrMigration{
		App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "DOWN",
	})

	expOut := []int{20180324120906}

	up, down := c.GetAllMigrations("gofr-app-v3")

	assert.Equal(t, expOut, up, "TEST failed.\n%s", "get all UP migrations")

	assert.Equal(t, expOut, down, "TEST failed.\n%s", "get all DOWN migrations")
}

func Test_CQL_GetAllMigrationsError(t *testing.T) {
	c := &Cassandra{}

	expectedUp := []int{-1}

	up, down := c.GetAllMigrations("gofr-app")

	assert.Equal(t, expectedUp, up, "TEST failed.\n%s", "get all migrations")

	assert.Nil(t, down, "TEST failed.\n%s", "get all migrations")
}

func Test_CQL_IsDirty(t *testing.T) {
	c := initCassandraTests()
	createCassandraTable(c, t)

	defer func() {
		err := c.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	check := c.isDirty("testingCassandra")
	if !check {
		t.Errorf("TEST failed. Dirty migration check")
	}
}

func Test_CQL_FinishMigration(t *testing.T) {
	c := initCassandraTests()
	createCassandraTable(c, t)

	defer func() {
		err := c.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	now := time.Now()
	migration := gofrMigration{App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "UP"}
	c.newMigrations = append(c.newMigrations, migration)
	err := c.FinishMigration()

	assert.Nilf(t, err, "TEST failed.")
}

func Test_CQL_FinishMigrationError(t *testing.T) {
	c := &Cassandra{}
	expectedError := errors.DataStoreNotInitialized{DBName: datastore.CassandraStore}

	err := c.FinishMigration()

	assert.EqualErrorf(t, err, expectedError.Error(), "TEST failed.")
}

// createCassandraTable will create a fresh table called gofr_migrations and insert the data required for TestCassandra_IsDirty
func createCassandraTable(cassandra *Cassandra, t *testing.T) {
	err := cassandra.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
	if err != nil {
		t.Errorf("Got error while dropping the existing table gofr_migrations: %v", err)
	}

	migrationTableSchema := "CREATE TABLE IF NOT EXISTS gofr_migrations ( app text, version bigint, start_time timestamp, " +
		"end_time timestamp, method text, PRIMARY KEY (app, version, method) )"
	err = cassandra.session.Query(migrationTableSchema).Exec()

	if err != nil {
		t.Errorf("Failed creation of gofr_migrations table :%v", err)
	}

	err = cassandra.session.Query("INSERT INTO gofr_migrations (app, version, start_time, method, end_time) "+
		"values ('testingCassandra', 7, dateof(now()), 'UP', ?)", time.Time{}).Exec()
	if err != nil {
		t.Errorf("Insert Error: %v", err)
	}
}
