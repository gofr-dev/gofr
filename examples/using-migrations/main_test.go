package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

const (
	configFolder = "./configs"

	// testDatabase is the MySQL database this test owns. The example's own configs point at the
	// shared `test` database, which every other example migrates into as well - migration
	// bookkeeping there is keyed by version only, so the highest version any example has run wins
	// and the others get skipped. Migrating into a database this test owns keeps the run
	// independent of the other examples and of what previous runs left behind.
	//
	// It is dropped and re-created at the start of every run rather than dropped at the end, so a
	// failing run leaves its data behind to be inspected.
	testDatabase = "test_using_migrations"

	// testRedisDB is the logical Redis database this test owns, and is the same idea as
	// testDatabase: the migrator uses the container's client, so pointing the example at a
	// database the test flushes first means no previous run's bookkeeping can make this one skip
	// its migrations. Namespacing rather than deleting a known list of keys - the list would go
	// stale, silently, the first time a Redis migration writes a key nobody thought to add to it.
	testRedisDB = 1
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")

	c := config.NewEnvFile(configFolder, logging.NewLogger(logging.ERROR))

	if err := setupSQL(c); err != nil {
		fmt.Fprintf(os.Stderr, "could not set up the test database: %v\n", err)
		os.Exit(1)
	}

	if err := setupRedis(c); err != nil {
		fmt.Fprintf(os.Stderr, "could not set up the test redis database: %v\n", err)
		os.Exit(1)
	}

	m.Run()
}

// setupSQL recreates the database this test migrates into and points the example at it.
func setupSQL(c config.Config) error {
	// Connecting without a database, as the one this test uses is about to be created.
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", c.Get("DB_USER"), c.Get("DB_PASSWORD"),
		net.JoinHostPort(c.Get("DB_HOST"), c.Get("DB_PORT")))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	defer db.Close()

	for _, query := range []string{"DROP DATABASE IF EXISTS " + testDatabase, "CREATE DATABASE " + testDatabase} {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	// The config files are read again by gofr.New(), which does not override what is already set.
	return os.Setenv("DB_NAME", testDatabase)
}

// setupRedis empties the logical Redis database this test migrates into and points the example at
// it. GoFr reads REDIS_DB (pkg/gofr/datasource/redis/config.go), and the Redis migrator runs on
// that same client, so the selection covers the migration bookkeeping as well as the data.
func setupRedis(c config.Config) error {
	client := redis.NewClient(&redis.Options{
		Addr: net.JoinHostPort(c.Get("REDIS_HOST"), c.Get("REDIS_PORT")),
		DB:   testRedisDB,
	})

	defer client.Close()

	if err := client.FlushDB(context.Background()).Err(); err != nil {
		return err
	}

	// The config files are read again by gofr.New(), which does not override what is already set.
	return os.Setenv("REDIS_DB", strconv.Itoa(testRedisDB))
}

func TestExampleMigration(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	// The migrations run at startup, which a fixed sleep would not reliably cover.
	testutil.WaitForHTTPServer(t, configs.HTTPHost)

	tests := []struct {
		desc       string
		method     string
		path       string
		body       []byte
		statusCode int
	}{
		{"post new employee with valid data", http.MethodPost, "/employee",
			[]byte(`{"id":2,"name":"John","gender":"Male","contact_number":1234567890,"dob":"2000-01-01"}`), 201},
		{"get employee with valid name", http.MethodGet, "/employee?name=John", nil, 200},
		{"get employee does not exist", http.MethodGet, "/employee?name=Invalid", nil, 500},
		{"get employee with empty name", http.MethodGet, "/employee", nil, http.StatusInternalServerError},
		{"post new employee with invalid data", http.MethodPost, "/employee", []byte(`{"id":2"}`),
			http.StatusInternalServerError},
		{"post new employee with invalid gender", http.MethodPost, "/employee",
			[]byte(`{"id":2,"name":"John","gender":"Male123","contact_number":1234567890,"dob":"2000-01-01"}`), 500},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(tc.method, configs.HTTPHost+tc.path, bytes.NewBuffer(tc.body))
		req.Header.Set("content-type", "application/json")
		c := http.Client{}
		resp, err := c.Do(req)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
