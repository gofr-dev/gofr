package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	dbmigration "gofr.dev/cmd/gofr/migration/dbMigration"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

type K20200324120906 struct{}

func (k K20200324120906) Up(_ *datastore.DataStore, l log.Logger) error {
	l.Info("Running test migration: UP")
	return nil
}

func (k K20200324120906) Down(_ *datastore.DataStore, _ log.Logger) error {
	return &errors.Response{Reason: "test error"}
}

type K20200324150906 struct{}

func (k K20200324150906) Up(_ *datastore.DataStore, l log.Logger) error {
	l.Info("Running test migration: UP")
	return nil
}

func (k K20200324150906) Down(_ *datastore.DataStore, _ log.Logger) error {
	return &errors.Response{Reason: "test error"}
}

type K20190324150906 struct{}

func (k K20190324150906) Up(_ *datastore.DataStore, l log.Logger) error {
	l.Info("Running test migration: UP")
	return nil
}

func (k K20190324150906) Down(_ *datastore.DataStore, _ log.Logger) error {
	return &errors.Response{Reason: "test error"}
}

type K20200402143245 struct{}

func (k K20200402143245) Up(_ *datastore.DataStore, l log.Logger) error {
	l.Info("Running test migration: UP")
	return &errors.Response{Reason: "test error"}
}

func (k K20200402143245) Down(_ *datastore.DataStore, _ log.Logger) error {
	return nil
}

type K20200423083024 struct{}

func (k K20200423083024) Up(_ *datastore.DataStore, _ log.Logger) error {
	return nil
}

func (k K20200423083024) Down(_ *datastore.DataStore, _ log.Logger) error {
	return nil
}

type K20200423093024 struct{}

func (k K20200423093024) Up(_ *datastore.DataStore, _ log.Logger) error {
	return nil
}

func (k K20200423093024) Down(_ *datastore.DataStore, _ log.Logger) error {
	return nil
}

const (
	appName  = "gofr-test"
	keyspace = "test_migrations"
)

func Test_runUP_Fail(t *testing.T) {
	db := &dbmigration.GORM{}
	expectedError := errors.DataStoreNotInitialized{DBName: "sql"}
	res, err := runUP("gofr-app", db, map[string]dbmigration.Migrator{}, log.NewMockLogger(io.Discard))

	assert.Nilf(t, res, "Test Failed. Expected: Nil Got: %v", res)
	assert.Equalf(t, expectedError, err, "Test Failed. Expected: %v Got: %v", expectedError, err)
}

func Test_runDOWN_Fail(t *testing.T) {
	db := &dbmigration.GORM{}
	expectedError := errors.DataStoreNotInitialized{DBName: "sql"}
	res, err := runDOWN("gofr-app", db, map[string]dbmigration.Migrator{}, log.NewMockLogger(io.Discard))

	assert.Nilf(t, res, "Test Failed. Expected: Nil Got: %v", res)
	assert.Equalf(t, expectedError, err, "Test Failed. Expected: %v Got: %v", expectedError, err)
}
func TestMain(m *testing.M) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../../configs")
	cassandraPort, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))
	cassandraCfg := datastore.CassandraCfg{
		Hosts:    c.Get("CASS_DB_HOST"),
		Port:     cassandraPort,
		Username: c.Get("CASS_DB_USER"),
		Password: c.Get("CASS_DB_PASS"),
		Keyspace: "system",
	}

	cassandra, err := datastore.GetNewCassandra(logger, &cassandraCfg)
	if err != nil {
		logger.Errorf("[FAILED] unable to connect to cassandra with system keyspace %s", err)
	}

	query := fmt.Sprintf("CREATE KEYSPACE IF NOT EXISTS %v WITH replication = "+
		"{'class':'SimpleStrategy', 'replication_factor' : 1} ", keyspace)

	err = cassandra.Session.Query(query).Exec()
	if err != nil {
		logger.Errorf("unable to create %v keyspace %s", keyspace, err)
	}

	os.Exit(m.Run())
}

