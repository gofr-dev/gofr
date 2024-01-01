package dbmigration

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

type Clickhouse struct {
	datastore.ClickHouseDB
	existingMigration []gofrMigration
}

func NewClickhouse(c datastore.ClickHouseDB) *Clickhouse {
	return &Clickhouse{c, make([]gofrMigration, 0)}
}

// Run executes a migration
func (c *Clickhouse) Run(m Migrator, app, name, methods string, logger log.Logger) error {
	if c.Conn == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.ClickHouse}
	}

	err := c.preRun(app, methods, name)
	if err != nil {
		return err
	}

	ds := &datastore.DataStore{ClickHouse: c.ClickHouseDB}

	if methods == UP {
		err = m.Up(ds, logger)
	} else {
		err = m.Down(ds, logger)
	}

	if err != nil {
		return &errors.Response{Reason: "error encountered in running the migration", Detail: err}
	}

	err = c.postRun(app, methods, name)
	if err != nil {
		return err
	}

	return nil
}

func (c *Clickhouse) preRun(app, method, name string) error {
	const migrationTableSchema = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    app String,
    version Int64,
    start_time String DEFAULT now(),
    end_time Nullable(String),
    method String
) ENGINE = MergeTree()
ORDER BY (app, version, method);
`

	err := c.ClickHouseDB.Exec(context.Background(), migrationTableSchema)
	if err != nil {
		return &errors.Response{Reason: "unable to create table", Detail: err.Error()}
	}

	ver, _ := strconv.Atoi(name)

	c.existingMigration = append(c.existingMigration, gofrMigration{
		App:       app,
		Version:   int64(ver),
		StartTime: time.Now(),
		Method:    method,
	})

	return nil
}

func (c *Clickhouse) postRun(app, method, name string) error {
	ver, _ := strconv.Atoi(name)

	for i, v := range c.existingMigration {
		if v.App == app && v.Method == method && v.Version == int64(ver) {
			c.existingMigration[i].EndTime = time.Now()
		}
	}

	return nil
}

// LastRunVersion retrieves the last run migration version
func (c *Clickhouse) LastRunVersion(app, method string) (lv int) {
	if c.Conn == nil {
		return -1
	}

	query := fmt.Sprintf(`
    SELECT MAX(version) AS version
    FROM gofr_migrations
    WHERE app = '%s' AND method = '%s'
`, app, method)

	row := c.Conn.QueryRow(context.Background(), query)

	var version int64
	_ = row.Scan(&version)

	return int(version)
}

// GetAllMigrations retrieves all migrations
func (c *Clickhouse) GetAllMigrations(app string) (upMigration, downMigration []int) {
	if c.Conn == nil {
		return []int{-1}, nil
	}

	rows, err := c.Conn.Query(context.Background(), fmt.Sprintf(`
    SELECT version, method
    FROM gofr_migrations
    WHERE app = '%s'
`, app))

	if err != nil {
		return nil, nil
	}

	defer rows.Close()

	for rows.Next() {
		var (
			version int64
			method  string
		)

		if err := rows.Scan(&version, &method); err != nil {
			return nil, nil
		}

		if method == UP {
			upMigration = append(upMigration, int(version))
		} else {
			downMigration = append(downMigration, int(version))
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil
	}

	return upMigration, downMigration
}

// FinishMigration completes the migration
func (c *Clickhouse) FinishMigration() error {
	if c.Conn == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.ClickHouse}
	}

	if len(c.existingMigration) != 0 {
		query := "INSERT INTO gofr_migrations (app, version, start_time, end_time, method) VALUES"

		for _, v := range c.existingMigration {
			query += fmt.Sprintf(" ('%s', %d, '%s', '%s', '%s'),", v.App, v.Version, v.StartTime, v.EndTime, v.Method)
		}

		query = strings.TrimSuffix(query, ",")

		err := c.Conn.Exec(context.Background(), query)
		if err != nil {
			return err
		}
	}

	return nil
}
