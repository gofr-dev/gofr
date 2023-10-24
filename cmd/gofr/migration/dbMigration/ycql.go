package dbmigration

import (
	"strconv"
	"time"

	"github.com/yugabyte/gocql"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

type YCQL struct {
	cluster       *gocql.ClusterConfig
	session       *gocql.Session
	newMigrations []gofrMigration
}

// NewYCQL return a new YCQL instance
func NewYCQL(d *datastore.YCQL) *YCQL {
	return &YCQL{
		cluster:       d.Cluster,
		session:       d.Session,
		newMigrations: make([]gofrMigration, 0),
	}
}

// Run executes a migration
//
//nolint:dupl // YCQL shares the same migration logic with Cassandra with slight changes in the database queries
func (y *YCQL) Run(m Migrator, app, name, method string, logger log.Logger) error {
	if y.session == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.Ycql}
	}

	err := y.preRun(app, method, name)
	if err != nil {
		return err
	}

	ds := &datastore.DataStore{YCQL: datastore.YCQL{
		Cluster: y.cluster,
		Session: y.session,
	}}

	if method == UP {
		err = m.Up(ds, logger)
	} else {
		err = m.Down(ds, logger)
	}

	if err != nil {
		y.deleteRow(app, name)
		return &errors.Response{Reason: "error encountered in running the migration", Detail: err}
	}

	err = y.postRun(app, method, name)
	if err != nil {
		y.deleteRow(app, name)
		return err
	}

	return nil
}

func (y *YCQL) preRun(app, method, name string) error {
	migrationTableSchema := "CREATE TABLE IF NOT EXISTS gofr_migrations ( " +
		"app text, version bigint, start_time timestamp, end_time timestamp, method text, PRIMARY KEY (app, version, method) )"

	err := y.session.Query(migrationTableSchema).Exec()
	if err != nil {
		return &errors.Response{Reason: "unable to create table, err: " + err.Error()}
	}

	if y.isDirty(app) {
		return &errors.Response{Reason: "dirty migration check failed"}
	}

	ver, _ := strconv.Atoi(name)

	y.newMigrations = append(y.newMigrations, gofrMigration{
		App:       app,
		Version:   int64(ver),
		StartTime: time.Now(),
		Method:    method,
	})

	return nil
}

func (y *YCQL) isDirty(app string) bool {
	var dirtyCount int

	err := y.session.Query("SELECT COUNT(*) FROM gofr_migrations WHERE app = ? AND end_time=? ALLOW FILTERING;",
		app, time.Time{}).Scan(&dirtyCount)
	if err != nil || dirtyCount > 0 {
		return true
	}

	return false
}

func (y *YCQL) postRun(app, method, name string) error {
	ver, _ := strconv.Atoi(name)

	for i, v := range y.newMigrations {
		if v.App == app && v.Method == method && v.Version == int64(ver) && v.EndTime.IsZero() {
			y.newMigrations[i].EndTime = time.Now()
		}
	}

	return nil
}

func (y *YCQL) deleteRow(app, name string) {
	n, _ := strconv.Atoi(name)
	_ = y.session.Query("DELETE FROM gofr_migrations WHERE app = ? AND version = ?", app, n).Exec()
}

// LastRunVersion retrieves the last run migration version
func (y *YCQL) LastRunVersion(app, method string) int {
	if y.session == nil {
		return -1
	}

	var lastVersion int
	_ = y.session.Query("SELECT MAX(version) FROM gofr_migrations WHERE app = ? and method = ? ALLOW FILTERING;",
		app, method).Scan(&lastVersion)

	return lastVersion
}

// GetAllMigrations retrieves all migrations
func (y *YCQL) GetAllMigrations(app string) (upMigration, downMigration []int) {
	if y.session == nil {
		return []int{-1}, nil
	}

	iter := y.session.Query("SELECT version, method FROM gofr_migrations WHERE app = ? ALLOW FILTERING",
		app).Iter()

	verSlcMap, _ := iter.SliceMap()
	for _, v := range verSlcMap {
		ver, _ := v["version"].(int64)
		method, _ := v["method"].(string)

		if method == UP {
			upMigration = append(upMigration, int(ver))
		} else {
			downMigration = append(downMigration, int(ver))
		}
	}

	return
}

// FinishMigration completes the migration
func (y *YCQL) FinishMigration() error {
	if y.session == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.Ycql}
	}

	for _, l := range y.newMigrations {
		err := y.session.Query("INSERT INTO gofr_migrations(app, version, method, start_time, end_time) "+
			"VALUES (?, ?, ?, ?, ?)", l.App, l.Version, l.Method, l.StartTime, l.EndTime).Exec()
		if err != nil {
			return err
		}
	}

	return nil
}