func TestRedisAndMongo_Migration(t *testing.T) {
	logger := log.NewMockLogger(new(bytes.Buffer))
	c := config.NewGoDotEnvProvider(log.NewMockLogger(new(bytes.Buffer)), "../../../configs")

	// initialize data stores
	redis, _ := datastore.NewRedis(logger, &datastore.RedisConfig{
		HostName: c.Get("REDIS_HOST"),
		Port:     c.Get("REDIS_PORT"),
	})

	mongo, _ := datastore.GetNewMongoDB(logger, &datastore.MongoConfig{
		HostName: c.Get("MONGO_DB_HOST"),
		Port:     c.Get("MONGO_DB_PORT"),
		Username: c.Get("MONGO_DB_USER"),
		Password: c.Get("MONGO_DB_PASS"),
		Database: c.Get("MONGO_DB_NAME")})

	defer func() {
		_ = mongo.Collection("gofr_migrations").Drop(context.TODO())

		redis.Del(context.Background(), "gofr_migrations")
	}()

	testcases := []struct {
		method     string
		migrations map[string]dbmigration.Migrator

		err error
	}{
		{"UP", map[string]dbmigration.Migrator{"20190324150906": K20190324150906{}}, nil},
		{"UP", nil, nil},
		{"UP", map[string]dbmigration.Migrator{"20200324120906": K20200324120906{}}, nil},
		{"UP", map[string]dbmigration.Migrator{"20200402143245": K20200402143245{}}, &errors.Response{Reason: "test error"}},
		{"DOWN", map[string]dbmigration.Migrator{"20200324120906": K20200324120906{}}, &errors.Response{Reason: "test error"}},
		{"UP", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}, "20200423093024": K20200423093024{}}, nil},
		{"DOWN", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}, "20200423093024": K20200423093024{}}, nil},
		{"DOWN", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}}, nil},
	}

	for i, v := range testcases {
		err := Migrate(appName, dbmigration.NewRedis(redis), v.migrations, v.method, logger)

		assert.Equal(t, v.err, err, "TEST[%d], Failed.\n", i)

		err = Migrate(appName, dbmigration.NewMongo(mongo), v.migrations, v.method, logger)

		assert.Equal(t, v.err, err, "TEST[%d], Failed.\n", i)
	}
}

func TestSQL_Migration(t *testing.T) {
	logger := log.NewMockLogger(new(bytes.Buffer))
	c := config.NewGoDotEnvProvider(log.NewMockLogger(new(bytes.Buffer)), "../../../configs")
	pgsql, _ := datastore.NewORM(&datastore.DBConfig{
		HostName: c.Get("PGSQL_HOST"),
		Username: c.Get("PGSQL_USER"),
		Password: c.Get("PGSQL_PASSWORD"),
		Database: c.Get("PGSQL_DB_NAME"),
		Port:     c.Get("PGSQL_PORT"),
		Dialect:  "postgres",
	})

	defer func() {
		_ = pgsql.Migrator().DropTable("gofr_migrations")
	}()

	// ensures the gofr_migrations table is dropped in DB
	tx := pgsql.DB.Exec("DROP TABLE IF EXISTS gofr_migrations")
	if tx != nil {
		assert.NoError(t, tx.Error)
	}

	testcases := []struct {
		method     string
		migrations map[string]dbmigration.Migrator

		err error
	}{
		{"UP", map[string]dbmigration.Migrator{"20190324150906": K20190324150906{}}, nil},
		{"UP", nil, nil},
		{"UP", map[string]dbmigration.Migrator{"20200402143245": K20200402143245{}},
			&errors.Response{Reason: "error encountered in running the migration", Detail: &errors.Response{Reason: "test error"}}},
		{"UP", map[string]dbmigration.Migrator{"20200324120906": K20200324120906{}}, nil},
		{"DOWN", map[string]dbmigration.Migrator{"20200324120906": K20200324120906{}},
			&errors.Response{Reason: "error encountered in running the migration", Detail: &errors.Response{Reason: "test error"}}},
		{"UP", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}, "20200423093024": K20200423093024{}}, nil},
		{"DOWN", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}, "20200423093024": K20200423093024{}}, nil},
		{"DOWN", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}}, nil},
	}

	for i, v := range testcases {
		err := Migrate(appName, dbmigration.NewGorm(pgsql.DB), v.migrations, v.method, logger)

		assert.Equal(t, v.err, err, "TEST[%d], Failed.\n", i)
	}
}

