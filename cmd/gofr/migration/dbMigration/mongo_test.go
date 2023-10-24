package dbmigration

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

func initMongoTests() *Mongo {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")

	configs := datastore.MongoConfig{
		HostName: c.Get("MONGO_DB_HOST"),
		Port:     c.Get("MONGO_DB_PORT"),
		Username: c.Get("MONGO_DB_USER"),
		Password: c.Get("MONGO_DB_PASS"),
		Database: c.Get("MONGO_DB_NAME"),
	}

	db, _ := datastore.GetNewMongoDB(logger, &configs)
	mongo := NewMongo(db)

	return mongo
}

func insertMongoMigration(ctx context.Context, t *testing.T, md *Mongo, mig *gofrMigration) {
	md.newMigrations = append(md.newMigrations, *mig)

	for i, v := range md.newMigrations {
		_, err := md.coll.InsertOne(ctx, gofrMigration{v.App, v.Version, v.StartTime, v.EndTime, v.Method})
		if err != nil {
			t.Errorf("Failed insertion of %d gofr_migrations table :%v", i, err)
		}
	}

	md.newMigrations = nil
}

func TestMongo_Run(t *testing.T) {
	m := initMongoTests()
	ctx := context.TODO()

	logger := log.NewMockLogger(io.Discard)

	defer func() {
		err := m.coll.Drop(ctx)
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	tests := []struct {
		desc   string
		method string
		mongo  *Mongo
		err    error
	}{
		{"success case", "UP", m, nil},
		{"failure case", "DOWN", m, errors.Error("test error")},
		{"db not initialized", "UP", &Mongo{}, errors.DataStoreNotInitialized{DBName: datastore.MongoStore}},
	}

	for i, tc := range tests {
		err := tc.mongo.Run(K20180324120906{}, "testApp", "20180324120906", tc.method, logger)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestMongo_LastRunVersion(t *testing.T) {
	m := initMongoTests()
	ctx := context.TODO()

	defer func() {
		err := m.coll.Drop(ctx)
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	testcases := []struct {
		desc            string
		mongo           *Mongo
		expectedVersion int
	}{
		{"success case", m, 0},
		{"db not initialized", &Mongo{}, -1},
	}

	for i, tc := range testcases {
		lastVersion := tc.mongo.LastRunVersion("sample-api-v2", "UP")

		assert.Equal(t, tc.expectedVersion, lastVersion, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}

func TestMongo_GetAllMigrations(t *testing.T) {
	m := initMongoTests()
	now := time.Now()
	ctx := context.TODO()

	defer func() {
		err := m.coll.Drop(ctx)
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	insertMongoMigration(ctx, t, m, &gofrMigration{
		App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "UP",
	})
	insertMongoMigration(ctx, t, m, &gofrMigration{
		App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "DOWN",
	})

	expOut := []int{20180324120906}

	up, down := m.GetAllMigrations("gofr-app-v3")

	assert.Equal(t, expOut, up, "TEST failed.\n%s", "get all UP migrations")

	assert.Equal(t, expOut, down, "TEST failed.\n%s", "get all DOWN migrations")
}

func TestMongo_FinishMigration(t *testing.T) {
	m := initMongoTests()
	ctx := context.TODO()

	defer func() {
		err := m.coll.Drop(ctx)
		if err != nil {
			t.Errorf("Got error while dropping the table at last: %v", err)
		}
	}()

	now := time.Now()
	migration := gofrMigration{App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "UP"}
	m.newMigrations = append(m.newMigrations, migration)
	err := m.FinishMigration()

	assert.Nil(t, err)
}

func TestNewMongo_Fail(t *testing.T) {
	mongo := NewMongo(nil)

	assert.Empty(t, mongo)
}

func TestMongo_Run_Fail(t *testing.T) {
	m := &Mongo{}

	expectedError := errors.DataStoreNotInitialized{DBName: datastore.MongoStore}

	err := m.Run(K20180324120906{}, "sample-api", "20180324120906", "UP", log.NewMockLogger(io.Discard))

	assert.EqualError(t, err, expectedError.Error())
}

func TestMongo_LastRunVersion_Fail(t *testing.T) {
	m := &Mongo{}

	expectedLastVersion := -1

	lastVersion := m.LastRunVersion("gofr", "UP")

	assert.Equal(t, expectedLastVersion, lastVersion)
}

func TestMongo_GetAllMigrations_Fail(t *testing.T) {
	m := &Mongo{}

	expectedUP := []int{-1}

	upMigrations, downMigrations := m.GetAllMigrations("gofr")

	assert.Equal(t, expectedUP, upMigrations)
	assert.Nil(t, downMigrations)
}

func TestMongo_FinishMigration_Fail(t *testing.T) {
	m := &Mongo{}

	expectedError := errors.DataStoreNotInitialized{DBName: datastore.MongoStore}

	err := m.FinishMigration()

	assert.EqualError(t, err, expectedError.Error())
}

func TestMongo_IsDirty(t *testing.T) {
	logger := log.NewMockLogger(new(bytes.Buffer))

	mongo, _ := datastore.GetMongoDBFromEnv(logger)
	md := NewMongo(mongo)

	defer func() {
		_ = md.coll.Drop(context.TODO())
	}()

	_, _ = md.coll.InsertOne(context.TODO(), gofrMigration{App: "testing", Version: 20170101100101, Method: UP, StartTime: time.Now()})

	type args struct {
		m      Migrator
		app    string
		name   string
		method string
		logger log.Logger
	}

	tt := struct {
		name    string
		args    args
		wantErr bool
	}{"migration UP", args{nil, "testing", "20200324162754", "UP", logger}, true}

	if err := md.Run(tt.args.m, tt.args.app, tt.args.name, tt.args.method, tt.args.logger); (err != nil) != tt.wantErr {
		t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
	}
}

func TestMongo_DOWN(t *testing.T) {
	logger := log.NewMockLogger(new(bytes.Buffer))

	database, _ := datastore.GetMongoDBFromEnv(logger)

	type args struct {
		app    string
		method string
		ver    int
	}

	tt := struct {
		name    string
		args    args
		wantErr bool
	}{"down error", args{"testing", "DOWN", 20180324120906}, true}

	m := NewMongo(database)
	if err := m.Run(K20180324120906{}, tt.args.app, "20180324120906", tt.args.method, logger); (err != nil) != tt.wantErr {
		t.Errorf("postRun() error = %v, wantErr %v", err, tt.wantErr)
	}
}
