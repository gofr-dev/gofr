package dbmigration

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

type K20200324120906 struct {
}

func (k K20200324120906) Up(_ *datastore.DataStore, l log.Logger) error {
	l.Info("Running test migration: UP")
	return nil
}

func (k K20200324120906) Down(_ *datastore.DataStore, _ log.Logger) error {
	return &errors.Response{Reason: "test error"}
}

func initRedisTests() *Redis {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")

	configs := datastore.RedisConfig{
		HostName: c.Get("REDIS_HOST"),
		Port:     c.Get("REDIS_PORT"),
		Password: c.Get("REDIS_PASS"),
	}

	db, _ := datastore.NewRedis(logger, configs)
	redis := NewRedis(db)

	return redis
}

func insertRedisMigration(t *testing.T, r *Redis, mig *gofrMigration) {
	r.existingMigration = append(r.existingMigration, *mig)

	resBytes, _ := json.Marshal(r.existingMigration)

	err := r.HSet(context.Background(), "gofr_migrations", mig.App, string(resBytes)).Err()
	if err != nil {
		t.Errorf("Failed insertion in gofr_migrations table :%v", err)
	}
}

func TestRedis_Run(t *testing.T) {
	logger := log.NewLogger()
	redis, _ := datastore.NewRedisFromEnv(nil)

	type args struct {
		mig    Migrator
		app    string
		name   string
		method string
		logger log.Logger
	}

	r := NewRedis(redis)

	testcases := []struct {
		desc          string
		args          args
		redis         *Redis
		expectedError error
	}{
		{"lock acquired", args{K20200324120906{}, "redisTest", "20200324120906", "UP", logger}, r, nil},
		{"db not initialized", args{K20200324120906{}, "redisTest", "20200324120906", "UP", logger},
			&Redis{}, errors.DataStoreNotInitialized{DBName: datastore.RedisStore}},
	}

	for i, tt := range testcases {
		err := tt.redis.Run(tt.args.mig, tt.args.app, tt.args.name, tt.args.method, tt.args.logger)

		assert.Equal(t, tt.expectedError, err, "Test[%d] Failed. error = %v, wantErr %v", i, err, tt.expectedError)
	}
}

func TestRedis_LastRunVersion(t *testing.T) {
	redis := initRedisTests()
	ctx := context.Background()

	redis.Incr(ctx, "redisTest"+migrationLock)

	defer func() {
		redis.Del(ctx, "redisTest"+migrationLock)
	}()

	testcases := []struct {
		desc            string
		redis           *Redis
		expectedVersion int
	}{
		{"success case", redis, 0},
		{"db not initialized", &Redis{}, -1},
	}

	for i, tc := range testcases {
		lastVersion := tc.redis.LastRunVersion("sample-api-v2", "UP")

		assert.Equal(t, tc.expectedVersion, lastVersion, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}

func TestRedis_GetAllMigrations(t *testing.T) {
	r := initRedisTests()
	now := time.Now()

	insertRedisMigration(t, r, &gofrMigration{
		App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "UP",
	})
	insertRedisMigration(t, r, &gofrMigration{
		App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "DOWN",
	})

	expOut := []int{20180324120906}

	up, down := r.GetAllMigrations("gofr-app-v3")

	assert.Equal(t, expOut, up, "TEST failed.\n%s", "get all UP migrations")

	assert.Equal(t, expOut, down, "TEST failed.\n%s", "get all DOWN migrations")
}

func TestRedis_FinishMigration(t *testing.T) {
	r := initRedisTests()

	now := time.Now()
	migration := gofrMigration{App: "gofr-app-v3", Version: int64(20180324120906), StartTime: now, EndTime: now, Method: "UP"}
	r.existingMigration = append(r.existingMigration, migration)
	err := r.FinishMigration()

	assert.Nil(t, err)
}

func TestRedis_LastRunVersion_Fail(t *testing.T) {
	m := &Redis{}

	expectedLastVersion := -1

	lastVersion := m.LastRunVersion("gofr", "UP")

	assert.Equal(t, expectedLastVersion, lastVersion)
}

func TestRedis_GetAllMigrations_Fail(t *testing.T) {
	m := &Redis{}

	expectedUP := []int{-1}

	upMigrations, downMigrations := m.GetAllMigrations("gofr")

	assert.Equal(t, expectedUP, upMigrations)
	assert.Nil(t, downMigrations)
}

func TestRedis_FinishMigration_Fail(t *testing.T) {
	m := &Redis{}

	expectedError := errors.DataStoreNotInitialized{DBName: datastore.RedisStore}

	err := m.FinishMigration()

	assert.EqualError(t, err, expectedError.Error())
}

func TestRedis_DOWN(t *testing.T) {
	database, _ := datastore.NewRedisFromEnv(nil)

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

	r := NewRedis(database)
	if err := r.Run(K20180324120906{}, tt.args.app, "20180324120906", tt.args.method, log.NewLogger()); (err != nil) != tt.wantErr {
		t.Errorf("postRun() error = %v, wantErr %v", err, tt.wantErr)
	}
}