func TestCassandra_Migration(t *testing.T) {
	logger := log.NewMockLogger(new(bytes.Buffer))
	c := config.NewGoDotEnvProvider(logger, "../../../configs")
	cassandraPort, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))
	cassandraCfg := datastore.CassandraCfg{
		Hosts:    c.Get("CASS_DB_HOST"),
		Port:     cassandraPort,
		Username: c.Get("CASS_DB_USER"),
		Password: c.Get("CASS_DB_PASS"),
		Keyspace: keyspace,
	}

	cassandra, err := datastore.GetNewCassandra(logger, &cassandraCfg)
	if err != nil {
		t.Errorf("CQL not connected, err: %v. Cannot execute test", err)
	}

	err = cassandra.Session.Query("DROP TABLE IF EXISTS gofr_migrations  ").Exec()
	if err != nil {
		t.Errorf("CQL not connected, err: %v. Cannot execute test", err)
	}

	defer func() {
		_ = cassandra.Session.Query("DROP TABLE IF EXISTS gofr_migrations ").Exec()
	}()

	testcases := []struct {
		method     string
		migrations map[string]dbmigration.Migrator

		err error
	}{
		{"UP", map[string]dbmigration.Migrator{"20190324150906": K20190324150906{}}, nil},
		{"UP", map[string]dbmigration.Migrator{"20200324120906": K20200324120906{}}, nil},
		{"UP", map[string]dbmigration.Migrator{"20200402143245": K20200402143245{}},
			&errors.Response{Reason: "error encountered in running the migration", Detail: &errors.Response{Reason: "test error"}}},
		{"DOWN", map[string]dbmigration.Migrator{"20200324120906": K20200324120906{}},
			&errors.Response{Reason: "error encountered in running the migration", Detail: &errors.Response{Reason: "test error"}}},
		{"UP", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}, "20200423093024": K20200423093024{}}, nil},
		{"DOWN", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}, "20200423093024": K20200423093024{}}, nil},
		{"DOWN", map[string]dbmigration.Migrator{"20200423083024": K20200423083024{}}, nil},
	}

	for i, v := range testcases {
		err := Migrate(appName, dbmigration.NewCassandra(&cassandra), v.migrations, v.method, logger)

		assert.Equal(t, v.err, err, "TEST[%d], Failed.\n", i)
	}
}

func TestMigrateError(t *testing.T) {
	logger := log.NewMockLogger(new(bytes.Buffer))

	err := Migrate(appName, nil, nil, "UP", logger)
	if err == nil {
		t.Errorf("expected err, got nil")
	}
}

func Test_MigrateCheck(t *testing.T) {
	b := new(bytes.Buffer)
	mockLogger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(mockLogger, "../../../configs")

	mssql, _ := datastore.NewORM(&datastore.DBConfig{
		HostName: c.Get("MSSQL_HOST"),
		Username: c.Get("MSSQL_USER"),
		Password: c.Get("MSSQL_PASSWORD"),
		Database: c.Get("MSSQL_DB_NAME"),
		Port:     c.Get("MSSQL_PORT"),
		Dialect:  "mssql",
	})

	defer func() {
		_ = mssql.Migrator().DropTable("gofr_migrations")
	}()

	// ensures the gofr_migrations table is dropped in DB
	tx := mssql.DB.Exec("DROP TABLE IF EXISTS gofr_migrations")
	if tx != nil {
		assert.NoError(t, tx.Error)
	}

	migrations := map[string]dbmigration.Migrator{"20210324150906": K20200324150906{},
		"20200324120906": K20200324120906{},
		"20190324150906": K20190324150906{}}

	if err := Migrate(appName, dbmigration.NewGorm(mssql.DB), migrations, "UP", mockLogger); err != nil {
		t.Errorf("expected nil error\tgot %v", err)
	}

	loggedStr := b.String()
	i1 := strings.Index(loggedStr, "20190324150906")
	i2 := strings.Index(loggedStr, "20200324120906")
	i3 := strings.Index(loggedStr, "20210324150906")

	if i1 > i2 || i2 > i3 {
		t.Errorf("Sequence of migration run is not in order, got: %v", loggedStr)
	}
}

type gofrMigration struct {
	App       string    `gorm:"primary_key"`
	Version   int64     `gorm:"primary_key;auto_increment:false"`
	StartTime time.Time `gorm:"autoCreateTime"`
	EndTime   time.Time `gorm:"default:NULL"`
	Method    string    `gorm:"primary_key"`
}

