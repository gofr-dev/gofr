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

func initYCQLTests(t *testing.T) *YCQL {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")

	port, err := strconv.Atoi(c.Get("YCQL_DB_PORT"))
	if err != nil {
		port = 9042
	}

	yugabyteDBConfig := datastore.CassandraCfg{
		Hosts:       c.Get("CASS_DB_HOST"),
		Port:        port,
		Consistency: "LOCAL_QUORUM",
		Username:    c.Get("YCQL_DB_USER"),
		Password:    c.Get("YCQL_DB_PASS"),
		Keyspace:    "test",
		Timeout:     600,
	}

	db, err := datastore.GetNewYCQL(logger, &yugabyteDBConfig)

	if err != nil {
		t.Error(err)
	}

	ycqlDB := NewYCQL(&db)

	return ycqlDB
}

func createYCQLTable(ycql *YCQL, t *testing.T) {
	migrationTableSchema := "CREATE TABLE IF NOT EXISTS gofr_migrations ( " +
		"app text, version bigint, start_time timestamp, end_time timestamp, method text, PRIMARY KEY (app, version, method) )"

	err := ycql.session.Query(migrationTableSchema).Exec()
	if err != nil {
		t.Errorf("Failed creation of gofr_migrations table :%v", err)
	}
}

func TestYCQL_Run(t *testing.T) {
	ycql := initYCQLTests(t)
	createYCQLTable(ycql, t)

	logger := log.NewMockLogger(io.Discard)

	defer func() {
		err := ycql.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	tests := []struct {
		desc   string
		method string
		ycql   *YCQL
		err    error
	}{
		{"success case", "UP", ycql, nil},
		{"failure case", "DOWN", ycql,
			&errors.Response{Reason: "error encountered in running the migration", Detail: errors.Error("test error")}},
		{"db not initialized", "UP", &YCQL{}, errors.DataStoreNotInitialized{DBName: datastore.Ycql}},
	}

	for i, tc := range tests {
		err := tc.ycql.Run(K20180324120906{}, "testApp", "20180324120906", tc.method, logger)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestYCQL_GetAllMigrations(t *testing.T) {
	ycql := initYCQLTests(t)
	now := time.Now()

	createYCQLTable(ycql, t)

	defer func() {
		err := ycql.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	insertYCQLMigration(t, ycql, &gofrMigration{
		App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "UP",
	})
	insertYCQLMigration(t, ycql, &gofrMigration{
		App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "DOWN",
	})

	expOut := []int{20180324120906}

	up, down := ycql.GetAllMigrations("gofr-app-v3")

	assert.Equal(t, expOut, up, "TEST failed.\n%s", "get all UP migrations")

	assert.Equal(t, expOut, down, "TEST failed.\n%s", "get all DOWN migrations")
}

func TestYCQL_Run_Fail(t *testing.T) {
	m := &YCQL{}

	expectedError := errors.DataStoreNotInitialized{DBName: datastore.Ycql}

	err := m.Run(K20180324120906{}, "sample-api", "20180324120906", "UP", log.NewMockLogger(io.Discard))

	assert.EqualError(t, err, expectedError.Error())
}

func TestYCQL_LastRunVersion_Fail(t *testing.T) {
	m := &YCQL{}

	expectedLastVersion := -1

	lastVersion := m.LastRunVersion("gofr", "UP")

	assert.Equal(t, expectedLastVersion, lastVersion)
}

func TestYCQL_GetAllMigrations_Fail(t *testing.T) {
	m := &YCQL{}

	expectedUP := []int{-1}

	upMigrations, downMigrations := m.GetAllMigrations("gofr")

	assert.Equal(t, expectedUP, upMigrations)
	assert.Nil(t, downMigrations)
}

func fetchYCQL(t *testing.T) YCQL {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")

	port, err := strconv.Atoi(c.Get("YCQL_DB_PORT"))
	if err != nil {
		port = 9042
	}

	yugabyteDBConfig := datastore.CassandraCfg{
		Hosts:       c.Get("CASS_DB_HOST"),
		Port:        port,
		Consistency: "LOCAL_QUORUM",
		Username:    c.Get("YCQL_DB_USER"),
		Password:    c.Get("YCQL_DB_PASS"),
		Keyspace:    c.Get("CASS_DB_KEYSPACE"),
		Timeout:     600,
	}

	db, err := datastore.GetNewYCQL(logger, &yugabyteDBConfig)

	if err != nil {
		t.Error(err)
	}

	ycqlDB := NewYCQL(&db)

	return *ycqlDB
}

func insertYCQLMigration(t *testing.T, ycql *YCQL, mig *gofrMigration) {
	ycql.newMigrations = append(ycql.newMigrations, *mig)

	for i, l := range ycql.newMigrations {
		err := ycql.session.Query("INSERT INTO gofr_migrations(app, version, method, start_time, end_time) "+
			"VALUES (?, ?, ?, ?, ?)", l.App, l.Version, l.Method, l.StartTime, l.EndTime).Exec()
		if err != nil {
			t.Errorf("Failed insertion of %d gofr_migrations table :%v", i, err)
		}
	}
}

func Test_ycqlMethods(t *testing.T) {
	ycqlDB := fetchYCQL(t)

	migrationTableSchema := "CREATE TABLE IF NOT EXISTS gofr_migrations ( " +
		"app text, version bigint, start_time timestamp, end_time timestamp, method text, PRIMARY KEY (app, version, method) )"

	err := ycqlDB.session.Query(migrationTableSchema).Exec()

	if !ycqlDB.isDirty("appName") {
		t.Errorf("Failed. Got %v ,Expected %v", err, true)
	}

	if err := ycqlDB.preRun("appName", "UP", "K20210116140839"); (err != nil) != true {
		t.Errorf("Failed. Got %s", err)
	}

	if err := ycqlDB.postRun("appName", "UP", "K20210116140839"); err != nil {
		t.Errorf("Failed. Got %s", err)
	}

	if err := ycqlDB.FinishMigration(); err != nil {
		t.Errorf("Failed. Got %s", err)
	}
}

func Test_FinishMigrationError(t *testing.T) {
	y := fetchYCQL(t)

	testcases := []struct {
		desc string
		ycql *YCQL
	}{
		{"failure case", &y},
		{"db not initialized", &YCQL{}},
	}

	for i, tc := range testcases {
		tc.ycql.newMigrations = []gofrMigration{{"gofr_migrations", 20010102121212, time.Now(), time.Time{}, "UP"}}

		err := tc.ycql.FinishMigration()

		assert.NotNil(t, err, "Test[%d] Failed.", i)
	}
}

func Test_FinishMigration(t *testing.T) {
	y := fetchYCQL(t)
	y.newMigrations = []gofrMigration{}

	err := y.FinishMigration()

	assert.Nil(t, err, "Test case failed")
}

func Test_postRunYcql(t *testing.T) {
	y := fetchYCQL(t)
	y.newMigrations = []gofrMigration{{"testing", 20010102121212, time.Now(), time.Time{}, "UP"},
		{"testing", 20000102121212, time.Now(), time.Time{}, "UP"}}

	err := y.postRun("testing", "UP", "20000102121212")

	assert.Nil(t, err, "Test case failed.")
}

func TestYCQL_IsDirty(t *testing.T) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")

	// Initialize Cassandra for Dirty Check
	port, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))

	ycqlConf, err := datastore.GetNewYCQL(logger, &datastore.CassandraCfg{
		Hosts:    c.Get("CASS_DB_HOST"),
		Port:     port,
		Username: c.Get("YCQL_DB_USER"),
		Password: c.Get("YCQL_DB_PASS"),
		Keyspace: "test"})

	assert.Nil(t, err, "YCQL not connected")

	ycql := NewYCQL(&ycqlConf)
	createYcqlTable(ycql, t)

	defer func() {
		err := ycql.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
		assert.Nil(t, err, "Got error while dropping the table.")
	}()

	check := ycql.isDirty("testingYcql")

	assert.Equal(t, true, check, "Test case failed.")
}

func createYcqlTable(ycql *YCQL, t *testing.T) {
	err := ycql.session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
	assert.Nil(t, err, "Got error while dropping the existing table gofr_migrations")

	migrationTableSchema := "CREATE TABLE IF NOT EXISTS gofr_migrations ( app text, version bigint, start_time timestamp, " +
		"end_time timestamp, method text, PRIMARY KEY (app, version, method) )"

	err = ycql.session.Query(migrationTableSchema).Exec()

	assert.Nil(t, err, "Got error while creating the table gofr_migrations: %v", err)

	err = ycql.session.Query("INSERT INTO gofr_migrations (app, version, start_time, method, end_time) "+
		"values ('testingYcql', 7, dateof(now()), 'UP', ?)", time.Time{}).Exec()

	assert.Nil(t, err, "Test case failed.")
}

type K20180324120551 struct{}

func (k K20180324120551) Up(_ *datastore.DataStore, _ log.Logger) error {
	return nil
}

func (k K20180324120551) Down(_ *datastore.DataStore, _ log.Logger) error {
	return errors.Error("test error")
}

func Test_runError(t *testing.T) {
	y := fetchYCQL(t)

	err := y.Run(K20180324120551{}, "testing", "k20180324120551", "UP", log.NewLogger())

	assert.NotNil(t, err, "Test case failed")
}

func TestYCQL_LastRunVersion(t *testing.T) {
	y := fetchYCQL(t)

	var expected int

	y.newMigrations = []gofrMigration{{"testing", 20010102121212, time.Now(), time.Time{}, "UP"},
		{"testing", 20000102121212, time.Now(), time.Time{}, "UP"}}

	resp := y.LastRunVersion("testing", "UP")

	assert.IsType(t, expected, resp, "Test case failed")
}
