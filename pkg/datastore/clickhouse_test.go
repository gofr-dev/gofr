package datastore

import (
	"context"
	"io"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

func getClickHouseDB() DataStore {
	c := config.NewGoDotEnvProvider(log.NewMockLogger(io.Discard), "../../configs")

	dc := ClickHouseConfig{
		Host:     c.Get("CLICKHOUSE_HOST"),
		Username: c.Get("CLICKHOUSE_USER"),
		Password: c.Get("CLICKHOUSE_PASSWORD"),
		Database: c.Get("CLICKHOUSE_NAME"),
		Port:     c.Get("CLICKHOUSE_PORT"),
	}

	db, _ := GetNewClickHouseDB(log.NewMockLogger(io.Discard), &dc)

	store := new(DataStore)

	store.ClickHouse.Conn = db.Conn
	store.ClickHouse.config = db.config
	store.ClickHouse.logger = log.NewMockLogger(io.Discard)

	return *store
}

func Test_getClickHouseDBError(t *testing.T) {
	dc := ClickHouseConfig{}

	_, err := GetNewClickHouseDB(log.NewMockLogger(io.Discard), &dc)

	assert.NotNilf(t, err, "Test Failed")
}

func TestCLICKHOUSEClient_Exec(t *testing.T) {
	db := getClickHouseDB()

	err := db.ClickHouse.Exec(context.Background(), "SHOW TABLES")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	err = db.ClickHouse.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS testtable(id int PRIMARY KEY) engine=MergeTree order by id")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	err = db.ClickHouse.Exec(context.Background(), "DROP TABLE IF EXISTS testtable")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	var d DataStore
	err = d.ClickHouse.Exec(context.Background(), "Drop table test")
	expErr := errors.ClickhouseNotInitialized

	assert.Equal(t, expErr, err, "Exec operation failed")
}

func TestCLCIKHOUSE_Query(t *testing.T) {
	db := getClickHouseDB()

	err := db.ClickHouse.Exec(context.Background(), "DROP TABLE IF EXISTS testtable")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	err = db.ClickHouse.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS testtable(id int PRIMARY KEY)engine=MergeTree order by id")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	err = db.ClickHouse.Exec(context.Background(), "INSERT INTO testtable(id) values(32)")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	rows, err := db.ClickHouse.Query(context.Background(), "SELECT * FROM testtable")
	if err != nil {
		t.Errorf("Query operation failed. Got: %s", err)
	}

	if rows.Err() != nil {
		t.Errorf("Encountered error: %s", rows.Err())
	}

	defer rows.Close()

	if rows == nil {
		t.Errorf("Failed. Got empty rows")
	}

	var (
		d       DataStore
		expRows driver.Rows
	)

	rows, err = d.ClickHouse.Query(context.Background(), "Select * from test")
	expErr := errors.ClickhouseNotInitialized

	assert.Equal(t, expErr, err, "Query operation failed")
	assert.Equal(t, expRows, rows, "Query operation failed")
}

func TestClickHouse_QueryRow(t *testing.T) {
	db := getClickHouseDB()

	err := db.ClickHouse.Exec(context.Background(), "DROP TABLE IF EXISTS testtable")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	err = db.ClickHouse.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS testtable(id int PRIMARY KEY) engine=MergeTree order by id")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	err = db.ClickHouse.Exec(context.Background(), "INSERT INTO testtable(id) values(32)")
	if err != nil {
		t.Errorf("Exec operation failed. Got: %s", err)
	}

	row := db.ClickHouse.QueryRow(context.Background(), "SELECT * FROM testtable")
	if row == nil {
		t.Errorf("QueryRow operation failed.")
	}
}