func Test_DirtyTest(t *testing.T) {
	logger := log.NewMockLogger(new(bytes.Buffer))
	c := config.NewGoDotEnvProvider(log.NewMockLogger(new(bytes.Buffer)), "../../../configs")

	// initialize data stores
	redis, _ := datastore.NewRedis(logger, &datastore.RedisConfig{
		HostName: c.Get("REDIS_HOST"),
		Port:     c.Get("REDIS_PORT"),
	})

	port, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))
	cassandra, _ := datastore.GetNewCassandra(logger, &datastore.CassandraCfg{
		Hosts:       c.Get("CASS_DB_HOST"),
		Port:        port,
		Consistency: c.Get("CASS_DB_CONSISTENCY"),
		Username:    c.Get("CASS_DB_CONSISTENCY"),
		Password:    c.Get("CASS_DB_PASS"),
		Keyspace:    "test"})

	_ = cassandra.Session.Query("drop table gofr_migrations").Exec()

	pgsql, _ := datastore.NewORM(&datastore.DBConfig{
		HostName: c.Get("PGSQL_HOST"),
		Username: c.Get("PGSQL_USER"),
		Password: c.Get("PGSQL_PASSWORD"),
		Database: c.Get("PGSQL_DB_NAME"),
		Port:     c.Get("PGSQL_PORT"),
		Dialect:  "postgres",
	})

	migrationTableSchema := "CREATE TABLE IF NOT EXISTS gofr_migrations ( app text, version bigint, start_time timestamp, end_time text, " +
		"method text, PRIMARY KEY (app, version, method) )"

	_ = cassandra.Session.Query(migrationTableSchema).Exec()
	_ = cassandra.Session.Query("insert into gofr_migrations (app, version, start_time, method, end_time) " +
		"values ('testing', 12, dateof(now()), 'UP', '')").Exec()

	err := pgsql.Migrator().CreateTable(&gofrMigration{})
	if err != nil {
		t.Log("Error while creating table:", err)
	}

	assert.NoError(t, err)

	pgsql.Create(&gofrMigration{App: "testing", Method: "UP", Version: 20000102121212, StartTime: time.Now()})

	ctx := context.Background()
	redisMigrator := dbmigration.NewRedis(redis)
	resBytes, _ := json.Marshal([]gofrMigration{{"testing", 20010102121212, time.Now(), time.Time{}, "UP"},
		{"testing", 20000102121212, time.Now(), time.Time{}, "UP"}})
	redis.HSet(ctx, "gofr_migrations", "testing", string(resBytes))

	redisMigrator.LastRunVersion("testing", "UP")

	defer func() {
		_ = pgsql.Migrator().DropTable("gofr_migrations")
		_ = cassandra.Session.Query("truncate gofr_migrations").Exec()
		_ = redis.Del(ctx, "gofr_migrations")
	}()

	type args struct {
		app    string
		method string
		name   string
	}

	tests := []struct {
		name     string
		dbDriver dbmigration.DBDriver
		args     args
		wantErr  bool
	}{
		{"cassandra: dirty check", dbmigration.NewCassandra(&cassandra), args{"testing", "UP", "12"}, true},
		{"pgsql: dirty check", dbmigration.NewGorm(pgsql.DB), args{"testing", "UP", "12"}, true},
		{"redis: dirty check", redisMigrator, args{"testing", "UP", "12"}, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.dbDriver.Run(nil, tt.args.app, tt.args.method, tt.args.name, logger); (err != nil) != tt.wantErr {
				t.Errorf("preRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getDBName(t *testing.T) {
	var kafka dbmigration.DBDriver

	testCases := []struct {
		desc           string
		expectedDBName string
		datastore      dbmigration.DBDriver
	}{
		{"mongo DB driver", "mongo", &dbmigration.Mongo{}},
		{"cassandra DB driver", "cassandra", &dbmigration.Cassandra{}},
		{"ycql DB driver", "ycql", &dbmigration.YCQL{}},
		{"sql DB driver", "sql", &dbmigration.GORM{}},
		{"redis driver", "redis", &dbmigration.Redis{}},
		{"default db", "datastore", kafka},
	}

	for i, tc := range testCases {
		dbName := getDBName(tc.datastore)

		assert.Equal(t, tc.expectedDBName, dbName, "Test[%d] Failed. %s", i, tc.desc)
	}
}
