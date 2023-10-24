package dbmigration

import (
	"strconv"
	"time"

	"github.com/gocql/gocql"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

type Cassandra struct {
	cluster       *gocql.ClusterConfig
	session       *gocql.Session
	newMigrations []gofrMigration // tracks all the migrations run with it's startTime and endTime
}

// NewCassandra returns a new Cassandra instance
func NewCassandra(d *datastore.Cassandra) *Cassandra {
	return &Cassandra{d.Cluster, d.Session, make([]gofrMigration, 0)}
}

// Run executes a migration
//
//nolint:dupl // Cassandra shares the same migration logic with YCQL with slight changes in the database logic
func (c *Cassandra) Run(m Migrator, app, name, method string, logger log.Logger) error {
	if c.session == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.CassandraStore}
	}

	err := c.preRun(app, method, name)
	if err != nil {
		return err
	}

	ds := &datastore.DataStore{Cassandra: datastore.Cassandra{
		Cluster: c.cluster,
		Session: c.session,
	}}

	if method == UP {
		err = m.Up(ds, logger)
	} else {
		err = m.Down(ds, logger)
	}

	if err != nil {
		c.deleteRow(app, name)
		return &errors.Response{Reason: "error encountered in running the migration", Detail: err}
	}

	err = c.postRun(app, method, name)
	if err != nil {
		c.deleteRow(app, name)
		return err
	}

	return nil
}

func (c *Cassandra) preRun(app, method, name string) error {
	migrationTableSchema := "CREATE TABLE IF NOT EXISTS gofr_migrations ( " +
		"app text, version bigint, start_time timestamp, end_time timestamp, method text, PRIMARY KEY (app, version, method) )"

	err := c.session.Query(migrationTableSchema).Exec()
	if err != nil {
		return &errors.Response{Reason: "unable to create table, err: " + err.Error()}
	}

	if c.isDirty(app) {
		return &errors.Response{Reason: "dirty migration check failed"}
	}

	ver, _ := strconv.Atoi(name)

	c.newMigrations = append(c.newMigrations, gofrMigration{
		App:       app,
		Version:   int64(ver),
		StartTime: time.Now(),
		Method:    method,
	})

	return nil
}

func (c *Cassandra) isDirty(app string) bool {
	var dirtyCount int

	err := c.session.Query("SELECT COUNT(*) FROM gofr_migrations WHERE app = ? AND end_time = ? ALLOW FILTERING;",
		app, time.Time{}).Scan(&dirtyCount)
	if err != nil || dirtyCount > 0 {
		return true
	}

	return false
}

func (c *Cassandra) postRun(app, method, name string) error {
	ver, _ := strconv.Atoi(name)

	for i, v := range c.newMigrations {
		if v.App == app && v.Method == method && v.Version == int64(ver) && v.EndTime.IsZero() {
			c.newMigrations[i].EndTime = time.Now()
		}
	}

	return nil
}

func (c *Cassandra) deleteRow(app, name string) {
	n, _ := strconv.Atoi(name)
	_ = c.session.Query("DELETE FROM gofr_migrations WHERE app = ? AND version = ?", app, n).Exec()
}

// LastRunVersion retrieves the last run migration version
func (c *Cassandra) LastRunVersion(app, method string) (lv int) {
	if c.session == nil {
		return -1
	}

	_ = c.session.Query("SELECT MAX(version) FROM gofr_migrations WHERE app = ? and method = ? ALLOW FILTERING;",
		app, method).Scan(&lv)

	return
}

// GetAllMigrations retrieves migration versions
func (c *Cassandra) GetAllMigrations(app string) (upMigrations, downMigrations []int) {
	if c.session == nil {
		return []int{-1}, nil
	}

	iter := c.session.Query("SELECT version, method FROM gofr_migrations WHERE app = ? ALLOW FILTERING",
		app).Iter()

	verSlcMap, _ := iter.SliceMap()
	for _, v := range verSlcMap {
		ver, _ := v["version"].(int64)
		method, _ := v["method"].(string)

		if method == UP {
			upMigrations = append(upMigrations, int(ver))
		} else {
			downMigrations = append(downMigrations, int(ver))
		}
	}

	return
}

// FinishMigration completes the migration
func (c *Cassandra) FinishMigration() error {
	if c.session == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.CassandraStore}
	}

	for _, l := range c.newMigrations {
		err := c.session.Query("INSERT INTO gofr_migrations(app, version, method, start_time, end_time) "+
			"VALUES (?, ?, ?, ?, ?)", l.App, l.Version, l.Method, l.StartTime, l.EndTime).Exec()
		if err != nil {
			return err
		}
	}

	return nil
}
