package dbmigration

import (
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

func initClickHouseTests() *Clickhouse {
	logger := log.NewMockLogger(io.Discard)

	c := config.NewGoDotEnvProvider(log.NewMockLogger(io.Discard), "../../../../configs")

	configs := datastore.ClickHouseConfig{
		Host:     c.Get("CLICKHOUSE_HOST"),
		Username: c.Get("CLICKHOUSE_USER"),
		Password: c.Get("CLICKHOUSE_PASSWORD"),
		Database: c.Get("CLICKHOUSE_NAME"),
		Port:     c.Get("CLICKHOUSE_PORT"),
	}

	db, _ := datastore.GetNewClickHouseDB(logger, &configs)

	clickhouse := NewClickhouse(db)

	return clickhouse
}

func Test_ClickHouseRun(t *testing.T) {
	g := initClickHouseTests()

	defer func() {
		err := g.ClickHouseDB.Exec(context.Background(), "DROP TABLE gofr_migrations")
		if err != nil {
			t.Errorf("Unexpected Error while dropping gofr-migrations. %v", err)
		}
	}()

	tests := []struct {
		desc       string
		method     string
		clickhouse *Clickhouse
		err        error
	}{
		{"success case", "UP", g, nil},
		{"failure case", "DOWN", g, &errors.Response{Reason: "error encountered in running the migration", Detail: errors.Error("test error")}},
		{"db not initialized", "UP", &Clickhouse{}, errors.DataStoreNotInitialized{DBName: datastore.ClickHouse}},
	}

	for i, tc := range tests {
		err := tc.clickhouse.Run(K20230324120906{}, "sample-api", "2023032412090", tc.method, log.NewMockLogger(io.Discard))

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func createClickhouseTable(t *testing.T, conn *Clickhouse) {
	err := conn.ClickHouseDB.Exec(context.Background(), "DROP TABLE IF EXISTS gofr_migrations")
	if err != nil {
		t.Errorf("Unexpected Error while dropping gofr-migrations. %v", err)
	}

	// Define the ClickHouse table creation query
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS gofr_migrations (
    app String,
    version Int64,
    start_time String DEFAULT now(),
    end_time Nullable(String),
    method String
) ENGINE = MergeTree()
ORDER BY (app, version, method);
	`

	// Execute the table creation query
	err = conn.Exec(context.Background(), createTableQuery)
	if err != nil {
		t.Errorf("Unexpected Error while dropping gofr-migrations. %v", err)
	}
}

func insertClickhouseMigration(t *testing.T, conn *Clickhouse, mig *gofrMigration) {
	query := `
		INSERT INTO gofr_migrations (app, version, start_time, end_time, method) 
		VALUES (?, ?, ?, ?, ?)
	`

	// Execute the INSERT query
	err := conn.Exec(context.Background(), query, mig.App, mig.Version, mig.StartTime, mig.EndTime, mig.Method)
	if err != nil {
		t.Errorf("FAILED, error in insertion. err: %v", err)
	}
}

func Test_clickhousePreRun(t *testing.T) {
	g := initClickHouseTests()

	defer func() {
		err := g.ClickHouseDB.Exec(context.Background(), "DROP TABLE gofr_migrations")
		if err != nil {
			t.Errorf("Unexpected Error while dropping gofr-migrations. %v", err)
		}
	}()

	err := g.preRun("gofr-example", "UP", "20230324120906")

	assert.Nil(t, err, "TEST Failed.")
}

func Test_clickhousePostRun(t *testing.T) {
	g := initClickHouseTests()

	err := g.postRun("gofr-example", "UP", "20230324120906")

	assert.Nil(t, err, "TEST Failed.")
}

func Test_clickhouseFinishMigration(t *testing.T) {
	g := initClickHouseTests()

	createClickhouseTable(t, g)

	defer func() {
		err := g.ClickHouseDB.Exec(context.Background(), "DROP TABLE gofr_migrations")
		if err != nil {
			t.Errorf("Unexpected Error while dropping gofr-migrations. %v", err)
		}
	}()

	now := time.Now().UTC()

	tests := []struct {
		desc               string
		conn               *Clickhouse
		existingMigrations []gofrMigration
	}{
		{"without rows", g, nil},
		{"with rows", g, []gofrMigration{{App: "gofr-example", Version: int64(20230324120906),
			StartTime: now, EndTime: now, Method: "UP"}}},
	}

	for i, tc := range tests {
		c := tc.conn
		c.existingMigration = tc.existingMigrations

		err := g.FinishMigration()

		assert.Nil(t, err, "TEST[%d] Failed.", i)
	}
}

func Test_lastRunVersion(t *testing.T) {
	g := initClickHouseTests()

	createClickhouseTable(t, g)

	defer func() {
		err := g.ClickHouseDB.Exec(context.Background(), "DROP TABLE gofr_migrations")
		if err != nil {
			t.Errorf("Unexpected Error while dropping gofr-migrations. %v", err)
		}
	}()

	now := time.Now().UTC()

	insertClickhouseMigration(t, g, &gofrMigration{App: "gofr-example", Version: int64(20230324120906),
		StartTime: now, EndTime: now, Method: "UP"})

	res := g.LastRunVersion("gofr-example", "UP")

	assert.Equal(t, 20230324120906, res, "TEST Failed.")
}

func TestClickhouse_GetAllMigrations(t *testing.T) {
	g := initClickHouseTests()

	createClickhouseTable(t, g)

	defer func() {
		err := g.ClickHouseDB.Exec(context.Background(), "DROP TABLE gofr_migrations")
		if err != nil {
			t.Errorf("Unexpected Error while dropping gofr-migrations. %v", err)
		}
	}()

	now := time.Now().UTC()

	insertClickhouseMigration(t, g, &gofrMigration{App: "gofr-example", Version: int64(20230324120906), StartTime: now,
		EndTime: now, Method: "UP"})
	insertClickhouseMigration(t, g, &gofrMigration{App: "gofr-example", Version: int64(20230324120906), StartTime: now,
		EndTime: now, Method: "DOWN"})

	up, down := g.GetAllMigrations("gofr-example")

	assert.Equal(t, []int{20230324120906}, up, "TEST Failed.")
	assert.Equal(t, []int{20230324120906}, down, "TEST Failed.")
}

type K20230324120906 struct{}

func (k K20230324120906) Up(_ *datastore.DataStore, _ log.Logger) error {
	return nil
}

func (k K20230324120906) Down(_ *datastore.DataStore, _ log.Logger) error {
	return errors.Error("test error")
}
