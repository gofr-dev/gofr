package dbmigration

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

const migrationLock = ":migration:lock"

type Redis struct {
	datastore.Redis
	existingMigration []gofrMigration // tracks all the migrations already run and also the new migrations run with it's startTime and endTime
}

// NewRedis returns a new Redis instance
func NewRedis(r datastore.Redis) *Redis {
	return &Redis{r, make([]gofrMigration, 0)}
}

// acquireLock checks whether no other process is running the same migration
func (r *Redis) acquireLock(app string) bool {
	ctx := context.Background()
	l, _ := r.Incr(ctx, app+migrationLock).Result()

	const expDuration = 5

	if l == 1 {
		// in case of unexpected termination of a server, the lock will be present for a maximum of 5 minutes.
		r.Expire(ctx, app+migrationLock, expDuration*time.Minute) // key will expire after 5 minutes
	}

	return l == 1
}

// Run executes a migration
func (r *Redis) Run(mig Migrator, app, name, method string, logger log.Logger) error {
	if r.Redis == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.RedisStore}
	}

	if !r.acquireLock(app) {
		return nil
	}

	defer func() {
		r.Del(context.Background(), app+migrationLock)
	}()

	err := r.preRun(app, method, name)
	if err != nil {
		return err
	}

	ds := datastore.DataStore{Redis: r}

	if method == UP {
		err = mig.Up(&ds, logger)
	} else {
		err = mig.Down(&ds, logger)
	}

	if err != nil {
		return err
	}

	return r.postRun(app, method, name)
}

// LastRunVersion retrieves the last run migration version
func (r *Redis) LastRunVersion(app, method string) int {
	if r.Redis == nil {
		return -1
	}

	res, _ := r.HGet(context.Background(), "gofr_migrations", app).Bytes()

	_ = json.Unmarshal(res, &r.existingMigration)

	filterMig := make([]gofrMigration, 0)

	for _, v := range r.existingMigration {
		if v.Method == method {
			filterMig = append(filterMig, v)
		}
	}

	if len(filterMig) == 0 {
		return 0
	}

	sort.Slice(filterMig, func(i, j int) bool {
		return filterMig[i].Version > filterMig[j].Version
	})

	return int(filterMig[0].Version)
}

func (r *Redis) preRun(app, method, name string) error {
	// dirty migration check
	if r.isDirty(app) {
		return &errors.Response{Reason: "dirty migration check failed"}
	}

	ver, _ := strconv.Atoi(name)

	r.existingMigration = append(r.existingMigration, gofrMigration{
		App:       app,
		Version:   int64(ver),
		StartTime: time.Now(),
		Method:    method,
	})

	return nil
}

func (r *Redis) postRun(app, method, name string) error {
	ver, _ := strconv.Atoi(name)
	length := len(r.existingMigration)

	lastElem := r.existingMigration[length-1]
	if lastElem.EndTime.IsZero() && lastElem.Version == int64(ver) && lastElem.Method == method {
		r.existingMigration[length-1].EndTime = time.Now()
	}

	resBytes, _ := json.Marshal(r.existingMigration)

	err := r.HSet(context.Background(), "gofr_migrations", app, string(resBytes)).Err()

	return err
}

func (r *Redis) isDirty(app string) bool {
	for _, v := range r.existingMigration {
		if v.EndTime.IsZero() && v.App == app {
			return true
		}
	}

	return false
}

// GetAllMigrations retrieves all migrations
func (r *Redis) GetAllMigrations(app string) (upMigrations, downMigrations []int) {
	if r.Redis == nil {
		return []int{-1}, nil
	}

	if len(r.existingMigration) == 0 {
		res, _ := r.HGet(context.Background(), "gofr_migrations", app).Bytes()

		_ = json.Unmarshal(res, &r.existingMigration)
	}

	for _, v := range r.existingMigration {
		if v.Method == UP {
			upMigrations = append(upMigrations, int(v.Version))
		} else {
			downMigrations = append(downMigrations, int(v.Version))
		}
	}

	return
}

// FinishMigration completes the migration
func (r *Redis) FinishMigration() error {
	if r.Redis == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.RedisStore}
	}

	app := r.existingMigration[0].App
	resBytes, _ := json.Marshal(r.existingMigration)
	err := r.HSet(context.Background(), "gofr_migrations", app, string(resBytes)).Err()

	return err
}
